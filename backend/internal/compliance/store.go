package compliance

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type store struct {
	pool    *pgxpool.Pool
	brandID string
}

// subject is what a submission is about.
type subject struct {
	country, regulator, kind, key string
	driverID, vehicleID, tripID   *string
}

// enqueueTx queues (or re-queues) a submission in the caller's transaction:
// a new row is due now; an existing one is reset to PENDING, due now, with its
// attempts cleared (the details changed, so it must be sent again).
func (s store) enqueueTx(ctx context.Context, tx pgx.Tx, sub subject) error {
	_, err := tx.Exec(ctx, `INSERT INTO regulator_submissions (brand_id, country_code, regulator, kind, subject_key, driver_id, vehicle_id, trip_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (brand_id, regulator, kind, subject_key) DO UPDATE SET status='PENDING', attempts=0, next_attempt_at=now(),
		  stuck_at=NULL, last_code=NULL, last_error=NULL, vehicle_id=COALESCE(EXCLUDED.vehicle_id, regulator_submissions.vehicle_id), updated_at=now()`,
		s.brandID, sub.country, sub.regulator, sub.kind, sub.key, sub.driverID, sub.vehicleID, sub.tripID)
	return err
}

// enqueueOnceTx queues a submission only if none exists (trips and days are sent once).
func (s store) enqueueOnceTx(ctx context.Context, q interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}, sub subject) error {
	_, err := q.Exec(ctx, `INSERT INTO regulator_submissions (brand_id, country_code, regulator, kind, subject_key, driver_id, vehicle_id, trip_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT (brand_id, regulator, kind, subject_key) DO NOTHING`,
		s.brandID, sub.country, sub.regulator, sub.kind, sub.key, sub.driverID, sub.vehicleID, sub.tripID)
	return err
}

// claimed is a due row taken by one sweeper for the lease.
type claimed struct {
	ID, Country, Regulator, Kind, SubjectKey string
	DriverID, VehicleID, TripID              *string
	Attempts                                 int
}

// claimDue takes due rows of the countries that are sending, pushing them a lease ahead
// so a concurrent sweeper skips them (each row is sent by one sweeper at a time).
func (s store) claimDue(ctx context.Context, countries []string, limit int) ([]claimed, error) {
	rows, err := s.pool.Query(ctx, `UPDATE regulator_submissions r SET next_attempt_at = now() + $3::interval
		WHERE r.id IN (SELECT id FROM regulator_submissions WHERE brand_id=$1 AND status='PENDING' AND next_attempt_at <= now()
		  AND country_code = ANY($2) ORDER BY next_attempt_at LIMIT $4 FOR UPDATE SKIP LOCKED)
		RETURNING r.id, r.country_code, r.regulator, r.kind, r.subject_key, r.driver_id, r.vehicle_id, r.trip_id, r.attempts`,
		s.brandID, countries, ClaimLease, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[claimed])
}

// accept records an accepted submission with the regulator's reference and what was sent.
func (s store) accept(ctx context.Context, id, code, reference string, payload any) error {
	raw, _ := json.Marshal(payload)
	_, err := s.pool.Exec(ctx, `UPDATE regulator_submissions SET status='ACCEPTED', last_code=NULLIF($2,''), reference=NULLIF($3,''),
		payload=$4, accepted_at=now(), last_attempt_at=now(), attempts=attempts+1, stuck_at=NULL, last_error=NULL, updated_at=now()
		WHERE id=$1 AND status='PENDING'`, id, code, reference, raw)
	return err
}

// skip closes a submission that has nothing to send.
func (s store) skip(ctx context.Context, id, code, msg string) error {
	_, err := s.pool.Exec(ctx, `UPDATE regulator_submissions SET status='SKIPPED', last_code=NULLIF($2,''), last_error=NULLIF($3,''),
		last_attempt_at=now(), updated_at=now() WHERE id=$1 AND status='PENDING'`, id, code, msg)
	return err
}

// fail counts an attempt and schedules the next one; stuck is stamped once.
func (s store) fail(ctx context.Context, id, code, msg string, next time.Time, stuck bool) error {
	_, err := s.pool.Exec(ctx, `UPDATE regulator_submissions SET attempts=attempts+1, last_attempt_at=now(), next_attempt_at=$4,
		last_code=NULLIF($2,''), last_error=NULLIF($3,''), stuck_at=CASE WHEN $5 THEN COALESCE(stuck_at, now()) ELSE stuck_at END, updated_at=now()
		WHERE id=$1 AND status='PENDING'`, id, code, msg, next, stuck)
	return err
}

// wait puts a claimed row back without counting an attempt (its prerequisites come first).
func (s store) wait(ctx context.Context, id string, next time.Time) error {
	_, err := s.pool.Exec(ctx, `UPDATE regulator_submissions SET next_attempt_at=$2, updated_at=now() WHERE id=$1 AND status='PENDING'`, id, next)
	return err
}

// statusOf is a subject's submission status ("" when none).
func (s store) statusOf(ctx context.Context, regulatorName, kind, key string) (string, error) {
	var st string
	err := s.pool.QueryRow(ctx, `SELECT status FROM regulator_submissions WHERE brand_id=$1 AND regulator=$2 AND kind=$3 AND subject_key=$4`,
		s.brandID, regulatorName, kind, key).Scan(&st)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return st, err
}

// makeDue brings a prerequisite (driver or vehicle registration) forward.
func (s store) makeDue(ctx context.Context, regulatorName, kind, key string) error {
	_, err := s.pool.Exec(ctx, `UPDATE regulator_submissions SET next_attempt_at=LEAST(next_attempt_at, now()), updated_at=now()
		WHERE brand_id=$1 AND regulator=$2 AND kind=$3 AND subject_key=$4 AND status='PENDING'`, s.brandID, regulatorName, kind, key)
	return err
}

