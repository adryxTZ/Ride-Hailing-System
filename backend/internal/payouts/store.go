package payouts

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

const columns = `id, driver_id, amount, currency, phone, gateway, status, failure_code, provider_ref, created_at, decided_at, decision_reason, sent_at, completed_at`

type store struct {
	pool    *pgxpool.Pool
	brandID string
}

type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func one(rows pgx.Rows, err error) (*Payout, error) {
	if err != nil {
		return nil, err
	}
	p, err := pgx.CollectOneRow(rows, pgx.RowToStructByPos[Payout])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &p, err
}

// get loads a payout, scoped to its driver when driverID is set; forUpdate locks it.
func (s store) get(ctx context.Context, q querier, id, driverID string, forUpdate bool) (*Payout, error) {
	lock := ""
	if forUpdate {
		lock = " FOR UPDATE"
	}
	return one(q.Query(ctx, `SELECT `+columns+` FROM driver_payouts WHERE brand_id=$1 AND id=$2 AND ($3='' OR driver_id=$3::uuid)`+lock, s.brandID, id, driverID))
}

func (s store) byKey(ctx context.Context, driverID, key string) (*Payout, error) {
	return one(s.pool.Query(ctx, `SELECT `+columns+` FROM driver_payouts WHERE brand_id=$1 AND driver_id=$2 AND idempotency_key=$3`, s.brandID, driverID, key))
}

func (s store) openFor(ctx context.Context, driverID string) (*Payout, error) {
	return one(s.pool.Query(ctx, `SELECT `+columns+` FROM driver_payouts WHERE brand_id=$1 AND driver_id=$2 AND status IN ('REQUESTED','PENDING')`, s.brandID, driverID))
}

func (s store) listForDriver(ctx context.Context, driverID string, limit int) ([]Payout, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+columns+` FROM driver_payouts WHERE brand_id=$1 AND driver_id=$2 ORDER BY created_at DESC LIMIT $3`, s.brandID, driverID, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[Payout])
}

// requestsToday counts a driver's cash-outs in the last 24 hours. Rejected and
// cancelled requests do not count: a refusal must not also cost the driver a try.
func (s store) requestsToday(ctx context.Context, driverID string) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM driver_payouts WHERE brand_id=$1 AND driver_id=$2
		AND status NOT IN ('REJECTED','CANCELLED') AND created_at > now() - interval '1 day'`, s.brandID, driverID).Scan(&n)
	return n, err
}

// insert stores a new request; a second open payout for the driver is ErrInProgress.
func (s store) insert(ctx context.Context, tx pgx.Tx, p Payout, key string) error {
	_, err := tx.Exec(ctx, `INSERT INTO driver_payouts (id, brand_id, driver_id, amount, currency, phone, gateway, status, provider_ref, idempotency_key)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, p.ID, s.brandID, p.DriverID, p.Amount, p.Currency, p.Phone, p.Gateway, p.Status, p.ProviderRef, key)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
		return ErrInProgress
	}
	return err
}

// move changes status from one to another (false when it had already moved).
func (s store) move(ctx context.Context, tx pgx.Tx, id, from, to string, set string, args ...any) (bool, error) {
	sql := `UPDATE driver_payouts SET status=$3` + set + ` WHERE id=$1 AND status=$2`
	tag, err := tx.Exec(ctx, sql, append([]any{id, from, to}, args...)...)
	return tag.RowsAffected() > 0, err
}

func (s store) touch(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `UPDATE driver_payouts SET checks=checks+1, last_checked_at=now() WHERE id=$1`, id)
	return err
}

