package money

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
)

// txPayoutPaid is the ledger kind for money sent to a driver's wallet.
const txPayoutPaid = "payout.paid"

// DriverOwed is what the platform owes a driver (their positive balance, never
// negative) and the wallet currency. Cash debt and earnings net out first: a
// driver who owes more than they earned has nothing to cash out.
func (l *Ledger) DriverOwed(ctx context.Context, driverID string) (decimal.Decimal, string, error) {
	return l.driverOwed(ctx, l.pool, driverID)
}

// DriverOwedIn is DriverOwed inside the caller's transaction (under the payout lock).
func (l *Ledger) DriverOwedIn(ctx context.Context, tx pgx.Tx, driverID string) (decimal.Decimal, string, error) {
	return l.driverOwed(ctx, tx, driverID)
}

func (l *Ledger) driverOwed(ctx context.Context, q rowQuerier, driverID string) (decimal.Decimal, string, error) {
	var sum decimal.Decimal
	var cur string
	err := q.QueryRow(ctx, `SELECT coalesce(-sum(e.amount),0), coalesce(max(a.currency),'')
		FROM ledger_entries e JOIN ledger_accounts a ON a.id=e.account_id
		WHERE a.brand_id=$1 AND a.kind='DRIVER_WALLET' AND a.owner_id=$2`, l.brandID, driverID).Scan(&sum, &cur)
	if err != nil {
		return decimal.Zero, "", err
	}
	return decimal.Max(sum, decimal.Zero), cur, nil
}

// PostPayoutPaidTx records a payout the gateway confirmed as paid, in the
// transaction that marks it PAID: the platform owes the driver less (wallet
// debited) and the money left the gateway account (clearing credited).
// Posting the same payout twice is a no-op.
func (l *Ledger) PostPayoutPaidTx(ctx context.Context, tx pgx.Tx, payoutID, driverID string, amount decimal.Decimal, currency, ref string) (bool, error) {
	legs := []Leg{
		{Kind: DriverWallet, OwnerID: driverID, Amount: amount},
		{Kind: GatewayClearing, Amount: amount.Neg()},
	}
	return l.PostTx(ctx, tx, txPayoutPaid, payoutID, fmt.Sprintf("Cash out %s to your phone (%s)", amount.StringFixed(0), ref), currency, legs)
}
