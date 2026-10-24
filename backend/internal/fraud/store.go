package fraud

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// store holds the queries; no rules here.
type store struct {
	pool    *pgxpool.Pool
	brandID string
}

// raise inserts a finding, or adds a hit to the open flag with the same dedupe
// key. A reviewed flag with that key is left alone (the next period opens a new one).
func (s store) raise(ctx context.Context, f Finding) (string, error) {
	raw, err := json.Marshal(f.Evidence)
	if err != nil {
		return "", err
	}
	var id string
	err = s.pool.QueryRow(ctx, `
		INSERT INTO fraud_flags (brand_id, country_code, rule, severity, subject_kind, subject_id, trip_id, evidence, dedupe_key)
		VALUES ($1, NULLIF($2,''), $3, $4, $5, $6, NULLIF($7,'')::uuid, $8, $9)
		ON CONFLICT (brand_id, dedupe_key) DO UPDATE SET hits = fraud_flags.hits + 1, last_seen_at = now(),
		  evidence = EXCLUDED.evidence || jsonb_build_object('previous', fraud_flags.evidence - 'previous'),
		  severity = CASE WHEN 'HIGH' IN (fraud_flags.severity, EXCLUDED.severity) THEN 'HIGH'
		                  WHEN 'MEDIUM' IN (fraud_flags.severity, EXCLUDED.severity) THEN 'MEDIUM' ELSE 'LOW' END
		  WHERE fraud_flags.status = 'OPEN'
		RETURNING id`, s.brandID, f.Country, f.Rule, f.Severity, f.SubjectKind, f.SubjectID, f.TripID, raw, f.DedupeKey).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil // already reviewed in this period
	}
	return id, err
}

const flagColumns = `f.id, f.country_code, f.rule, f.severity, f.subject_kind, f.subject_id, u.full_name, u.phone, f.trip_id::text, t.code,
	f.evidence, f.hits, f.status, f.opened_at, f.last_seen_at, rv.full_name, f.reviewed_at, f.review_note`

const flagFrom = ` FROM fraud_flags f JOIN users u ON u.id = f.subject_id LEFT JOIN trips t ON t.id = f.trip_id LEFT JOIN users rv ON rv.id = f.reviewed_by`

func scanFlag(row pgx.Row) (Flag, error) {
	var f Flag
	var ev []byte
	err := row.Scan(&f.ID, &f.Country, &f.Rule, &f.Severity, &f.Subject.Kind, &f.Subject.ID, &f.Subject.Name, &f.Subject.Phone, &f.TripID, &f.TripCode,
		&ev, &f.Hits, &f.Status, &f.OpenedAt, &f.LastSeenAt, &f.ReviewedBy, &f.ReviewedAt, &f.ReviewNote)
	if errors.Is(err, pgx.ErrNoRows) {
		return f, ErrFlagNotFound
	}
	f.Evidence = json.RawMessage(ev)
	return f, err
}

// list reads the queue, newest first (uses fraud_flags_queue_idx).
func (s store) list(ctx context.Context, f Filter) ([]Flag, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+flagColumns+flagFrom+`
		WHERE f.brand_id=$1 AND ($2='' OR f.status=$2) AND ($3='' OR f.rule=$3) AND ($4='' OR f.subject_kind=$4)
		  AND ($5='' OR f.subject_id::text=$5) AND ($6='' OR f.country_code=$6) AND ($7::timestamptz IS NULL OR f.opened_at < $7)
		ORDER BY f.opened_at DESC LIMIT $8`, s.brandID, f.Status, f.Rule, f.SubjectKind, f.SubjectID, f.Country, f.Before, f.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Flag{}
	for rows.Next() {
		fl, err := scanFlag(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, fl)
	}
	return out, rows.Err()
}

func (s store) get(ctx context.Context, q pgx.Tx, id string, lock bool) (Flag, error) {
	sql := `SELECT ` + flagColumns + flagFrom + ` WHERE f.id::text=$1 AND f.brand_id=$2`
	if lock {
		sql += ` FOR UPDATE OF f`
	}
	if q == nil {
		return scanFlag(s.pool.QueryRow(ctx, sql, id, s.brandID))
	}
	return scanFlag(q.QueryRow(ctx, sql, id, s.brandID))
}

// openCount counts a person's open flags.
func (s store) openCount(ctx context.Context, kind, id string) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM fraud_flags WHERE subject_kind=$1 AND subject_id::text=$2 AND status='OPEN'`, kind, id).Scan(&n)
	return n, err
}

// openTotal counts open flags in a country ("" = all).
func (s store) openTotal(ctx context.Context, country string) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM fraud_flags WHERE brand_id=$1 AND status='OPEN' AND ($2='' OR country_code=$2)`, s.brandID, country).Scan(&n)
	return n, err
}

