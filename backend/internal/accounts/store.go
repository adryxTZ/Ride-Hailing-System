package accounts

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// store reads the ledger for one brand.
type store struct {
	pool    *pgxpool.Pool
	brandID string
}

// uuidRef turns a transaction reference into a uuid when it is one (references
// of test and legacy postings are free text; a bare cast would fail on them).
const uuidRef = `(CASE WHEN t.reference ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$' THEN t.reference::uuid END)`

// listSQL is the whole Accounts page in one statement: per ledger account its
// balance and the period's debits and credits, the fare share it took on the
// period's completed trips, then one line per kind (per payee for charges).
// The totals row is always returned, so an empty ledger still reports its fares.
const listSQL = `
WITH acc AS (
  SELECT a.id, a.kind::text AS kind, a.owner_id FROM ledger_accounts a WHERE a.brand_id = $1 AND a.currency = $2),
per_acct AS (
  SELECT acc.id, acc.kind, acc.owner_id, coalesce(sum(e.amount), 0) AS bal,
    coalesce(sum(e.amount) FILTER (WHERE e.amount > 0 AND e.created_at >= $3 AND e.created_at < $4), 0) AS deb,
    coalesce(-sum(e.amount) FILTER (WHERE e.amount < 0 AND e.created_at >= $3 AND e.created_at < $4), 0) AS cred,
    count(e.seq) AS n, max(e.created_at) AS last_at
  FROM acc LEFT JOIN ledger_entries e ON e.account_id = acc.id
  GROUP BY acc.id, acc.kind, acc.owner_id),
fare_tx AS (
  SELECT t.id, coalesce((tr.fees->>'total')::numeric, tr.final_fare, 0) AS fare
  FROM ledger_transactions t JOIN trips tr ON tr.id = ` + uuidRef + `
  WHERE t.brand_id = $1 AND t.kind = 'trip.completed' AND t.created_at >= $3 AND t.created_at < $4 AND tr.currency = $2),
shares AS (
  SELECT x.account_id, sum(x.fare) AS base, sum(x.amt) AS accrued FROM (
    SELECT e.account_id, f.fare, -sum(e.amount) AS amt
    FROM fare_tx f JOIN ledger_entries e ON e.transaction_id = f.id
    GROUP BY e.account_id, f.id, f.fare) x
  GROUP BY x.account_id),
totals AS (SELECT coalesce(sum(fare), 0) AS fares, count(*)::int AS trips FROM fare_tx),
lines AS (
  SELECT p.kind, CASE WHEN p.kind = 'CHARGES_PAYABLE' THEN p.owner_id END AS payee,
    sum(p.bal) AS bal,
    coalesce(sum(p.bal) FILTER (WHERE p.bal > 0), 0) AS pos,
    coalesce(-sum(p.bal) FILTER (WHERE p.bal < 0), 0) AS neg,
    sum(p.deb) AS deb, sum(p.cred) AS cred, sum(p.n)::int AS n, max(p.last_at) AS last_at,
    count(*)::int AS holders, sum(s.base) AS base, sum(s.accrued) AS accrued
  FROM per_acct p LEFT JOIN shares s ON s.account_id = p.id
  GROUP BY 1, 2)
SELECT l.kind, l.payee::text, py.name, py.kind, l.bal, l.pos, l.neg, l.deb, l.cred, l.n, l.last_at, l.holders,
  l.base, l.accrued, t.fares, t.trips
FROM totals t LEFT JOIN lines l ON true LEFT JOIN charge_payees py ON py.id = l.payee`

// rawLine is one row of listSQL, debit-positive as the ledger stores it.
type rawLine struct {
	kind, payee, name, payeeKind *string
	bal, pos, neg, deb, cred     *decimal.Decimal
	entries, holders             *int
	lastAt                       *time.Time
	base, accrued                *decimal.Decimal
}

// list reads every account line in one currency for a period, with the
// period's fare total and trip count.
func (s store) list(ctx context.Context, currency string, p Period) ([]rawLine, decimal.Decimal, int, error) {
	rows, err := s.pool.Query(ctx, listSQL, s.brandID, currency, p.From, p.To)
	if err != nil {
		return nil, decimal.Zero, 0, err
	}
	defer rows.Close()
	var out []rawLine
	fares, trips := decimal.Zero, 0
	for rows.Next() {
		var r rawLine
		if err := rows.Scan(&r.kind, &r.payee, &r.name, &r.payeeKind, &r.bal, &r.pos, &r.neg, &r.deb, &r.cred,
			&r.entries, &r.lastAt, &r.holders, &r.base, &r.accrued, &fares, &trips); err != nil {
			return nil, decimal.Zero, 0, err
		}
		if r.kind != nil {
			out = append(out, r)
		}
	}
	return out, fares, trips, rows.Err()
}

// entriesSQL is one page of an account's entries, newest first, with the
// running balance after each (computed over the whole account, then paged)
// and the trip each posting belongs to. One statement.
const entriesSQL = `
WITH acc AS (
  SELECT a.id, a.owner_id FROM ledger_accounts a
  WHERE a.brand_id = $1 AND a.kind = $2::account_kind AND a.currency = $3 AND ($4::uuid IS NULL OR a.owner_id = $4::uuid)),
run AS (
  SELECT e.seq, e.transaction_id, e.account_id, e.amount, e.created_at, sum(e.amount) OVER (ORDER BY e.seq) AS running
  FROM ledger_entries e WHERE e.account_id IN (SELECT id FROM acc)),
page AS (
  SELECT r.*, count(*) OVER () AS total FROM run r
  WHERE r.created_at >= $5 AND r.created_at < $6 ORDER BY r.seq DESC LIMIT $7 OFFSET $8)
SELECT p.seq, p.created_at, t.kind, t.reference, coalesce(t.memo, ''), p.amount, p.running, p.total::int,
  tr.id::text, tr.code, CASE WHEN $9 THEN acc.owner_id::text END, CASE WHEN $9 THEN coalesce(u.full_name, u.phone) END
FROM page p
JOIN ledger_transactions t ON t.id = p.transaction_id
JOIN acc ON acc.id = p.account_id
LEFT JOIN trip_refunds rf ON t.kind = 'trip.refund' AND rf.id = ` + uuidRef + `
LEFT JOIN trips tr ON tr.id = coalesce(rf.trip_id,
  CASE WHEN t.kind IN ('trip.completed', 'trip.cancellation_fee', 'trip.dues_collected') THEN ` + uuidRef + ` END)
LEFT JOIN users u ON $9::boolean AND u.id = acc.owner_id
ORDER BY p.seq DESC`

// entryQuery selects the entries of one account line.
type entryQuery struct {
	kind, payee, currency string
	period                Period
	limit, offset         int
	perHolder             bool
}

// entries reads one page of entries (amounts debit-positive) and the total in the period.
func (s store) entries(ctx context.Context, q entryQuery) ([]Entry, int, error) {
	var payee any
	if q.payee != "" {
		payee = q.payee
	}
	rows, err := s.pool.Query(ctx, entriesSQL, s.brandID, q.kind, q.currency, payee, q.period.From, q.period.To,
		q.limit, q.offset, q.perHolder)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out, total := []Entry{}, 0
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.Seq, &e.At, &e.Kind, &e.Reference, &e.Memo, &e.Amount, &e.Running, &total,
			&e.TripID, &e.TripCode, &e.HolderID, &e.Holder); err != nil {
			return nil, 0, err
		}
		out = append(out, e)
	}
	return out, total, rows.Err()
}
