package money

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/shopspring/decimal"
)

// Ledger transaction kinds for cancellation money.
const (
	txCancellationFee = "trip.cancellation_fee"
	txDuesCollected   = "trip.dues_collected"
)

// PostCancellationFee records a late-cancel or no-show fee: the rider owes it
// (RIDER_DUES), the driver is owed their share, the platform keeps the rest.
// Idempotent per trip.
func (l *Ledger) PostCancellationFee(ctx context.Context, tripID, riderID, driverID string, fee, driverShare decimal.Decimal, currency string) (bool, error) {
	toDriver := decimal.Zero
	if driverID != "" {
		toDriver = fee.Mul(driverShare).Round(0)
	}
	legs := []Leg{{Kind: RiderDues, OwnerID: riderID, Amount: fee}, {Kind: PlatformRevenue, Amount: fee.Sub(toDriver).Neg()}}
	if toDriver.IsPositive() {
		legs = append(legs, Leg{Kind: DriverWallet, OwnerID: driverID, Amount: toDriver.Neg()})
	}
	memo := fmt.Sprintf("Cancellation fee %s, driver's share %s", fee, toDriver)
	return l.Post(ctx, txCancellationFee, tripID, memo, currency, legs)
}

// PostDuesCollected records earlier cancellation fees the rider paid (in cash)
// on a later trip: the driver holds that cash, so it becomes the driver's debt,
// and the rider no longer owes it. Idempotent per trip.
func (l *Ledger) PostDuesCollected(ctx context.Context, tripID, riderID, driverID string, amount decimal.Decimal, currency string) (bool, error) {
	legs := []Leg{{Kind: DriverWallet, OwnerID: driverID, Amount: amount}, {Kind: RiderDues, OwnerID: riderID, Amount: amount.Neg()}}
	return l.Post(ctx, txDuesCollected, tripID, fmt.Sprintf("Collected %s from an earlier cancellation", amount), currency, legs)
}

// PostDuesCollectedTx is PostDuesCollected inside the completing transaction.
func (l *Ledger) PostDuesCollectedTx(ctx context.Context, tx pgx.Tx, tripID, riderID, driverID string, amount decimal.Decimal, currency string) (bool, error) {
	legs := []Leg{{Kind: DriverWallet, OwnerID: driverID, Amount: amount}, {Kind: RiderDues, OwnerID: riderID, Amount: amount.Neg()}}
	return l.PostTx(ctx, tx, txDuesCollected, tripID, fmt.Sprintf("Collected %s from an earlier cancellation", amount), currency, legs)
}

// RiderDues is what a rider owes from earlier cancellations (0 when nothing).
func (l *Ledger) RiderDues(ctx context.Context, riderID string) (decimal.Decimal, error) {
	var owed decimal.Decimal
	err := l.pool.QueryRow(ctx, `SELECT coalesce(sum(e.amount), 0) FROM ledger_entries e JOIN ledger_accounts a ON a.id=e.account_id
		WHERE a.brand_id=$1 AND a.kind='RIDER_DUES' AND a.owner_id=$2`, l.brandID, riderID).Scan(&owed)
	return owed, err
}
