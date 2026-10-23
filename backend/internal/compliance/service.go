package compliance

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kalen-limited/hauler-rider/backend/internal/audit"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/brand"
	"github.com/kalen-limited/hauler-rider/backend/internal/providers/regulator"
)

// Audit actions.
const (
	actionSettings = "regulator.settings"
	actionRetry    = "regulator.retry"
)

// ServiceRules returns a service's compliance settings (catalogue data).
type ServiceRules func(ctx context.Context, serviceKey string) (json.RawMessage, error)

// Service queues and sends regulator submissions.
type Service struct {
	store      store
	countries  []brand.Country
	regulators map[string]regulator.Regulator // by regulator name; a missing entry means "pending" (queue only)
	adapters   map[string]string              // adapter name per regulator, for the admin page
	rules      ServiceRules
	now        func() time.Time
	// documentHolds and reminder serve the document expiry rules (nil = off).
	documentHolds DocumentHolds
	reminder      Reminder
	gate          sync.Map // driverID → gateEntry
}

// Deps is everything the service needs.
type Deps struct {
	Pool       *pgxpool.Pool
	BrandID    string
	Countries  []brand.Country
	Regulators map[string]regulator.Regulator
	Adapters   map[string]string
	Rules      ServiceRules
}

// SetDocumentRules plugs in the hold and reminder sides of document expiry.
func (s *Service) SetDocumentRules(h DocumentHolds, r Reminder) { s.documentHolds, s.reminder = h, r }

// New builds the service.
func New(d Deps) *Service {
	return &Service{store: store{pool: d.Pool, brandID: d.BrandID}, countries: d.Countries, regulators: d.Regulators,
		adapters: d.Adapters, rules: d.Rules, now: time.Now}
}

// regulated returns the country's regulator name, or "" when it has none.
func (s *Service) regulated(code string) (*brand.Country, string) {
	for i := range s.countries {
		c := &s.countries[i]
		if c.Code == code && c.Regulator != "" && c.Regulator != "none" {
			return c, c.Regulator
		}
	}
	return nil, ""
}

// EnqueueApprovalTx registers a driver and their vehicle with the country's
// regulator, in the transaction that approves them (re-approval sends again).
func (s *Service) EnqueueApprovalTx(ctx context.Context, tx pgx.Tx, country, driverID, vehicleID string) error {
	_, reg := s.regulated(country)
	if reg == "" {
		return nil
	}
	if err := s.store.enqueueTx(ctx, tx, subject{country: country, regulator: reg, kind: KindDriver, key: driverID, driverID: &driverID}); err != nil {
		return err
	}
	if vehicleID == "" {
		return nil
	}
	return s.store.enqueueTx(ctx, tx, subject{country: country, regulator: reg, kind: KindVehicle, key: vehicleID, driverID: &driverID, vehicleID: &vehicleID})
}

// EnqueueTripTx queues a completed trip once, in its completing transaction,
// when its country has a regulator and its service submits trips.
func (s *Service) EnqueueTripTx(ctx context.Context, tx pgx.Tx, country, serviceKey, tripID, driverID string) error {
	_, reg := s.regulated(country)
	if reg == "" || s.rules == nil {
		return nil
	}
	rules, err := s.rules(ctx, serviceKey)
	if err != nil || !ShouldSubmit(reg, rules) {
		return err
	}
	return s.store.enqueueOnceTx(ctx, tx, subject{country: country, regulator: reg, kind: KindTrip, key: tripID, tripID: &tripID, driverID: &driverID})
}

// Statuses lists every regulated country (or one) with its switches and figures.
func (s *Service) Statuses(ctx context.Context, country string) ([]Status, error) {
	out := []Status{}
	for _, c := range s.countries {
		if _, reg := s.regulated(c.Code); reg == "" || (country != "" && c.Code != country) {
			continue
		}
		st := Status{Country: c.Code, Regulator: c.Regulator, Adapter: s.adapterName(c.Regulator), Configured: s.regulators[c.Regulator] != nil}
		set, err := s.store.settings(ctx, c.Code)
		if err != nil {
			return nil, err
		}
		st.Enabled, st.FailClosed = set.Enabled, set.FailClosed
		if err := s.store.counts(ctx, &st, midnight(s.now(), c.Timezone)); err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, nil
}

func (s *Service) adapterName(reg string) string {
	if a := s.adapters[reg]; a != "" {
		return a
	}
	return "pending"
}

// SetSettings changes a country's switches, audited. Fail-closed needs a real
// adapter: with nothing sending, no driver could ever become registered.
func (s *Service) SetSettings(ctx context.Context, staffID, country string, enabled, failClosed *bool, reason string) (Settings, error) {
	if strings.TrimSpace(reason) == "" {
		return Settings{}, ErrReasonRequired
	}
	_, reg := s.regulated(country)
	if reg == "" {
		return Settings{}, ErrNoRegulator
	}
	st, err := s.store.settings(ctx, country)
	if err != nil {
		return st, err
	}
	if enabled != nil {
		st.Enabled = *enabled
	}
	if failClosed != nil {
		st.FailClosed = *failClosed
	}
	if st.FailClosed && s.regulators[reg] == nil {
		return st, ErrNotConfigured
	}
	err = pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		if err := s.store.saveSettings(ctx, tx, country, st, staffID); err != nil {
			return err
		}
		return audit.Record(ctx, tx, s.store.brandID, staffID, actionSettings, "regulator:"+country, reason, st)
	})
	s.gate.Clear() // the gate must see the new switch at once
	return st, err
}

// Retry makes a pending submission due now, audited.
func (s *Service) Retry(ctx context.Context, staffID, id, reason string) (*Submission, error) {
	if strings.TrimSpace(reason) == "" {
		return nil, ErrReasonRequired
	}
	err := pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		ok, err := s.store.retryNow(ctx, tx, id)
		if err != nil {
			return err
		}
		if !ok {
			return ErrNotPending
		}
		return audit.Record(ctx, tx, s.store.brandID, staffID, actionRetry, "submission:"+id, reason, nil)
	})
	if err != nil {
		return nil, err
	}
	return s.store.get(ctx, id)
}

// List returns submissions for the admin table.
func (s *Service) List(ctx context.Context, f Filter) ([]Submission, error) {
	if f.Limit <= 0 || f.Limit > listLimit {
		f.Limit = listLimit
	}
	f.Kind, f.Status = strings.ToUpper(f.Kind), strings.ToUpper(f.Status)
	return s.store.list(ctx, f)
}

// Badges are the standing of subjects of one kind (e.g. trips on a list).
func (s *Service) Badges(ctx context.Context, kind string, keys []string) (map[string]Badge, error) {
	return s.store.badges(ctx, kind, keys)
}

// midnight is the start of today in a timezone.
func midnight(now time.Time, tz string) time.Time {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}
	n := now.In(loc)
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, loc)
}
