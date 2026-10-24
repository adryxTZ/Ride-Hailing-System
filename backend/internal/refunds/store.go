package refunds

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

type store struct {
	pool    *pgxpool.Pool
	brandID string
}

// paidTrip is what a refund needs from the trip (read and locked in the refund's transaction).
type paidTrip struct {
	riderID, driverID, currency, status string
	paymentStatus                       *string
	paid, refunded                      decimal.Decimal
}

// paidSQL is this trip's own fare: the stored split's total, which leaves out
// earlier cancellation dues collected with it (those went to other trips).
const paidSQL = `SELECT t.rider_id::text, t.driver_id::text, t.currency, t.status::text, t.payment_status,
		coalesce((t.fees->>'total')::numeric, t.final_fare, t.estimated_fare), coalesce((SELECT sum(amount) FROM trip_refunds r WHERE r.trip_id=t.id), 0)
		FROM trips t WHERE t.id::text=$1 AND t.brand_id=$2`

// readTrip reads the refund facts (lock: inside the refund's transaction, before inserting).
func (s store) readTrip(ctx context.Context, q pgx.Tx, tripID string, lock bool) (*paidTrip, error) {
	var t paidTrip
	var driver *string
	sql := paidSQL
	if lock {
		sql += ` FOR UPDATE OF t`
	}
	var row pgx.Row
	if q != nil {
		row = q.QueryRow(ctx, sql, tripID, s.brandID)
	} else {
		row = s.pool.QueryRow(ctx, sql, tripID, s.brandID)
	}
	err := row.
		Scan(&t.riderID, &driver, &t.currency, &t.status, &t.paymentStatus, &t.paid, &t.refunded)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrTripNotFound
	}
	if driver != nil {
		t.driverID = *driver
	}
	return &t, err
}

func (s store) insert(ctx context.Context, tx pgx.Tx, tripID string, t *paidTrip, amount decimal.Decimal, bearer, reason, staffID, key string) (string, error) {
	var id string
	err := tx.QueryRow(ctx, `INSERT INTO trip_refunds (brand_id, trip_id, rider_id, amount, currency, borne_by, reason, created_by, idempotency_key)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,NULLIF($9,'')) RETURNING id`, s.brandID, tripID, t.riderID, amount, t.currency, bearer, reason, staffID, key).Scan(&id)
	return id, err
}

// seen reports whether a refund with this idempotency key was already made on the trip.
func (s store) seen(ctx context.Context, tx pgx.Tx, tripID, key string) (bool, error) {
	if key == "" {
		return false, nil
	}
	var n int
	err := tx.QueryRow(ctx, `SELECT count(*) FROM trip_refunds WHERE trip_id::text=$1 AND idempotency_key=$2`, tripID, key).Scan(&n)
	return n > 0, err
}

func (s store) forTrip(ctx context.Context, tripID string) ([]Refund, error) {
	rows, err := s.pool.Query(ctx, `SELECT r.id, r.trip_id, r.amount, r.currency, r.borne_by, r.reason, u.full_name, r.created_at, r.status, r.paid_at
		FROM trip_refunds r LEFT JOIN users u ON u.id=r.created_by WHERE r.trip_id::text=$1 AND r.brand_id=$2 ORDER BY r.created_at`, tripID, s.brandID)
	if err != nil {
		return nil, err
	}
	out, err := pgx.CollectRows(rows, pgx.RowToStructByPos[Refund])
	if out == nil {
		out = []Refund{}
	}
	return out, err
}
