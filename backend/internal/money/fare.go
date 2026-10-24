package money

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
)

// RiderRefunds holds money riders paid that no trip could take; Finance owes it back.
const RiderRefunds = "RIDER_REFUNDS"

// txFareUnapplied is the ledger kind for a fare payment that arrived when its trip no longer waited for it.
const txFareUnapplied = "fare.unapplied"

// ErrFareMismatch means the money collected is not the fare plus dues the trip
// recorded: nothing is posted, so the gap is visible instead of being absorbed.
var ErrFareMismatch = fmt.Errorf("money: collected amount does not match the trip's fare")

// DigitalFare is a trip fare collected through the payment gateway.
type DigitalFare struct {
	TripID   string
	RiderID  string
	DriverID string
	Fees     json.RawMessage // the trip's Breakdown, fixed when the ride ended
	Dues     decimal.Decimal // earlier cancellation fees included in Collected
	Currency string
	// Collected is what the gateway took from the rider.
	Collected decimal.Decimal
}

// PostTripPaidTx records a trip paid by mobile money, inside the transaction
// that completes the trip: the money is at the gateway (clearing debited), the
// driver is owed their earnings (wallet credited), the platform keeps its net,
// VAT and levy become payable, and earlier dues the rider carried are cleared.
// One balanced trip.completed transaction; posting the same trip twice is a no-op.
func (l *Ledger) PostTripPaidTx(ctx context.Context, tx pgx.Tx, f DigitalFare) (bool, error) {
	var b Breakdown
	if err := json.Unmarshal(f.Fees, &b); err != nil {
		return false, fmt.Errorf("money: trip %s has no fare split: %w", f.TripID, err)
	}
	if !b.Total.Add(f.Dues).Equal(f.Collected) {
		return false, fmt.Errorf("%w: trip %s split %s + dues %s, collected %s", ErrFareMismatch, f.TripID, b.Total, f.Dues, f.Collected)
	}
	legs := []Leg{
		{Kind: GatewayClearing, Amount: f.Collected},
		{Kind: DriverWallet, OwnerID: f.DriverID, Amount: b.DriverEarnings.Neg()},
		{Kind: PlatformRevenue, Amount: b.PlatformNet.Neg()},
		{Kind: VATPayable, Amount: b.VAT.Neg()},
		{Kind: LevyPayable, Amount: b.Levy.Neg()},
	}
	legs = append(legs, thirdPartyLegs(b)...)
	if f.Dues.IsPositive() {
		legs = append(legs, Leg{Kind: RiderDues, OwnerID: f.RiderID, Amount: f.Dues.Neg()})
	}
	memo := fmt.Sprintf("Mobile money trip: fare %s, your earnings %s", b.Total, b.DriverEarnings)
	return l.PostTx(ctx, tx, txTripCompleted, f.TripID, memo, f.Currency, legs)
}

// PostFareUnappliedTx records fare money that arrived after its trip was settled
// another way (the rider paid cash meanwhile): it sits at the gateway and is
// owed back to the rider until Finance refunds it. Idempotent per gateway row.
func (l *Ledger) PostFareUnappliedTx(ctx context.Context, tx pgx.Tx, payinID, riderID string, amount decimal.Decimal, currency string) (bool, error) {
	legs := []Leg{
		{Kind: GatewayClearing, Amount: amount},
		{Kind: RiderRefunds, OwnerID: riderID, Amount: amount.Neg()},
	}
	return l.PostTx(ctx, tx, txFareUnapplied, payinID, fmt.Sprintf("Fare payment %s arrived after the trip was settled; owed to the rider", amount), currency, legs)
}

// WalletMoved tells wallet listeners (holds, the driver's app) that a posting
// made inside a caller's transaction moved this driver's wallet. Call after commit.
func (l *Ledger) WalletMoved(ctx context.Context, driverID string) {
	l.notifyWallets(ctx, []Leg{{Kind: DriverWallet, OwnerID: driverID, Amount: decimal.NewFromInt(1)}})
}

// txRefund is the ledger kind for a refund staff granted on a completed trip.
const txRefund = "trip.refund"

// Refund bearers.
const (
	RefundByPlatform = "PLATFORM"
	RefundByDriver   = "DRIVER"
)

// PostRefundTx records a refund owed to the rider (RIDER_REFUNDS, paid out by
// Finance), taken from the platform's revenue or recovered from the driver's
// wallet. Idempotent per refund id.
func (l *Ledger) PostRefundTx(ctx context.Context, tx pgx.Tx, refundID, riderID, driverID, bearer string, amount decimal.Decimal, currency string) (bool, error) {
	from := Leg{Kind: PlatformRevenue, Amount: amount}
	memo := fmt.Sprintf("Refund %s to the rider, borne by the platform", amount)
	if bearer == RefundByDriver {
		from = Leg{Kind: DriverWallet, OwnerID: driverID, Amount: amount} // the driver owes it back
		memo = fmt.Sprintf("Refund %s to the rider, recovered from your earnings", amount)
	}
	legs := []Leg{from, {Kind: RiderRefunds, OwnerID: riderID, Amount: amount.Neg()}}
	return l.PostTx(ctx, tx, txRefund, refundID, memo, currency, legs)
}
