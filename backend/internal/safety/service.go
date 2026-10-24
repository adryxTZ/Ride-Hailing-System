package safety

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/phone"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/scope"
)

// Service is the safety use cases for one brand.
type Service struct {
	store     store
	trips     Trips
	locate    DriverLocator
	sms       Texter
	country   string
	brandName string
	now       func() time.Time
}

// shareTokenBytes makes share links unguessable.
const shareTokenBytes = 18

// alertsPage bounds the staff alert list.
const alertsPage = 100

// New wires the service.
func New(pool *pgxpool.Pool, brandID, country, brandName string, t Trips, loc DriverLocator, sms Texter) *Service {
	return &Service{store: store{pool: pool, brandID: brandID}, trips: t, locate: loc, sms: sms, country: country, brandName: brandName, now: time.Now}
}

// Contacts returns the rider's trusted contacts.
func (s *Service) Contacts(ctx context.Context, userID string) ([]TrustedContact, error) {
	return s.store.contacts(ctx, userID)
}

// SaveContacts replaces the rider's trusted contacts (up to MaxTrustedContacts).
func (s *Service) SaveContacts(ctx context.Context, userID string, list []TrustedContact) ([]TrustedContact, error) {
	if len(list) > MaxTrustedContacts {
		return nil, ErrTooManyContacts
	}
	clean := make([]TrustedContact, 0, len(list))
	for i, c := range list {
		p, err := phone.Normalize(c.Phone, s.country)
		if err != nil || strings.TrimSpace(c.Name) == "" {
			return nil, apperr.Validation.WithDetails(map[string]any{"fields": []map[string]string{{"path": fmt.Sprintf("contacts[%d]", i), "code": "INVALID"}}})
		}
		clean = append(clean, TrustedContact{Name: strings.TrimSpace(c.Name), Phone: p})
	}
	if err := s.store.replaceContacts(ctx, userID, clean); err != nil {
		return nil, err
	}
	return s.store.contacts(ctx, userID)
}

// ShareLink returns (creating once) the public link of a trip the rider is on.
func (s *Service) ShareLink(ctx context.Context, tripID, userID, baseURL string) (*ShareLink, error) {
	f, err := s.participantFacts(ctx, tripID, userID)
	if err != nil {
		return nil, err
	}
	token, err := s.store.shareToken(ctx, f.ID, newToken())
	if err != nil {
		return nil, err
	}
	return &ShareLink{URL: strings.TrimRight(baseURL, "/") + "/share/" + token, Token: token}, nil
}

// PublicView is the live trip behind a share link, until ShareGrace after it ends.
func (s *Service) PublicView(ctx context.Context, token string) (*PublicTrip, error) {
	id, err := s.store.tripByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	if id == "" {
		return nil, apperr.NotFound
	}
	f, err := s.trips.Facts(ctx, id)
	if err != nil || f == nil {
		return nil, err
	}
	if f.Terminal && f.EndedAt != nil && s.now().Sub(*f.EndedAt) > ShareGrace {
		return nil, ErrShareExpired
	}
	v := &PublicTrip{Code: f.Code, Status: f.Status, RiderName: firstName(f.RiderName), DriverName: firstName(f.DriverName),
		Vehicle: f.Vehicle, Pickup: f.Pickup, Dropoff: f.Dropoff, UpdatedAt: s.now(), Ended: f.Terminal}
	if f.DriverID != "" && !f.Terminal && s.locate != nil {
		v.Driver = s.locate(ctx, f.DriverID)
	}
	return v, nil
}

// SOS records an emergency on the trip, texts the rider's trusted contacts the
// live link, and queues it for staff. Texting failures never block the alert.
func (s *Service) SOS(ctx context.Context, tripID, userID, baseURL string, in SOSInput) (*Alert, error) {
	f, err := s.participantFacts(ctx, tripID, userID)
	if err != nil {
		return nil, err
	}
	link, err := s.ShareLink(ctx, tripID, userID, baseURL)
	if err != nil {
		return nil, err
	}
	contacts, err := s.store.contacts(ctx, userID)
	if err != nil {
		return nil, err
	}
	alerted := s.textContacts(ctx, contacts, firstName(f.RiderName), link.URL)
	id, err := s.store.insertAlert(ctx, f.ID, userID, in, alerted)
	if err != nil {
		return nil, err
	}
	actor := userID
	_ = s.trips.RecordEvent(ctx, f.ID, "trip.sos", "RIDER", &actor, map[string]any{"alert_id": id, "location": in.Location, "contacts_alerted": alerted})
	slog.Warn("SOS raised", "trip", f.Code, "alert", id, "contacts_alerted", alerted)
	return &Alert{ID: id, TripID: f.ID, TripCode: f.Code, UserID: userID, ContactsAlerted: alerted, CreatedAt: s.now()}, nil
}

// Alerts lists SOS alerts for staff (open ones only by default).
// The scope limits them to a country or region ("" = everywhere).
func (s *Service) Alerts(ctx context.Context, openOnly bool, sc scope.Scope) ([]Alert, error) {
	return s.store.alerts(ctx, openOnly, sc, alertsPage)
}

// AlertScope is the country and region of an alert's trip.
func (s *Service) AlertScope(ctx context.Context, alertID string) (country, areaID string, err error) {
	return s.store.alertScope(ctx, alertID)
}

// Resolve closes an SOS alert with what staff did.
func (s *Service) Resolve(ctx context.Context, alertID, staffID, resolution string) error {
	if strings.TrimSpace(resolution) == "" {
		return apperr.Validation.WithDetails(map[string]any{"fields": []map[string]string{{"path": "resolution", "code": "REQUIRED"}}})
	}
	ok, err := s.store.resolve(ctx, alertID, staffID, resolution)
	if err == nil && !ok {
		return apperr.NotFound
	}
	return err
}

func (s *Service) textContacts(ctx context.Context, contacts []TrustedContact, rider, link string) int {
	if s.sms == nil {
		return 0
	}
	sent := 0
	for _, c := range contacts {
		body := fmt.Sprintf("%s: %s pressed SOS during a ride. Follow the trip live: %s", s.brandName, rider, link)
		if err := s.sms.Send(ctx, c.Phone, body); err != nil {
			slog.Warn("SOS text failed", "to", c.Phone, "err", err)
			continue
		}
		sent++
	}
	return sent
}

func (s *Service) participantFacts(ctx context.Context, tripID, userID string) (*TripFacts, error) {
	f, err := s.trips.Facts(ctx, tripID)
	if err != nil {
		return nil, err
	}
	if f == nil {
		return nil, apperr.NotFound
	}
	if f.RiderID != userID && f.DriverID != userID {
		return nil, ErrNotParticipant
	}
	return f, nil
}

func newToken() string {
	b := make([]byte, shareTokenBytes)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func firstName(full string) string {
	if f := strings.Fields(full); len(f) > 0 {
		return f[0]
	}
	return ""
}
