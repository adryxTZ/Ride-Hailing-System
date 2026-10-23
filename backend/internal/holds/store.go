package holds

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// pgUniqueViolation is the Postgres error code for a duplicate key.
const pgUniqueViolation = "23505"

// querier is a pool or a transaction.
type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type store struct {
	pool    *pgxpool.Pool
	brandID string
}

// active lists a driver's active holds, oldest first.
func (s *store) active(ctx context.Context, q querier, driverID string) ([]Hold, error) {
	rows, err := q.Query(ctx, `SELECT id, kind, cause, reason, placed_at FROM driver_holds
		WHERE brand_id=$1 AND driver_id=$2 AND released_at IS NULL ORDER BY placed_at`, s.brandID, driverID)
	if err != nil {
		return nil, err
	}
	out, err := pgx.CollectRows(rows, pgx.RowToStructByPos[Hold])
	if out == nil {
		out = []Hold{}
	}
	return out, err
}

// clock returns when the driver's debt reached the soft limit, or nil.
func (s *store) clock(ctx context.Context, q querier, driverID string) (*time.Time, error) {
	var t time.Time
	err := q.QueryRow(ctx, `SELECT over_soft_since FROM driver_debt_clock WHERE brand_id=$1 AND driver_id=$2`, s.brandID, driverID).Scan(&t)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &t, err
}

// saveClock keeps or clears the soft clock to match the decision.
func (s *store) saveClock(ctx context.Context, tx pgx.Tx, driverID string, start *time.Time) error {
	if start == nil {
		_, err := tx.Exec(ctx, `DELETE FROM driver_debt_clock WHERE brand_id=$1 AND driver_id=$2`, s.brandID, driverID)
		return err
	}
	_, err := tx.Exec(ctx, `INSERT INTO driver_debt_clock (brand_id, driver_id, over_soft_since) VALUES ($1,$2,$3)
		ON CONFLICT (brand_id, driver_id) DO NOTHING`, s.brandID, driverID, *start)
	return err
}

// debtHold returns the active DEBT hold's id and cause ("" when none).
func (s *store) debtHold(ctx context.Context, tx pgx.Tx, driverID string) (string, string, error) {
	var id, cause string
	err := tx.QueryRow(ctx, `SELECT id, cause FROM driver_holds WHERE brand_id=$1 AND driver_id=$2 AND kind='DEBT' AND released_at IS NULL`,
		s.brandID, driverID).Scan(&id, &cause)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", nil
	}
	return id, cause, err
}

// place inserts an active hold; a second active hold of the same kind is ErrHoldExists.
func (s *store) place(ctx context.Context, tx pgx.Tx, driverID, kind, cause, reason, staffID string) error {
	_, err := tx.Exec(ctx, `INSERT INTO driver_holds (brand_id, driver_id, kind, cause, reason, placed_by)
		VALUES ($1,$2,$3,$4,NULLIF($5,''),NULLIF($6,'')::uuid)`, s.brandID, driverID, kind, cause, reason, staffID)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
		return ErrHoldExists
	}
	return err
}

// release ends the active hold of one kind; false when there was none.
func (s *store) release(ctx context.Context, tx pgx.Tx, driverID, kind, reason, staffID string) (bool, error) {
	tag, err := tx.Exec(ctx, `UPDATE driver_holds SET released_at=now(), released_by=NULLIF($4,'')::uuid, release_reason=NULLIF($5,'')
		WHERE brand_id=$1 AND driver_id=$2 AND kind=$3 AND released_at IS NULL`, s.brandID, driverID, kind, staffID, reason)
	return tag.RowsAffected() > 0, err
}

// setCause updates why an active DEBT hold holds.
func (s *store) setCause(ctx context.Context, tx pgx.Tx, holdID, cause string) error {
	_, err := tx.Exec(ctx, `UPDATE driver_holds SET cause=$2 WHERE id=$1`, holdID, cause)
	return err
}

// driverExists reports whether the user is a registered driver of this brand.
func (s *store) driverExists(ctx context.Context, driverID string) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM driver_profiles WHERE user_id=$1 AND brand_id=$2)`, driverID, s.brandID).Scan(&ok)
	return ok, err
}

// listActive returns every active hold with the driver's name and debt in one query.
func (s *store) listActive(ctx context.Context, country string, limit int) ([]ActiveHold, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT h.id, h.kind, h.cause, h.reason, h.placed_at, h.driver_id, u.full_name, u.phone,
		  greatest(coalesce((SELECT sum(e.amount) FROM ledger_entries e JOIN ledger_accounts a ON a.id=e.account_id
		    WHERE a.brand_id=$1 AND a.kind='DRIVER_WALLET' AND a.owner_id=h.driver_id), 0), 0)
		FROM driver_holds h JOIN users u ON u.id=h.driver_id JOIN driver_profiles p ON p.user_id=h.driver_id
		WHERE h.brand_id=$1 AND h.released_at IS NULL AND ($3='' OR p.country_code=$3)
		ORDER BY h.placed_at DESC LIMIT $2`, s.brandID, limit, country)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ActiveHold{}
	for rows.Next() {
		var a ActiveHold
		if err := rows.Scan(&a.Hold.ID, &a.Hold.Kind, &a.Hold.Cause, &a.Hold.Reason, &a.Hold.Since, &a.DriverID, &a.Name, &a.Phone, &a.Debt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// sweepCandidates lists drivers the rule may need to look at: an active DEBT hold,
// a running soft clock, or debt at or above the soft limit. Keyset-paged by id.
func (s *store) sweepCandidates(ctx context.Context, soft decimal.Decimal, after string, limit int) ([]string, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id FROM (
		  SELECT driver_id::text id FROM driver_holds WHERE brand_id=$1 AND kind='DEBT' AND released_at IS NULL
		  UNION SELECT driver_id::text FROM driver_debt_clock WHERE brand_id=$1
		  UNION SELECT a.owner_id::text FROM ledger_entries e JOIN ledger_accounts a ON a.id=e.account_id
		    WHERE a.brand_id=$1 AND a.kind='DRIVER_WALLET' GROUP BY a.owner_id HAVING sum(e.amount) >= $2
		) c WHERE id > $3 ORDER BY id LIMIT $4`, s.brandID, soft, after, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}
