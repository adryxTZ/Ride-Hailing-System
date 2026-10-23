package receipts

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type store struct {
	pool    *pgxpool.Pool
	brandID string
}

// nextSeq takes the next number of the year inside the caller's transaction.
// The counter row stays locked until commit, so numbers are gapless: a
// rollback gives the number back to the next receipt.
func (s store) nextSeq(ctx context.Context, tx pgx.Tx, year int) (int, error) {
	var seq int
	err := tx.QueryRow(ctx, `INSERT INTO receipt_counters (brand_id, year, last) VALUES ($1,$2,1)
		ON CONFLICT (brand_id, year) DO UPDATE SET last = receipt_counters.last + 1 RETURNING last`, s.brandID, year).Scan(&seq)
	return seq, err
}

// exists reports whether the trip already has a receipt (issuing twice is a no-op).
func (s store) exists(ctx context.Context, tx pgx.Tx, tripID string) (bool, error) {
	var ok bool
	err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM receipts WHERE trip_id=$1)`, tripID).Scan(&ok)
	return ok, err
}

// insert stores an issued receipt.
func (s store) insert(ctx context.Context, tx pgx.Tx, r *Receipt) error {
	lines, _ := json.Marshal(r.Lines)
	taxes, _ := json.Marshal(r.Taxes)
	payment, _ := json.Marshal(r.Payment)
	issuer, _ := json.Marshal(r.Issuer)
	trip, _ := json.Marshal(r.Trip)
	driver, _ := json.Marshal(r.Driver)
	return tx.QueryRow(ctx, `INSERT INTO receipts (brand_id, trip_id, number, issued_at, currency, total, lines, taxes, payment, issuer, trip_info, driver_info, rider_id, driver_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14) RETURNING id`,
		s.brandID, r.TripID, r.Number, r.IssuedAt, r.Currency, r.Total, lines, taxes, payment, issuer, trip, driver, r.riderID, r.driverID).Scan(&r.ID)
}

const receiptColumns = `id, number, issued_at, trip_id, currency, total, lines, taxes, payment, issuer, trip_info, driver_info, rider_id, driver_id`

// byTrip loads a trip's receipt, or nil.
func (s store) byTrip(ctx context.Context, tripID string) (*Receipt, error) {
	return s.one(ctx, `SELECT `+receiptColumns+` FROM receipts WHERE brand_id=$1 AND trip_id=$2`, tripID)
}

// byID loads a receipt by id, or nil.
func (s store) byID(ctx context.Context, id string) (*Receipt, error) {
	return s.one(ctx, `SELECT `+receiptColumns+` FROM receipts WHERE brand_id=$1 AND id=$2`, id)
}

func (s store) one(ctx context.Context, sql, arg string) (*Receipt, error) {
	var r Receipt
	var lines, taxes, payment, issuer, trip, driver []byte
	err := s.pool.QueryRow(ctx, sql, s.brandID, arg).Scan(&r.ID, &r.Number, &r.IssuedAt, &r.TripID, &r.Currency, &r.Total,
		&lines, &taxes, &payment, &issuer, &trip, &driver, &r.riderID, &r.driverID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	for raw, dst := range map[*[]byte]any{&lines: &r.Lines, &taxes: &r.Taxes, &payment: &r.Payment, &issuer: &r.Issuer, &trip: &r.Trip, &driver: &r.Driver} {
		if err := json.Unmarshal(*raw, dst); err != nil {
			return nil, err
		}
	}
	return &r, nil
}