// recordInstall notes that a person signed in from an app install.
func (s store) recordInstall(ctx context.Context, userID, app, installID string) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO user_installs (user_id, install_id, app) VALUES ($1,$2,$3)
		ON CONFLICT (user_id, install_id) DO UPDATE SET last_seen_at = now(), app = EXCLUDED.app
		  WHERE user_installs.last_seen_at < now() - interval '1 day'`, userID, installID, app) // refreshes write at most daily
	return err
}

// tripParties reads a completed trip's driver, rider, contacts and shared installs (one query).
func (s store) tripParties(ctx context.Context, tripID string, since time.Time) (country, driverID string, p TripParties, err error) {
	err = s.pool.QueryRow(ctx, `
		SELECT coalesce(t.country_code,''), t.driver_id::text, d.phone,
		       coalesce(t.contacts->'pickup'->>'phone',''), coalesce(t.contacts->'dropoff'->>'phone',''),
		       coalesce(ARRAY(SELECT a.install_id::text FROM user_installs a JOIN user_installs b ON b.install_id = a.install_id
		         WHERE a.user_id = t.rider_id AND b.user_id = t.driver_id AND a.last_seen_at > $2 AND b.last_seen_at > $2), '{}')
		FROM trips t JOIN users d ON d.id = t.driver_id
		WHERE t.id = $1`, tripID, since).Scan(&country, &driverID, &p.DriverPhone, &p.PickupPhone, &p.ReceiverPhone, &p.SharedInstalls)
	return country, driverID, p, err
}

// driverCountry is the country a driver is registered in ("" if not a driver).
func (s store) driverCountry(ctx context.Context, driverID string) (string, error) {
	var c string
	err := s.pool.QueryRow(ctx, `SELECT country_code FROM driver_profiles WHERE user_id::text=$1`, driverID).Scan(&c)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return c, err
}

// claimScan marks a country's scan for a local day; false when it already ran.
func (s store) claimScan(ctx context.Context, country string, day time.Time) (bool, error) {
	// A run that started over an hour ago and never finished (an error or a
	// crash) is claimed again, so a failed day is retried, not skipped.
	tag, err := s.pool.Exec(ctx, `INSERT INTO fraud_scan_runs (brand_id, country_code, day, flags, ran_at) VALUES ($1,$2,$3,-1,now())
		ON CONFLICT (brand_id, country_code, day) DO UPDATE SET ran_at=now()
		WHERE fraud_scan_runs.flags < 0 AND fraud_scan_runs.ran_at < now() - interval '1 hour'`,
		s.brandID, country, day.Format(time.DateOnly))
	return tag.RowsAffected() == 1, err
}

func (s store) finishScan(ctx context.Context, country string, day time.Time, flags int) error {
	_, err := s.pool.Exec(ctx, `UPDATE fraud_scan_runs SET flags=$4, ran_at=now() WHERE brand_id=$1 AND country_code=$2 AND day=$3`,
		s.brandID, country, day.Format(time.DateOnly), flags)
	return err
}

// driverCounts reads every driver's accepts, cancels, arrivals and no-shows on
// the country's trips requested since `since`, from the trip events (a driver
// cancel puts the trip back into matching, so the trip row forgets the driver).
func (s store) driverCounts(ctx context.Context, country string, since time.Time) ([]CancelCounts, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT e.actor_id::text,
		       count(*) FILTER (WHERE e.type='trip.accepted'),
		       count(*) FILTER (WHERE e.type='trip.driver_cancelled'),
		       count(*) FILTER (WHERE e.type='trip.status' AND e.payload->>'status'='ARRIVED_AT_PICKUP'),
		       count(*) FILTER (WHERE e.type='trip.no_show'),
		       coalesce((array_agg(DISTINCT e.trip_id::text) FILTER (WHERE e.type IN ('trip.driver_cancelled','trip.no_show')))[1:$4], '{}')
		FROM trips t JOIN trip_events e ON e.trip_id = t.id
		WHERE t.brand_id=$1 AND t.country_code=$2 AND t.requested_at >= $3 AND e.actor_kind='DRIVER' AND e.actor_id IS NOT NULL
		  AND e.type IN ('trip.accepted','trip.driver_cancelled','trip.status','trip.no_show')
		GROUP BY e.actor_id`, s.brandID, country, since, evidenceTripIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CancelCounts
	for rows.Next() {
		c := CancelCounts{SubjectKind: SubjectDriver}
		if err := rows.Scan(&c.SubjectID, &c.Accepted, &c.Cancels, &c.Arrivals, &c.NoShows, &c.CancelTripIDs); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// riderCounts reads every rider's trips that got a driver, and how many of those they cancelled.
func (s store) riderCounts(ctx context.Context, country string, since time.Time) ([]CancelCounts, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT x.rider_id::text, count(*) FILTER (WHERE x.assigned), count(*) FILTER (WHERE x.assigned AND x.cancelled),
		       coalesce((array_agg(x.id::text) FILTER (WHERE x.assigned AND x.cancelled))[1:$4], '{}')
		FROM (SELECT t.id, t.rider_id,
		             EXISTS (SELECT 1 FROM trip_events e WHERE e.trip_id=t.id AND e.type='trip.accepted') AS assigned,
		             t.status = 'CANCELLED_BY_RIDER' AS cancelled
		      FROM trips t WHERE t.brand_id=$1 AND t.country_code=$2 AND t.requested_at >= $3) x
		GROUP BY x.rider_id HAVING count(*) FILTER (WHERE x.assigned) > 0`, s.brandID, country, since, evidenceTripIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CancelCounts
	for rows.Next() {
		c := CancelCounts{SubjectKind: SubjectRider}
		if err := rows.Scan(&c.SubjectID, &c.Accepted, &c.Cancels, &c.CancelTripIDs); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
