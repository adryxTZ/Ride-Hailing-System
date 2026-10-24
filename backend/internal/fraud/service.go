package fraud

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kalen-limited/hauler-rider/backend/internal/audit"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/phone"
)

// Service is the fraud use cases for one brand.
type Service struct {
	store     store
	countries []string
	tz        func(country string) *time.Location
	now       func() time.Time
}

// New wires the service; countries are the brand's, tz gives each one's timezone.
func New(pool *pgxpool.Pool, brandID string, countries []string, tz func(country string) *time.Location) *Service {
	return &Service{store: store{pool: pool, brandID: brandID}, countries: countries, tz: tz, now: time.Now}
}

// Raise records a finding (or adds a hit to its open flag); returns the flag id ("" when already reviewed this period).
func (s *Service) Raise(ctx context.Context, f Finding) (string, error) {
	id, err := s.store.raise(ctx, f)
	if err == nil && id != "" {
		slog.Info("fraud flag", "rule", f.Rule, "subject", f.SubjectID, "flag", id)
	}
	return id, err
}

// List returns a page of the review queue and the cursor of the next page (nil at the end).
func (s *Service) List(ctx context.Context, f Filter) ([]Flag, *time.Time, error) {
	if f.Limit <= 0 || f.Limit > listLimitMax {
		f.Limit = listLimitDefault
	}
	list, err := s.store.list(ctx, f)
	if err != nil || len(list) < f.Limit {
		return list, nil, err
	}
	next := list[len(list)-1].OpenedAt
	return list, &next, nil
}

// Get returns one flag.
func (s *Service) Get(ctx context.Context, id string) (*Flag, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, ErrFlagNotFound
	}
	f, err := s.store.get(ctx, nil, id, false)
	return &f, err
}

// Review closes a flag as dismissed or confirmed, with a note (audited). A
// reviewed flag never changes again; staff act on a confirmed one through
// suspend, hold or block.
func (s *Service) Review(ctx context.Context, staffID, id, decision, note string) (*Flag, error) {
	note = strings.TrimSpace(note)
	if note == "" {
		return nil, ErrReasonRequired
	}
	status := map[string]string{DecisionDismiss: StatusDismissed, DecisionConfirm: StatusConfirmed}[decision]
	if status == "" || len(note) > reviewNoteMaxLen {
		return nil, ErrDecision
	}
	if _, err := uuid.Parse(id); err != nil {
		return nil, ErrFlagNotFound
	}
	err := pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		cur, err := s.store.get(ctx, tx, id, true)
		if err != nil {
			return err
		}
		if cur.Status != StatusOpen {
			return ErrFlagReviewed
		}
		if _, err := tx.Exec(ctx, `UPDATE fraud_flags SET status=$2, reviewed_by=$3, reviewed_at=now(), review_note=$4 WHERE id=$1`, id, status, staffID, note); err != nil {
			return err
		}
		return audit.Record(ctx, tx, s.store.brandID, staffID, "fraud.review", "fraud_flag:"+id, note,
			map[string]any{"decision": decision, "rule": cur.Rule, "subject": cur.Subject.ID})
	})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, id)
}

// OpenCount is how many open flags a person has (driver and rider pages).
func (s *Service) OpenCount(ctx context.Context, kind, id string) (int, error) {
	return s.store.openCount(ctx, kind, id)
}

// OpenTotal is how many flags wait for review in a country ("" = everywhere).
func (s *Service) OpenTotal(ctx context.Context, country string) (int, error) {
	return s.store.openTotal(ctx, country)
}

// RecordInstall notes the app install a person signed in from; a malformed id is ignored.
func (s *Service) RecordInstall(ctx context.Context, userID, app, installID string) error {
	u, err := uuid.Parse(installID)
	if err != nil || u.Version() != 4 || userID == "" {
		return nil
	}
	return s.store.recordInstall(ctx, userID, app, u.String())
}

