package dispatch

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kalen-limited/hauler-rider/backend/internal/trips"
)

// settingsStore persists dispatch_settings (no rules) and is the cache's
// version source.
type settingsStore struct {
	pool    *pgxpool.Pool
	brandID string
}

// Version implements cache.VersionSource for the "dispatch" scope.
func (s settingsStore) Version(ctx context.Context, scope string) (int64, error) {
	var v int64
	err := s.pool.QueryRow(ctx, `SELECT version FROM config_versions WHERE scope=$1`, scope).Scan(&v)
	return v, err
}

// all reads every saved country row of the brand, keyed by country code.
func (s settingsStore) all(ctx context.Context) (map[string]QueueSettings, error) {
	rows, err := s.pool.Query(ctx, `SELECT country_code, trip_queue_enabled, finishing_statuses, updated_at
		FROM dispatch_settings WHERE brand_id=$1`, s.brandID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]QueueSettings{}
	for rows.Next() {
		var q QueueSettings
		var statuses []string
		var at time.Time
		if err := rows.Scan(&q.Country, &q.Enabled, &statuses, &at); err != nil {
			return nil, err
		}
		q.UpdatedAt = &at
		for _, st := range statuses {
			q.FinishingStatuses = append(q.FinishingStatuses, trips.Status(st))
		}
		out[q.Country] = q
	}
	return out, rows.Err()
}

// save upserts a country row and bumps the config version in the same transaction.
func (s settingsStore) save(ctx context.Context, tx pgx.Tx, q QueueSettings, staffID string) error {
	statuses := make([]string, len(q.FinishingStatuses))
	for i, st := range q.FinishingStatuses {
		statuses[i] = string(st)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO dispatch_settings (brand_id, country_code, trip_queue_enabled, finishing_statuses, updated_by)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (brand_id, country_code) DO UPDATE SET trip_queue_enabled=EXCLUDED.trip_queue_enabled,
		  finishing_statuses=EXCLUDED.finishing_statuses, updated_by=EXCLUDED.updated_by, updated_at=now()`,
		s.brandID, q.Country, q.Enabled, statuses, staffID); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `UPDATE config_versions SET version = version + 1, updated_at = now() WHERE scope=$1`, settingsScope)
	return err
}
