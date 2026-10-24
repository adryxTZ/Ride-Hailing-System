package privacy

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/brand"
)

// store holds the queries; no rules here.
type store struct {
	pool    *pgxpool.Pool
	brandID string
}

func (s store) upsertDocument(ctx context.Context, d brand.ConsentDoc) error {
	title, _ := json.Marshal(d.Title)
	summary, _ := json.Marshal(d.Summary)
	url, _ := json.Marshal(d.URL)
	_, err := s.pool.Exec(ctx, `INSERT INTO consent_documents (brand_id, app, version, title, summary, url, published_at) VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (brand_id, app, version) DO UPDATE SET title=EXCLUDED.title, summary=EXCLUDED.summary, url=EXCLUDED.url, published_at=EXCLUDED.published_at`,
		s.brandID, d.App, d.Version, title, summary, url, d.PublishedAt)
	return err
}

func (s store) acceptedVersion(ctx context.Context, userID, app string) (*int, error) {
	var v *int
	err := s.pool.QueryRow(ctx, `SELECT max(version) FROM user_consents WHERE user_id::text=$1 AND app=$2`, userID, app).Scan(&v)
	return v, err
}

func (s store) insertConsent(ctx context.Context, userID string, in AcceptInput, install string) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO user_consents (user_id, app, version, install_id, ip, user_agent)
		VALUES ($1,$2,$3,NULLIF($4,'')::uuid,NULLIF($5,'')::inet,NULLIF($6,'')) ON CONFLICT (user_id, app, version) DO NOTHING`,
		userID, in.App, in.Version, install, in.IP, in.UserAgent)
	return err
}

const requestColumns = `r.id, r.user_id, u.full_name, u.phone, r.app, r.kind, r.status, r.reason, r.created_at, r.due_at, h.full_name, r.handled_at,
	r.export_expires_at, r.blockers, r.country_code`

func scanRequest(row pgx.Row) (*Request, error) {
	var r Request
	var blockers []byte
	err := row.Scan(&r.ID, &r.UserID, &r.UserName, &r.UserPhone, &r.App, &r.Kind, &r.Status, &r.Reason, &r.CreatedAt, &r.DueAt, &r.HandledBy, &r.HandledAt,
		&r.DownloadExpiresAt, &blockers, &r.Country)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	r.Blockers = []string{}
	_ = json.Unmarshal(blockers, &r.Blockers)
	return &r, err
}

const requestFrom = ` FROM privacy_requests r JOIN users u ON u.id=r.user_id LEFT JOIN users h ON h.id=r.handled_by`

func (s store) insertRequest(ctx context.Context, userID, app, kind string, due time.Time) (string, error) {
	var id string
	// The person's country: a driver's registration, else a rider's latest trip, else the brand's home.
	err := s.pool.QueryRow(ctx, `INSERT INTO privacy_requests (brand_id, user_id, app, kind, due_at, country_code) VALUES ($1,$2,$3,$4,$5,
		coalesce((SELECT country_code FROM driver_profiles WHERE user_id=$2),
		         (SELECT country_code FROM trips WHERE rider_id=$2 AND country_code IS NOT NULL ORDER BY requested_at DESC LIMIT 1),
		         (SELECT country FROM brands WHERE id=$1))) RETURNING id`,
		s.brandID, userID, app, kind, due).Scan(&id)
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" {
		var open string
		_ = s.pool.QueryRow(ctx, `SELECT id FROM privacy_requests WHERE user_id=$1 AND kind=$2 AND status IN ('REQUESTED','IN_PROGRESS','READY')`, userID, kind).Scan(&open)
		return "", ErrRequestOpen.WithDetails(map[string]any{"request_id": open})
	}
	return id, err
}

// request reads one request (locked inside a transaction when lock is set).
func (s store) request(ctx context.Context, tx pgx.Tx, id string, lock bool) (*Request, error) {
	sql := `SELECT ` + requestColumns + requestFrom + ` WHERE r.id::text=$1 AND r.brand_id=$2`
	if lock {
		sql += ` FOR UPDATE OF r`
	}
	if tx != nil {
		return scanRequest(tx.QueryRow(ctx, sql, id, s.brandID))
	}
	return scanRequest(s.pool.QueryRow(ctx, sql, id, s.brandID))
}

func (s store) collect(rows pgx.Rows, err error) ([]Request, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Request{}
	for rows.Next() {
		r, err := scanRequest(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *r)
	}
	return out, rows.Err()
}

func (s store) mine(ctx context.Context, userID string) ([]Request, error) {
	return s.collect(s.pool.Query(ctx, `SELECT `+requestColumns+requestFrom+` WHERE r.user_id::text=$1 ORDER BY r.created_at DESC LIMIT 20`, userID))
}

func (s store) list(ctx context.Context, f Filter) ([]Request, error) {
	return s.collect(s.pool.Query(ctx, `SELECT `+requestColumns+requestFrom+`
		WHERE r.brand_id=$1 AND ($2='' OR r.status=$2) AND ($3='' OR r.kind=$3) AND ($4='' OR r.country_code=$4)
		ORDER BY (r.status IN ('REQUESTED','IN_PROGRESS')) DESC, r.due_at, r.created_at DESC LIMIT $5`, s.brandID, f.Status, f.Kind, f.Country, f.Limit))
}

func (s store) overdueCount(ctx context.Context, country string) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM privacy_requests WHERE brand_id=$1 AND ($2='' OR country_code=$2)
		AND status IN ('REQUESTED','IN_PROGRESS') AND due_at < now()`, s.brandID, country).Scan(&n)
	return n, err
}