// CheckTrip looks for signs a completed trip was booked by its own driver.
func (s *Service) CheckTrip(ctx context.Context, tripID string) error {
	country, driverID, parties, err := s.store.tripParties(ctx, tripID, s.now().Add(-InstallLookback))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil // no driver: nothing to check
		}
		return err
	}
	parties.PickupPhone, parties.ReceiverPhone = normalised(parties.PickupPhone, country), normalised(parties.ReceiverPhone, country)
	signals := SelfBookingSignals(parties)
	if len(signals) == 0 {
		return nil
	}
	_, err = s.Raise(ctx, Finding{Country: country, Rule: RuleSelfBooking, Severity: worstSeverity(signals), SubjectKind: SubjectDriver,
		SubjectID: driverID, TripID: tripID, DedupeKey: RuleSelfBooking + ":" + tripID, Evidence: map[string]any{"signals": signals}})
	return err
}

// GPSFlag raises the fake-GPS flag for a driver (from the ping-path guard's job).
func (s *Service) GPSFlag(ctx context.Context, a GPSFlagArgs) error {
	country, err := s.store.driverCountry(ctx, a.DriverID)
	if err != nil {
		return err
	}
	severity := SeverityMedium
	if a.Mocked {
		severity = SeverityHigh
	}
	day := a.At.In(s.location(country)).Format(time.DateOnly)
	_, err = s.Raise(ctx, Finding{Country: country, Rule: RuleGPSSpoof, Severity: severity, SubjectKind: SubjectDriver, SubjectID: a.DriverID,
		DedupeKey: fmt.Sprintf("%s:%s:%s", RuleGPSSpoof, a.DriverID, day), Evidence: map[string]any{"mocked": a.Mocked, "pairs": a.Pairs}})
	return err
}

// ScanCancellations runs the daily cancellation scan in each country whose
// local time is past CancelScanHour and that has not been scanned today.
func (s *Service) ScanCancellations(ctx context.Context) error {
	for _, c := range s.countries {
		local := s.now().In(s.location(c))
		if local.Hour() < CancelScanHour {
			continue
		}
		day := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
		ok, err := s.store.claimScan(ctx, c, day)
		if err != nil || !ok {
			if err != nil {
				return err
			}
			continue
		}
		n, err := s.scanCountry(ctx, c, local)
		if err != nil {
			return fmt.Errorf("cancel scan %s: %w", c, err)
		}
		if err := s.store.finishScan(ctx, c, day, n); err != nil {
			return err
		}
	}
	return nil
}

// scanCountry flags the rates over threshold in one country; returns how many flags it raised or hit.
func (s *Service) scanCountry(ctx context.Context, country string, local time.Time) (int, error) {
	since := s.now().Add(-CancelWindow)
	drivers, err := s.store.driverCounts(ctx, country, since)
	if err != nil {
		return 0, err
	}
	riders, err := s.store.riderCounts(ctx, country, since)
	if err != nil {
		return 0, err
	}
	year, week := local.ISOWeek()
	n := 0
	for _, c := range append(drivers, riders...) {
		for _, f := range CancelFindings(c) {
			key := fmt.Sprintf("%s:%s:%d-W%02d", f.Rule, c.SubjectID, year, week)
			if _, err := s.Raise(ctx, Finding{Country: country, Rule: f.Rule, Severity: SeverityMedium, SubjectKind: c.SubjectKind,
				SubjectID: c.SubjectID, DedupeKey: key, Evidence: f}); err != nil {
				return n, err
			}
			n++
		}
	}
	return n, nil
}

// normalised is a contact phone in the stored international form (contacts keep what was typed).
func normalised(raw, country string) string {
	if raw == "" {
		return ""
	}
	if p, err := phone.Normalize(raw, country); err == nil {
		return p
	}
	return raw
}

func (s *Service) location(country string) *time.Location {
	if s.tz != nil {
		if loc := s.tz(country); loc != nil {
			return loc
		}
	}
	return time.UTC
}
