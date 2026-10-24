package money

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
)

// PlatformBank is the platform's own bank or cash: payments staff record to the
// tax authority, the regulator, charge payees and riders leave from here.
const PlatformBank = "PLATFORM_BANK"

// txSettlement is the ledger kind for a payment staff recorded against a payable.
const txSettlement = "finance.settlement"

// ErrNothingToSettle means a settlement had no payable leg with an amount.
var ErrNothingToSettle = errors.New("money: a settlement needs a positive amount")

// Payable is one account a settlement pays down: kind, owner (payee or rider;
// empty for VAT and levy) and the amount paid.
type Payable struct {
	Kind    string
	OwnerID string
	Amount  decimal.Decimal
}

// LockTx takes the brand's ledger writer lock inside the caller's transaction,
// so a balance read after it cannot change before the caller posts. PostTx
// takes the same lock again (advisory locks stack within a session).
func (l *Ledger) LockTx(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('ledger:' || $1))`, l.brandID)
	return err
}

// PostSettlementTx records a payment staff made against payables: each payable
// is debited (the platform owes less) and PLATFORM_BANK is credited with the
// total (the money left the bank). Idempotent per settlement id.
func (l *Ledger) PostSettlementTx(ctx context.Context, tx pgx.Tx, settlementID, memo, currency string, paid []Payable) (bool, error) {
	total := decimal.Zero
	legs := make([]Leg, 0, len(paid)+1)
	for _, p := range paid {
		if !p.Amount.IsPositive() {
			return false, ErrNothingToSettle
		}
		total = total.Add(p.Amount)
		legs = append(legs, Leg(p))
	}
	if !total.IsPositive() {
		return false, ErrNothingToSettle
	}
	legs = append(legs, Leg{Kind: PlatformBank, Amount: total.Neg()})
	return l.PostTx(ctx, tx, txSettlement, settlementID, memo, currency, legs)
}