func (s store) cancel(ctx context.Context, userID, id string) (bool, error) {
	tag, err := s.pool.Exec(ctx, `UPDATE privacy_requests SET status='CANCELLED', handled_at=now() WHERE id=$1 AND user_id::text=$2 AND status='REQUESTED'`, id, userID)
	return tag.RowsAffected() == 1, err
}

func (s store) export(ctx context.Context, userID, id string) (*Request, string, error) {
	r, err := s.request(ctx, nil, id, false)
	if err != nil {
		return nil, "", err
	}
	if r.UserID != userID {
		return nil, "", ErrNotFound
	}
	var key *string
	err = s.pool.QueryRow(ctx, `SELECT export_key FROM privacy_requests WHERE id=$1`, id).Scan(&key)
	if key == nil {
		return r, "", err
	}
	return r, *key, err
}

func (s store) setStatus(ctx context.Context, tx pgx.Tx, id, status, staffID, reason string) error {
	_, err := tx.Exec(ctx, `UPDATE privacy_requests SET status=$2, handled_by=$3, handled_at=now(), reason=$4 WHERE id=$1`, id, status, staffID, reason)
	return err
}

func (s store) saveBlockers(ctx context.Context, id string, blockers []string) error {
	raw, _ := json.Marshal(blockers)
	_, err := s.pool.Exec(ctx, `UPDATE privacy_requests SET blockers=$2 WHERE id=$1`, id, raw)
	return err
}

func (s store) markReady(ctx context.Context, id, key string, expires time.Time) error {
	_, err := s.pool.Exec(ctx, `UPDATE privacy_requests SET status='READY', export_key=$2, export_expires_at=$3 WHERE id=$1 AND status='IN_PROGRESS'`, id, key, expires)
	return err
}

// openWork is what the ledger cannot answer about a deletion.
type openWork struct{ activeTrip, openPayout, owedToRider bool }

func (s store) openWork(ctx context.Context, userID string) (openWork, error) {
	var w openWork
	err := s.pool.QueryRow(ctx, `SELECT
		EXISTS (SELECT 1 FROM trips WHERE (rider_id::text=$1 OR driver_id::text=$1) AND status NOT IN ('COMPLETED','CANCELLED_BY_RIDER','CANCELLED_BY_DRIVER','CANCELLED_BY_STAFF','NO_DRIVER_FOUND','EXPIRED')),
		EXISTS (SELECT 1 FROM driver_payouts WHERE driver_id::text=$1 AND status NOT IN ('PAID','CANCELLED','REJECTED')),
		coalesce((SELECT sum(e.amount) FROM ledger_entries e JOIN ledger_accounts a ON a.id=e.account_id
		          WHERE a.kind='RIDER_REFUNDS' AND a.owner_id::text=$1), 0) <> 0`, userID).Scan(&w.activeTrip, &w.openPayout, &w.owedToRider)
	return w, err
}

