package money

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
)

// FinanceSummary is the platform's position from the ledger.
type FinanceSummary struct {
	Currency        string          `json:"currency"`
	PlatformRevenue decimal.Decimal `json:"platform_revenue"` // commission + fees net of VAT and levy
	VATPayable      decimal.Decimal `json:"vat_payable"`
	LevyPayable     decimal.Decimal `json:"levy_payable"`
	DriversOwe      decimal.Decimal `json:"drivers_owe"`      // cash trips: platform share still with drivers
	OwedToDrivers   decimal.Decimal `json:"owed_to_drivers"`  // e.g. cancellation fee shares not yet paid out
	RiderDues       decimal.Decimal `json:"rider_dues"`       // cancellation fees riders still owe
	GatewayClearing decimal.Decimal `json:"gateway_clearing"` // collected at the payment gateway, not yet in the bank
	OwedToRiders    decimal.Decimal `json:"owed_to_riders"`   // fare money that arrived after the trip was settled
	LedgerEntries   int             `json:"ledger_entries"`
	// ChargesPayable is extra charges owed to third parties, per payee.
	ChargesPayable []PayeeOwed `json:"charges_payable"`
}

// PayeeOwed is what the platform holds for one third party (an airport, a council …).
type PayeeOwed struct {
	PayeeID string          `json:"payee_id"`
	Name    string          `json:"name"`
	Kind    string          `json:"kind"`
	Amount  decimal.Decimal `json:"amount"`
}

// DriverBalance is one driver's wallet line.
type DriverBalance struct {
	DriverID string          `json:"driver_id"`
	Name     *string         `json:"name,omitempty"`
	Phone    string          `json:"phone"`
	Balance  decimal.Decimal `json:"balance"` // − = owes the platform
	LastAt   time.Time       `json:"last_at"`
	OnHold   bool            `json:"on_hold"`             // any active debt or manual hold
	HoldKind *string         `json:"hold_kind,omitempty"` // DEBT or MANUAL (MANUAL wins when both)
}

// JournalEntry is one ledger transaction with its legs, for the finance journal.
type JournalEntry struct {
	ID        string          `json:"id"`
	Kind      string          `json:"kind"`
	Reference string          `json:"reference"`
	Memo      string          `json:"memo"`
	At        time.Time       `json:"at"`
	Legs      []JournalLeg    `json:"legs"`
	Amount    decimal.Decimal `json:"amount"` // total debits
}

// JournalLeg is one account movement in a journal entry.
type JournalLeg struct {
	Account string          `json:"account"`
	OwnerID *string         `json:"owner_id,omitempty"`
	Amount  decimal.Decimal `json:"amount"` // + debit / − credit
}