// lockDriver serialises balance checks of one driver's payouts until the transaction ends.
func (s store) lockDriver(ctx context.Context, tx pgx.Tx, driverID string) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('payout:' || $1))`, driverID)
	return err
}

// toCheck lists pending payouts the sweep should look at: sent but not checked
// lately, or never sent (their send job was lost).
func (s store) toCheck(ctx context.Context, limit int) (sent, unsent []string, err error) {
	rows, err := s.pool.Query(ctx, `SELECT id, sent_at IS NOT NULL FROM driver_payouts
		WHERE status='PENDING' AND (last_checked_at IS NULL OR last_checked_at < now() - $1::interval)
		  AND (sent_at IS NOT NULL OR decided_at < now() - $1::interval OR created_at < now() - $1::interval)
		ORDER BY last_checked_at NULLS FIRST LIMIT $2`, staleAfter, limit)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var wasSent bool
		if err := rows.Scan(&id, &wasSent); err != nil {
			return nil, nil, err
		}
		if wasSent {
			sent = append(sent, id)
		} else {
			unsent = append(unsent, id)
		}
	}
	return sent, unsent, rows.Err()
}

// adminList returns payouts with the driver's name and current balance, in one query.
func (s store) adminList(ctx context.Context, status, currency string, limit int) ([]AdminPayout, error) {
	// The page first, then one grouped balance read for its drivers (not one per row).
	rows, err := s.pool.Query(ctx, `WITH page AS (
		  SELECT * FROM driver_payouts p
		  WHERE p.brand_id=$1 AND ($2='' OR p.status=$2) AND ($3='' OR p.currency=$3)
		  ORDER BY (p.status IN ('REQUESTED','PENDING')) DESC, p.created_at DESC LIMIT $4),
		balances AS (
		  SELECT a.owner_id, -sum(e.amount) AS balance FROM ledger_accounts a JOIN ledger_entries e ON e.account_id=a.id
		  WHERE a.brand_id=$1 AND a.kind='DRIVER_WALLET' AND a.owner_id IN (SELECT driver_id FROM page) GROUP BY a.owner_id)
		SELECT p.id, p.driver_id, p.amount, p.currency, p.phone, p.gateway, p.status, p.failure_code, p.provider_ref,
		  p.created_at, p.decided_at, p.decision_reason, p.sent_at, p.completed_at, u.full_name, coalesce(b.balance, 0),
		  p.status='PENDING' AND p.sent_at IS NOT NULL AND p.sent_at < now() - $5::interval
		FROM page p JOIN users u ON u.id=p.driver_id LEFT JOIN balances b ON b.owner_id=p.driver_id
		ORDER BY (p.status IN ('REQUESTED','PENDING')) DESC, p.created_at DESC`, s.brandID, status, currency, limit, StuckAfter)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AdminPayout{}
	for rows.Next() {
		var a AdminPayout
		p := &a.Payout
		if err := rows.Scan(&p.ID, &p.DriverID, &p.Amount, &p.Currency, &p.Phone, &p.Gateway, &p.Status, &p.FailureCode, &p.ProviderRef,
			&p.CreatedAt, &p.DecidedAt, &p.DecisionReason, &p.SentAt, &p.CompletedAt, &a.Name, &a.Balance, &a.Stuck); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// totals sums open payouts and those paid since midnight, in one currency.
func (s store) totals(ctx context.Context, currency string, since time.Time) (Totals, error) {
	var t Totals
	err := s.pool.QueryRow(ctx, `SELECT coalesce(sum(amount) FILTER (WHERE status IN ('REQUESTED','PENDING')),0),
		  coalesce(sum(amount) FILTER (WHERE status='PAID' AND completed_at >= $3),0)
		FROM driver_payouts WHERE brand_id=$1 AND ($2='' OR currency=$2)`, s.brandID, currency, since).Scan(&t.InProgress, &t.PaidToday)
	return t, err
}

// recentlyRefused lists sent payouts the gateway refused in the last window,
// least recently checked first: a refusal is re-checked in case it paid after all.
func (s store) recentlyRefused(ctx context.Context, window time.Duration, limit int) ([]string, error) {
	rows, err := s.pool.Query(ctx, `SELECT id FROM driver_payouts
		WHERE status='FAILED' AND sent_at IS NOT NULL AND provider_ref IS NOT NULL AND completed_at > now() - $1::interval
		  AND (last_checked_at IS NULL OR last_checked_at < now() - $2::interval)
		ORDER BY last_checked_at NULLS FIRST LIMIT $3`, window, staleAfter, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}