// settings returns a country's switches (all off when never set).
func (s store) settings(ctx context.Context, country string) (Settings, error) {
	var st Settings
	err := s.pool.QueryRow(ctx, `SELECT enabled, fail_closed FROM regulator_settings WHERE brand_id=$1 AND country_code=$2`, s.brandID, country).
		Scan(&st.Enabled, &st.FailClosed)
	if errors.Is(err, pgx.ErrNoRows) {
		return Settings{}, nil
	}
	return st, err
}

func (s store) saveSettings(ctx context.Context, tx pgx.Tx, country string, st Settings, staffID string) error {
	_, err := tx.Exec(ctx, `INSERT INTO regulator_settings (brand_id, country_code, enabled, fail_closed, updated_by) VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (brand_id, country_code) DO UPDATE SET enabled=EXCLUDED.enabled, fail_closed=EXCLUDED.fail_closed,
		  updated_by=EXCLUDED.updated_by, updated_at=now()`, s.brandID, country, st.Enabled, st.FailClosed, staffID)
	return err
}

// counts fills a country's figures for the admin page in one query.
func (s store) counts(ctx context.Context, st *Status, since time.Time) error {
	return s.pool.QueryRow(ctx, `SELECT
		  count(*) FILTER (WHERE status='PENDING'),
		  count(*) FILTER (WHERE status='PENDING' AND stuck_at IS NOT NULL),
		  count(*) FILTER (WHERE status='ACCEPTED' AND accepted_at >= $3),
		  count(*) FILTER (WHERE status='SKIPPED' AND updated_at >= $3),
		  max(accepted_at),
		  (SELECT last_error FROM regulator_submissions WHERE brand_id=$1 AND country_code=$2 AND status='PENDING' AND last_error IS NOT NULL
		     ORDER BY last_attempt_at DESC NULLS LAST LIMIT 1)
		FROM regulator_submissions WHERE brand_id=$1 AND country_code=$2`, s.brandID, st.Country, since).
		Scan(&st.Pending, &st.Stuck, &st.AcceptedToday, &st.SkippedToday, &st.LastAcceptedAt, &st.LastError)
}

// list returns submissions with a readable subject label, newest first, in one query.
func (s store) list(ctx context.Context, f Filter) ([]Submission, error) {
	rows, err := s.pool.Query(ctx, `SELECT r.id, r.country_code, r.regulator, r.kind, r.subject_key,
		  CASE r.kind WHEN 'TRIP' THEN coalesce(t.code, r.subject_key) WHEN 'VEHICLE' THEN coalesce(v.plate, r.subject_key)
		    WHEN 'DRIVER' THEN coalesce(u.full_name, u.phone, r.subject_key) ELSE r.subject_key END,
		  r.driver_id, r.trip_id, r.status, r.attempts, r.next_attempt_at, r.last_code, r.last_error, r.reference, r.accepted_at,
		  r.stuck_at IS NOT NULL, r.created_at
		FROM regulator_submissions r
		LEFT JOIN trips t ON t.id=r.trip_id LEFT JOIN vehicles v ON v.id=r.vehicle_id LEFT JOIN users u ON u.id=r.driver_id
		WHERE r.brand_id=$1 AND ($2='' OR r.country_code=$2) AND ($3='' OR r.kind=$3) AND ($4='' OR r.status=$4)
		  AND (NOT $5 OR r.stuck_at IS NOT NULL)
		  AND ($6='' OR t.code ILIKE $6 || '%' OR v.plate ILIKE '%' || $6 || '%' OR u.full_name ILIKE '%' || $6 || '%' OR r.subject_key = $6)
		ORDER BY r.created_at DESC LIMIT $7`, s.brandID, f.Country, f.Kind, f.Status, f.Stuck, f.Q, f.Limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[Submission])
}

// get loads one submission.
func (s store) get(ctx context.Context, id string) (*Submission, error) {
	rows, err := s.pool.Query(ctx, `SELECT r.id, r.country_code, r.regulator, r.kind, r.subject_key, r.subject_key, r.driver_id, r.trip_id,
		  r.status, r.attempts, r.next_attempt_at, r.last_code, r.last_error, r.reference, r.accepted_at, r.stuck_at IS NOT NULL, r.created_at
		FROM regulator_submissions r WHERE r.brand_id=$1 AND r.id=$2`, s.brandID, id)
	if err != nil {
		return nil, err
	}
	sub, err := pgx.CollectOneRow(rows, pgx.RowToStructByPos[Submission])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &sub, err
}

// retryNow makes a pending submission due at once.
func (s store) retryNow(ctx context.Context, tx pgx.Tx, id string) (bool, error) {
	tag, err := tx.Exec(ctx, `UPDATE regulator_submissions SET next_attempt_at=now(), updated_at=now() WHERE brand_id=$1 AND id=$2 AND status='PENDING'`, s.brandID, id)
	return tag.RowsAffected() > 0, err
}

// badges returns the latest standing of a set of subjects (one query).
func (s store) badges(ctx context.Context, kind string, keys []string) (map[string]Badge, error) {
	out := map[string]Badge{}
	rows, err := s.pool.Query(ctx, `SELECT subject_key, status, reference, last_error FROM regulator_submissions
		WHERE brand_id=$1 AND kind=$2 AND subject_key = ANY($3)`, s.brandID, kind, keys)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var key string
		var b Badge
		if err := rows.Scan(&key, &b.Status, &b.Reference, &b.LastError); err != nil {
			return nil, err
		}
		out[key] = b
	}
	return out, rows.Err()
}