// Summary sums the platform accounts and the driver and rider balances in one
// currency (each country keeps its own money; amounts are never converted).
// An empty currency sums every account, for single-currency brands and tests.
func (l *Ledger) Summary(ctx context.Context, currency string) (*FinanceSummary, error) {
	s := &FinanceSummary{Currency: currency}
	rows, err := l.pool.Query(ctx, `
		SELECT a.kind::text, a.currency, sum(e.amount), count(*)
		FROM ledger_entries e JOIN ledger_accounts a ON a.id=e.account_id
		WHERE a.brand_id=$1 AND ($2='' OR a.currency=$2) GROUP BY a.kind, a.currency`, l.brandID, currency)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var kind, cur string
		var sum decimal.Decimal
		var n int
		if err := rows.Scan(&kind, &cur, &sum, &n); err != nil {
			return nil, err
		}
		s.Currency, s.LedgerEntries = cur, s.LedgerEntries+n
		switch kind {
		case PlatformRevenue:
			s.PlatformRevenue = sum.Neg() // credits are revenue
		case VATPayable:
			s.VATPayable = sum.Neg()
		case LevyPayable:
			s.LevyPayable = sum.Neg()
		case RiderDues:
			s.RiderDues = sum
		case GatewayClearing:
			s.GatewayClearing = sum
		case RiderRefunds:
			s.OwedToRiders = sum.Neg()
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	err = l.pool.QueryRow(ctx, `
		SELECT coalesce(sum(b) FILTER (WHERE b > 0), 0), coalesce(-sum(b) FILTER (WHERE b < 0), 0) FROM (
		  SELECT sum(e.amount) b FROM ledger_entries e JOIN ledger_accounts a ON a.id=e.account_id
		  WHERE a.brand_id=$1 AND a.kind='DRIVER_WALLET' AND ($2='' OR a.currency=$2) GROUP BY a.owner_id) w`, l.brandID, currency).Scan(&s.DriversOwe, &s.OwedToDrivers)
	if err != nil {
		return nil, err
	}
	s.ChargesPayable, err = l.chargesPayable(ctx, currency)
	return s, err
}

// chargesPayable groups CHARGES_PAYABLE balances by payee (one query).
func (l *Ledger) chargesPayable(ctx context.Context, currency string) ([]PayeeOwed, error) {
	rows, err := l.pool.Query(ctx, `
		SELECT a.owner_id::text, coalesce(p.name,''), coalesce(p.kind,''), -sum(e.amount)
		FROM ledger_entries e JOIN ledger_accounts a ON a.id=e.account_id LEFT JOIN charge_payees p ON p.id=a.owner_id
		WHERE a.brand_id=$1 AND a.kind='CHARGES_PAYABLE' AND ($2='' OR a.currency=$2)
		GROUP BY a.owner_id, p.name, p.kind HAVING sum(e.amount) <> 0 ORDER BY 4 DESC`, l.brandID, currency)
	if err != nil {
		return nil, err
	}
	out, err := pgx.CollectRows(rows, pgx.RowToStructByPos[PayeeOwed])
	if out == nil {
		out = []PayeeOwed{}
	}
	return out, err
}

// DriverBalances lists drivers' wallet balances in one currency ("" = all), those owing most first.
func (l *Ledger) DriverBalances(ctx context.Context, currency string, limit int) ([]DriverBalance, error) {
	rows, err := l.pool.Query(ctx, `
		SELECT a.owner_id, u.full_name, u.phone, -sum(e.amount) AS balance, max(e.created_at),
		  EXISTS (SELECT 1 FROM driver_holds h WHERE h.brand_id=$1 AND h.driver_id=a.owner_id AND h.released_at IS NULL),
		  (SELECT h.kind FROM driver_holds h WHERE h.brand_id=$1 AND h.driver_id=a.owner_id AND h.released_at IS NULL ORDER BY h.kind DESC LIMIT 1)
		FROM ledger_entries e JOIN ledger_accounts a ON a.id=e.account_id JOIN users u ON u.id=a.owner_id
		WHERE a.brand_id=$1 AND a.kind='DRIVER_WALLET' AND ($3='' OR a.currency=$3)
		GROUP BY a.owner_id, u.full_name, u.phone ORDER BY balance ASC LIMIT $2`, l.brandID, limit, currency)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[DriverBalance])
}

// Journal lists recent ledger transactions in one currency ("" = all) with their legs (two queries).
func (l *Ledger) Journal(ctx context.Context, currency string, limit int) ([]JournalEntry, error) {
	rows, err := l.pool.Query(ctx, `
		SELECT t.id, t.kind, t.reference, coalesce(t.memo,''), t.created_at FROM ledger_transactions t
		WHERE t.brand_id=$1 AND ($3='' OR EXISTS (SELECT 1 FROM ledger_entries e JOIN ledger_accounts a ON a.id=e.account_id
		  WHERE e.transaction_id=t.id AND a.currency=$3))
		ORDER BY t.created_at DESC LIMIT $2`, l.brandID, limit, currency)
	if err != nil {
		return nil, err
	}
	out := []JournalEntry{}
	index := map[string]int{}
	var ids []string
	for rows.Next() {
		var j JournalEntry
		if err := rows.Scan(&j.ID, &j.Kind, &j.Reference, &j.Memo, &j.At); err != nil {
			rows.Close()
			return nil, err
		}
		j.Legs = []JournalLeg{}
		index[j.ID] = len(out)
		ids = append(ids, j.ID)
		out = append(out, j)
	}
	rows.Close()
	if len(ids) == 0 {
		return out, nil
	}
	legs, err := l.pool.Query(ctx, `
		SELECT e.transaction_id, a.kind::text, a.owner_id, e.amount FROM ledger_entries e JOIN ledger_accounts a ON a.id=e.account_id
		WHERE e.transaction_id = ANY($1::uuid[]) ORDER BY e.seq`, ids)
	if err != nil {
		return nil, err
	}
	defer legs.Close()
	for legs.Next() {
		var txID string
		var g JournalLeg
		if err := legs.Scan(&txID, &g.Account, &g.OwnerID, &g.Amount); err != nil {
			return nil, err
		}
		j := &out[index[txID]]
		j.Legs = append(j.Legs, g)
		if g.Amount.IsPositive() {
			j.Amount = j.Amount.Add(g.Amount)
		}
	}
	return out, legs.Err()
}
