package dispatch

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kalen-limited/hauler-rider/backend/internal/audit"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/cache"
	"github.com/kalen-limited/hauler-rider/backend/internal/trips"
)

// QueueSettings are a country's trip-queue switches: whether a driver who is
// finishing a trip is offered the next one, and which trip statuses count as
// finishing. Countries never set use DefaultQueueSettings (on).
type QueueSettings struct {
	Country           string         `json:"country"`
	Enabled           bool           `json:"trip_queue_enabled"`
	FinishingStatuses []trips.Status `json:"finishing_statuses"`
	UpdatedAt         *time.Time     `json:"updated_at,omitempty"`
}

// FinishingChoices are the statuses an operator may count as finishing: the
// ride is under way and the driver will be free at its drop-off.
var FinishingChoices = []trips.Status{trips.InTransit, trips.ArrivedAtDropoff, trips.AwaitingPayment}

// defaultFinishing: at the drop-off, or collecting the fare.
var defaultFinishing = []trips.Status{trips.ArrivedAtDropoff, trips.AwaitingPayment}

// DefaultQueueSettings is what a country without a saved row gets.
func DefaultQueueSettings(country string) QueueSettings {
	return QueueSettings{Country: country, Enabled: true, FinishingStatuses: slices.Clone(defaultFinishing)}
}

// IsFinishing reports whether a trip in status st makes its driver finishing.
// It ignores Enabled on purpose: drivers are marked finishing either way and
// matching checks Enabled, so switching the queue on reaches drivers who are
// already at a drop-off.
func (q QueueSettings) IsFinishing(st trips.Status) bool {
	return slices.Contains(q.FinishingStatuses, st)
}

// QueueSettingsInput changes a country's switches (nil = unchanged).
type QueueSettingsInput struct {
	Enabled           *bool          `json:"trip_queue_enabled"`
	FinishingStatuses []trips.Status `json:"finishing_statuses"`
	Reason            string         `json:"reason"`
}

// Error codes of the settings.
var (
	ErrReasonRequired    = apperr.New("REASON_REQUIRED", http.StatusBadRequest, "")
	ErrCountryUnknown    = apperr.New("COUNTRY_UNKNOWN", http.StatusNotFound, "")
	ErrFinishingStatuses = apperr.New("FINISHING_STATUSES_INVALID", http.StatusBadRequest, "")
)

// settingsScope is the config_versions scope bumped on every write.
const settingsScope = "dispatch"

// settingsCheckTTL bounds how long a change takes to reach every dispatcher.
const settingsCheckTTL = 5 * time.Second

// auditAction names the audit entry of a settings change.
const auditAction = "dispatch.settings"

// Settings reads and writes the per-country dispatch settings. Reads come from
// an in-process cache keyed by the "dispatch" config version, so matching never
// queries Postgres for them.
type Settings struct {
	store     settingsStore
	countries []string
	cached    *cache.Versioned[map[string]QueueSettings]
}

// NewSettings builds the settings for a brand's countries.
func NewSettings(pool *pgxpool.Pool, brandID string, countries []string) *Settings {
	st := settingsStore{pool: pool, brandID: brandID}
	return &Settings{store: st, countries: countries, cached: cache.New(settingsScope, st, st.all, settingsCheckTTL)}
}

// For returns a country's settings; defaults when none are saved or the
// store cannot be read (the queue is an optimisation, never a blocker).
func (s *Settings) For(ctx context.Context, country string) QueueSettings {
	if s == nil {
		return DefaultQueueSettings(country)
	}
	all, _, err := s.cached.Get(ctx)
	if q, ok := all[country]; ok && err == nil {
		return q
	}
	return DefaultQueueSettings(country)
}

// List returns every brand country's settings for the admin page.
func (s *Settings) List(ctx context.Context) ([]QueueSettings, error) {
	all, err := s.store.all(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]QueueSettings, 0, len(s.countries))
	for _, c := range s.countries {
		q, ok := all[c]
		if !ok {
			q = DefaultQueueSettings(c)
		}
		out = append(out, q)
	}
	return out, nil
}

// Set changes one country's settings, audited, and bumps the config version.
func (s *Settings) Set(ctx context.Context, staffID, country string, in QueueSettingsInput) (QueueSettings, error) {
	country = strings.ToUpper(strings.TrimSpace(country))
	if strings.TrimSpace(in.Reason) == "" {
		return QueueSettings{}, ErrReasonRequired
	}
	if !slices.Contains(s.countries, country) {
		return QueueSettings{}, ErrCountryUnknown
	}
	q := s.For(ctx, country)
	if err := apply(&q, in); err != nil {
		return QueueSettings{}, err
	}
	err := pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		if err := s.store.save(ctx, tx, q, staffID); err != nil {
			return err
		}
		return audit.Record(ctx, tx, s.store.brandID, staffID, auditAction, "dispatch:"+country, in.Reason, q)
	})
	if err != nil {
		return QueueSettings{}, err
	}
	now := time.Now()
	q.UpdatedAt = &now
	return q, nil
}

// apply validates and merges an input into the current settings.
func apply(q *QueueSettings, in QueueSettingsInput) error {
	if in.Enabled != nil {
		q.Enabled = *in.Enabled
	}
	if in.FinishingStatuses == nil {
		return nil
	}
	if !validFinishing(in.FinishingStatuses) {
		return ErrFinishingStatuses.WithDetails(map[string]any{"allowed": FinishingChoices})
	}
	q.FinishingStatuses = dedupe(in.FinishingStatuses)
	return nil
}

// validFinishing: at least one status, each one of FinishingChoices.
func validFinishing(list []trips.Status) bool {
	if len(list) == 0 {
		return false
	}
	for _, st := range list {
		if !slices.Contains(FinishingChoices, st) {
			return false
		}
	}
	return true
}

// dedupe keeps each status once, in FinishingChoices order.
func dedupe(list []trips.Status) []trips.Status {
	out := []trips.Status{}
	for _, st := range FinishingChoices {
		if slices.Contains(list, st) {
			out = append(out, st)
		}
	}
	return out
}
