package payments

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// pgUniqueViolation is the Postgres error code for a duplicate key.
const pgUniqueViolation = "23505"

const payinColumns = `id, payer_id, status, failure_code, amount, currency, phone, gateway, provider_ref, created_at, completed_at, expires_at, purpose, trip_id`

type store struct {
	pool    *pgxpool.Pool
	brandID string
}

// byKey returns the pay-in a driver created with this idempotency key, or nil.
func (s *store) byKey(ctx context.Context, driverID, key string) (*Payin, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+payinColumns+` FROM gateway_transactions
		WHERE brand_id=$1 AND payer_id=$2 AND idempotency_key=$3`, s.brandID, driverID, key)
	if err != nil {
		return nil, err
	}
	return one(rows)
}

// get returns one pay-in by id, scoped to its driver when driverID is set.
func (s *store) get(ctx context.Context, id, driverID string) (*Payin, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+payinColumns+` FROM gateway_transactions
		WHERE brand_id=$1 AND id=$2 AND ($3 = '' OR payer_id=$3::uuid)`, s.brandID, id, driverID)
	if err != nil {
		return nil, err
	}
	return one(rows)
}

// pending returns the driver's open pay-in, or nil.
func (s *store) pending(ctx context.Context, driverID string) (*Payin, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+payinColumns+` FROM gateway_transactions
		WHERE brand_id=$1 AND payer_id=$2 AND purpose=$3 AND status='PENDING'`, s.brandID, driverID, purposeDriverDebt)
	if err != nil {
		return nil, err
	}
	return one(rows)
}

// insert creates a PENDING row with the gateway reference already known; a
// second open pay-in for the driver is ErrInProgress.
func (s *store) insert(ctx context.Context, tx pgx.Tx, p Payin, key string) (string, error) {
	var id string
	err := tx.QueryRow(ctx, `INSERT INTO gateway_transactions
		(id, brand_id, purpose, payer_id, phone, amount, currency, gateway, idempotency_key, expires_at, provider_ref, trip_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) RETURNING id`,
		p.ID, s.brandID, p.Purpose, p.DriverID, p.Phone, p.Amount, p.Currency, p.Gateway, key, p.ExpiresAt, p.ProviderRef, p.TripID).Scan(&id)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
		return "", ErrInProgress
	}
	return id, err
}

// setProviderRef records the gateway's reference once the push was accepted.
func (s *store) setProviderRef(ctx context.Context, id, ref string) error {
	_, err := s.pool.Exec(ctx, `UPDATE gateway_transactions SET provider_ref=$2 WHERE id=$1`, id, ref)
	return err
}

// touch counts a status check.
func (s *store) touch(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `UPDATE gateway_transactions SET checks=checks+1, last_checked_at=now() WHERE id=$1`, id)
	return err
}

// finish moves a row from `from` to a terminal status; false when it had already moved.
func (s *store) finish(ctx context.Context, q interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}, id, from, to, failure string) (bool, error) {
	tag, err := q.Exec(ctx, `UPDATE gateway_transactions SET status=$3, failure_code=NULLIF($4,''), completed_at=now()
		WHERE id=$1 AND status=$2`, id, from, to, failure)
	return tag.RowsAffected() > 0, err
}

// stalePending lists open rows nobody checked for a while (reconciliation).
func (s *store) stalePending(ctx context.Context, olderThan time.Duration, limit int) ([]string, error) {
	rows, err := s.pool.Query(ctx, `SELECT id FROM gateway_transactions
		WHERE status='PENDING' AND (last_checked_at IS NULL OR last_checked_at < now() - $1::interval)
		ORDER BY last_checked_at NULLS FIRST LIMIT $2`, olderThan, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

// recentlyUnsettled lists failed pay-ins whose money may still arrive (the prompt
// timed out, or the gateway's reply was lost), least recently checked first.
func (s *store) recentlyUnsettled(ctx context.Context, window, staleAfter time.Duration, limit int) ([]string, error) {
	rows, err := s.pool.Query(ctx, `SELECT id FROM gateway_transactions
		WHERE status='FAILED' AND failure_code = ANY($1) AND provider_ref IS NOT NULL AND completed_at > now() - $2::interval
		  AND (last_checked_at IS NULL OR last_checked_at < now() - $3::interval)
		ORDER BY last_checked_at NULLS FIRST LIMIT $4`, recoverableFailures, window, staleAfter, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

// recentPushes counts prompts sent by a driver in the last hour and to a phone in the last day.
func (s *store) recentPushes(ctx context.Context, driverID, phone string) (byDriver, byPhone int, err error) {
	err = s.pool.QueryRow(ctx, `SELECT
		  count(*) FILTER (WHERE payer_id=$2 AND created_at > now() - interval '1 hour'),
		  count(*) FILTER (WHERE phone=$3 AND created_at > now() - interval '1 day')
		FROM gateway_transactions WHERE brand_id=$1 AND (payer_id=$2 OR phone=$3)`, s.brandID, driverID, phone).Scan(&byDriver, &byPhone)
	return byDriver, byPhone, err
}

// list returns recent pay-ins for staff, with the driver's name, in one query.
func (s *store) list(ctx context.Context, status, driverID, currency, purpose string, limit int) ([]AdminPayin, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+prefixed(payinColumns, "g.")+`, u.full_name
		FROM gateway_transactions g JOIN users u ON u.id=g.payer_id
		WHERE g.brand_id=$1 AND ($2 = '' OR g.status=$2) AND ($3 = '' OR g.payer_id=$3::uuid)
		  AND ($5 = '' OR g.currency=$5) AND ($6 = '' OR g.purpose=$6)
		ORDER BY g.created_at DESC LIMIT $4`, s.brandID, status, driverID, limit, currency, purpose)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AdminPayin{}
	for rows.Next() {
		var a AdminPayin
		if err := rows.Scan(&a.ID, &a.DriverID, &a.Status, &a.FailureCode, &a.Amount, &a.Currency, &a.Phone, &a.Gateway, &a.ProviderRef, &a.CreatedAt, &a.CompletedAt, &a.ExpiresAt, &a.Purpose, &a.TripID, &a.Name); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func one(rows pgx.Rows) (*Payin, error) {
	p, err := pgx.CollectOneRow(rows, pgx.RowToStructByPos[Payin])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &p, err
}

// prefixed qualifies a comma-separated column list with a table alias.
func prefixed(cols, alias string) string {
	out := ""
	for i, c := range splitCols(cols) {
		if i > 0 {
			out += ", "
		}
		out += alias + c
	}
	return out
}

func splitCols(cols string) []string {
	var out []string
	start := 0
	for i := 0; i <= len(cols); i++ {
		if i == len(cols) || cols[i] == ',' {
			c := cols[start:i]
			for len(c) > 0 && c[0] == ' ' {
				c = c[1:]
			}
			out = append(out, c)
			start = i + 1
		}
	}
	return out
}

// fareRows returns a trip's fare prompts, newest first (prompt count and latest state).
func (s *store) fareRows(ctx context.Context, tripID string) ([]Payin, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+payinColumns+` FROM gateway_transactions
		WHERE brand_id=$1 AND trip_id=$2 AND purpose=$3 ORDER BY created_at DESC`, s.brandID, tripID, purposeRideFare)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[Payin])
}

// farePushesToPhone counts fare prompts sent to a number in the last hour.
func (s *store) farePushesToPhone(ctx context.Context, phone string) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM gateway_transactions
		WHERE brand_id=$1 AND phone=$2 AND purpose=$3 AND created_at > now() - interval '1 hour'`, s.brandID, phone, purposeRideFare).Scan(&n)
	return n, err
}
