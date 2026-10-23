package money

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
)

// rowQuerier is a pool or a transaction.
type rowQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// txPayinCollected is the ledger kind for a driver's mobile-money debt payment.
const txPayinCollected = "payin.collected"

// PostPayinCollected records money a driver paid through the gateway against
// their debt: the gateway clearing account is debited (money is there) and the
// driver's wallet credited (they owe less). It runs inside the caller's database
// transaction so the pay-in's status change and the posting commit together;
// posting the same pay-in twice is a no-op.
func (l *Ledger) PostPayinCollected(ctx context.Context, tx pgx.Tx, payinID, driverID string, amount decimal.Decimal, currency, gateway, providerRef string) (bool, error) {
	legs := []Leg{
		{Kind: GatewayClearing, Amount: amount},
		{Kind: DriverWallet, OwnerID: driverID, Amount: amount.Neg()},
	}
	memo := fmt.Sprintf("Mobile money payment %s (%s %s)", amount.StringFixed(0), gateway, providerRef)
	return l.PostTx(ctx, tx, txPayinCollected, payinID, memo, currency, legs)
}

// DriverDebt is what a driver owes the platform (never negative) and the wallet currency.
func (l *Ledger) DriverDebt(ctx context.Context, driverID string) (decimal.Decimal, string, error) {
	return l.driverDebt(ctx, l.pool, driverID)
}

// DriverDebtIn is DriverDebt inside the caller's transaction, so a caller
// holding a lock does not need a second pool connection.
func (l *Ledger) DriverDebtIn(ctx context.Context, tx pgx.Tx, driverID string) (decimal.Decimal, string, error) {
	return l.driverDebt(ctx, tx, driverID)
}

func (l *Ledger) driverDebt(ctx context.Context, q rowQuerier, driverID string) (decimal.Decimal, string, error) {
	var sum decimal.Decimal
	var cur string
	err := q.QueryRow(ctx, `SELECT coalesce(sum(e.amount),0), coalesce(max(a.currency),'')
		FROM ledger_entries e JOIN ledger_accounts a ON a.id=e.account_id
		WHERE a.brand_id=$1 AND a.kind='DRIVER_WALLET' AND a.owner_id=$2`, l.brandID, driverID).Scan(&sum, &cur)
	if err != nil {
		return decimal.Zero, "", err
	}
	if sum.IsNegative() {
		return decimal.Zero, cur, nil // the platform owes the driver; nothing to pay
	}
	return sum, cur, nil
}
