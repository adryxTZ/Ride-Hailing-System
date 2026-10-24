package accounts

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/money"
)

// owedRidersLimit bounds the riders listed in the rider-refunds payment dialog.
const owedRidersLimit = 200

// paymentCols reads a finance_settlements row as a Payment (refund ids apart).
const paymentCols = `s.id::text, s.account_kind, s.payee_id::text, s.rider_id::text, s.currency, s.amount,
	to_char(s.paid_on, 'YYYY-MM-DD'), s.method, s.reference, s.reason, s.created_at`

// byIdemKey returns the payment recorded with this key, or nil.
func (s store) byIdemKey(ctx context.Context, tx pgx.Tx, key string) (*Payment, error) {
	var p Payment
	err := tx.QueryRow(ctx, `SELECT `+paymentCols+` FROM finance_settlements s WHERE s.brand_id = $1 AND s.idempotency_key = $2`,
		s.brandID, key).Scan(&p.ID, &p.Kind, &p.PayeeID, &p.RiderID, &p.Currency, &p.Amount, &p.PaidOn, &p.Method,
		&p.Reference, &p.Reason, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	p.Key = KeyOf(p.Kind, deref(p.PayeeID))
	p.RefundIDs, err = s.paidRefunds(ctx, tx, p.ID)
	return &p, err
}

// paidRefunds lists the refunds a payment covered.
func (s store) paidRefunds(ctx context.Context, tx pgx.Tx, paymentID string) ([]string, error) {
	rows, err := tx.Query(ctx, `SELECT id::text FROM trip_refunds WHERE brand_id = $1 AND paid_ref = $2 ORDER BY created_at`, s.brandID, paymentID)
	if err != nil {
		return nil, err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if ids == nil {
		ids = []string{}
	}
	return ids, err
}

// owed is what the platform owes on one account (its credit balance; the
// payee or rider for per-owner accounts, empty for VAT and levy).
func (s store) owed(ctx context.Context, tx pgx.Tx, kind, owner, currency string) (decimal.Decimal, error) {
	var ownerArg any
	if owner != "" {
		ownerArg = owner
	}
	var sum decimal.Decimal
	err := tx.QueryRow(ctx, `SELECT coalesce(-sum(e.amount), 0) FROM ledger_entries e JOIN ledger_accounts a ON a.id = e.account_id
		WHERE a.brand_id = $1 AND a.kind = $2::account_kind AND a.currency = $3 AND a.owner_id IS NOT DISTINCT FROM $4::uuid`,
		s.brandID, kind, currency, ownerArg).Scan(&sum)
	return sum, err
}

// ridersOwed is what each of these riders is owed (RIDER_REFUNDS) in one query.
func (s store) ridersOwed(ctx context.Context, tx pgx.Tx, currency string, riders []string) (map[string]decimal.Decimal, error) {
	rows, err := tx.Query(ctx, `SELECT a.owner_id::text, coalesce(-sum(e.amount), 0) FROM ledger_accounts a
		LEFT JOIN ledger_entries e ON e.account_id = a.id
		WHERE a.brand_id = $1 AND a.kind = 'RIDER_REFUNDS' AND a.currency = $2 AND a.owner_id = ANY($3::uuid[])
		GROUP BY a.owner_id`, s.brandID, currency, riders)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]decimal.Decimal{}
	for rows.Next() {
		var id string
		var owed decimal.Decimal
		if err := rows.Scan(&id, &owed); err != nil {
			return nil, err
		}
		out[id] = owed
	}
	return out, rows.Err()
}

// refundFilter picks owed refunds: these ids, one rider's, or all (oldest first).
type refundFilter struct {
	currency string
	ids      []string
	riderID  string
}

// owedRefundsSQL lists unpaid refunds oldest first; $3 and $4 narrow to ids or a rider.
const owedRefundsSQL = `SELECT r.id::text, r.trip_id::text, t.code, r.rider_id::text, coalesce(u.full_name, u.phone), r.amount, r.reason, r.created_at
	FROM trip_refunds r JOIN trips t ON t.id = r.trip_id JOIN users u ON u.id = r.rider_id
	WHERE r.brand_id = $1 AND r.currency = $2 AND r.status = 'OWED'
	  AND ($3::uuid[] IS NULL OR r.id = ANY($3::uuid[])) AND ($4::uuid IS NULL OR r.rider_id = $4::uuid)
	ORDER BY r.created_at, r.id`

// owedRefunds reads unpaid refunds; with lock, inside the payment's transaction.
func (s store) owedRefunds(ctx context.Context, q pgx.Tx, f refundFilter, lock bool) ([]OwedRefund, error) {
	var ids, rider any
	if f.ids != nil {
		ids = f.ids
	}
	if f.riderID != "" {
		rider = f.riderID
	}
	sql := owedRefundsSQL
	if lock {
		sql += ` FOR UPDATE OF r`
	}
	var rows pgx.Rows
	var err error
	if q != nil {
		rows, err = q.Query(ctx, sql, s.brandID, f.currency, ids, rider)
	} else {
		rows, err = s.pool.Query(ctx, sql, s.brandID, f.currency, ids, rider)
	}
	if err != nil {
		return nil, err
	}
	out, err := pgx.CollectRows(rows, pgx.RowToStructByPos[OwedRefund])
	if out == nil {
		out = []OwedRefund{}
	}
	return out, err
}

// owedRiders lists riders with a positive refund balance, most owed first.
func (s store) owedRiders(ctx context.Context, currency string) ([]OwedRider, error) {
	rows, err := s.pool.Query(ctx, `SELECT a.owner_id::text, coalesce(u.full_name, u.phone), -sum(e.amount) AS owed
		FROM ledger_accounts a JOIN ledger_entries e ON e.account_id = a.id JOIN users u ON u.id = a.owner_id
		WHERE a.brand_id = $1 AND a.kind = 'RIDER_REFUNDS' AND a.currency = $2
		GROUP BY a.owner_id, u.full_name, u.phone HAVING -sum(e.amount) > 0 ORDER BY owed DESC LIMIT $3`,
		s.brandID, currency, owedRidersLimit)
	if err != nil {
		return nil, err
	}
	out, err := pgx.CollectRows(rows, pgx.RowToStructByPos[OwedRider])
	if out == nil {
		out = []OwedRider{}
	}
	return out, err
}

// insert records a payment and returns its id and time.
func (s store) insert(ctx context.Context, tx pgx.Tx, in PaymentInput, kind, payee string) (string, error) {
	var id string
	var payeeArg, riderArg any
	if payee != "" {
		payeeArg = payee
	}
	if in.RiderID != "" {
		riderArg = in.RiderID
	}
	err := tx.QueryRow(ctx, `INSERT INTO finance_settlements
		(brand_id, account_kind, payee_id, rider_id, currency, amount, paid_on, method, reference, reason, idempotency_key, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7::date,$8,$9,$10,$11,$12) RETURNING id::text`,
		s.brandID, kind, payeeArg, riderArg, in.Currency, in.Amount, in.PaidOn, in.Method, in.Reference, in.Reason, in.IdemKey, in.StaffID).Scan(&id)
	return id, err
}

// markPaid marks refunds PAID by a payment.
func (s store) markPaid(ctx context.Context, tx pgx.Tx, ids []string, paymentID string) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := tx.Exec(ctx, `UPDATE trip_refunds SET status = 'PAID', paid_at = now(), paid_ref = $3
		WHERE brand_id = $1 AND id = ANY($2::uuid[]) AND status = 'OWED'`, s.brandID, ids, paymentID)
	return err
}

// payeeExists reports whether a charge payee belongs to the brand and the country.
func (s store) payeeExists(ctx context.Context, tx pgx.Tx, payeeID, country string) (bool, error) {
	var ok bool
	err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM charge_payees WHERE brand_id = $1 AND id = $2::uuid AND country_code = $3)`,
		s.brandID, payeeID, country).Scan(&ok)
	return ok, err
}

// isRefunds reports whether a kind is the rider-refunds account.
func isRefunds(kind string) bool { return kind == money.RiderRefunds }

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