// anonymise strips a person's identity and personal data in one transaction.
func (s store) anonymise(ctx context.Context, tx pgx.Tx, userID string) (*erased, error) {
	e := &erased{userID: userID}
	var photo *string
	err := tx.QueryRow(ctx, `UPDATE users SET full_name=NULL, email=NULL, phone='deleted:'||id::text, password_hash=NULL, photo_key=NULL,
		status='DELETED', deleted_at=now(), updated_at=now() WHERE id::text=$1
		RETURNING (SELECT photo_key FROM users WHERE id::text=$1), EXISTS (SELECT 1 FROM driver_profiles WHERE user_id::text=$1)`, userID).Scan(&photo, &e.driver)
	if err != nil {
		return nil, err
	}
	if photo != nil {
		e.photoKey = *photo
	}
	for _, sql := range []string{
		`UPDATE driver_profiles SET national_id=NULL, licence_number=NULL, licence_expiry=NULL, licence_class=NULL WHERE user_id::text=$1`,
		`DELETE FROM saved_places WHERE user_id::text=$1`,
		`DELETE FROM trusted_contacts WHERE user_id::text=$1`,
		`DELETE FROM devices WHERE user_id::text=$1`,
		`DELETE FROM user_installs WHERE user_id::text=$1`,
		`DELETE FROM refresh_tokens WHERE user_id::text=$1`,
		`UPDATE sessions SET ended_at=coalesce(ended_at, now()), ip=NULL, user_agent=NULL, device=NULL, location=NULL WHERE user_id::text=$1`,
		`UPDATE trip_messages SET body='` + removedMessage + `' WHERE sender_id::text=$1`,
	} {
		if _, err := tx.Exec(ctx, sql, userID); err != nil {
			return nil, err
		}
	}
	return e, nil
}

type fileRow struct{ id, key string }

// pickFiles runs a file-picking query: $1 cutoff, $2 limit, $3 brand.
func (s store) pickFiles(ctx context.Context, sql string, cutoff time.Time, limit int) ([]fileRow, error) {
	rows, err := s.pool.Query(ctx, sql, cutoff, limit, s.brandID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []fileRow
	for rows.Next() {
		var r fileRow
		if err := rows.Scan(&r.id, &r.key); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s store) markPurged(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `UPDATE documents SET purged_at=now() WHERE id=$1`, id)
	return err
}

func (s store) oldExports(ctx context.Context, cutoff time.Time, limit int) ([]fileRow, error) {
	return s.pickFiles(ctx, `SELECT id::text, export_key FROM privacy_requests WHERE brand_id=$3 AND export_key IS NOT NULL AND export_expires_at < $1 LIMIT $2`, cutoff, limit)
}

func (s store) closeExport(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `UPDATE privacy_requests SET export_key=NULL, status=CASE WHEN status='READY' THEN 'DONE' ELSE status END WHERE id=$1`, id)
	return err
}

func (s store) claimRun(ctx context.Context, rule string, day time.Time) (bool, error) {
	// A rule that failed or never finished is claimed again an hour later.
	tag, err := s.pool.Exec(ctx, `INSERT INTO retention_runs (brand_id, rule, run_date) VALUES ($1,$2,$3)
		ON CONFLICT (brand_id, rule, run_date) DO UPDATE SET started_at=now(), finished_at=NULL, error=NULL
		WHERE (retention_runs.error IS NOT NULL OR retention_runs.finished_at IS NULL) AND retention_runs.started_at < now() - interval '1 hour'`,
		s.brandID, rule, day.Format(time.DateOnly))
	return tag.RowsAffected() == 1, err
}

func (s store) finishRun(ctx context.Context, rule string, day time.Time, rows int, runErr error) error {
	var msg *string
	if runErr != nil {
		m := runErr.Error()
		msg = &m
	}
	_, err := s.pool.Exec(ctx, `UPDATE retention_runs SET finished_at=now(), rows=$4, error=$5 WHERE brand_id=$1 AND rule=$2 AND run_date=$3`,
		s.brandID, rule, day.Format(time.DateOnly), rows, msg)
	return err
}

func (s store) lastRuns(ctx context.Context) (map[string]RetentionStatus, error) {
	rows, err := s.pool.Query(ctx, `SELECT DISTINCT ON (rule) rule, coalesce(finished_at, started_at), rows, error FROM retention_runs
		WHERE brand_id=$1 ORDER BY rule, run_date DESC`, s.brandID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]RetentionStatus{}
	for rows.Next() {
		var name string
		var st RetentionStatus
		if err := rows.Scan(&name, &st.LastRunAt, &st.RowsLast, &st.Error); err != nil {
			return nil, err
		}
		out[name] = st
	}
	return out, rows.Err()
}
