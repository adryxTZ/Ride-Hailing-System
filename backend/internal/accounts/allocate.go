package accounts

import (
	"slices"

	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/money"
)

// allocation is what a rider-refunds payment covers: the refunds it marks
// PAID and the amount taken off each rider's balance.
type allocation struct {
	refundIDs []string
	perRider  map[string]decimal.Decimal
}

// payables turns an allocation into ledger legs, ordered by rider for a stable chain.
func (a allocation) payables() []money.Payable {
	riders := make([]string, 0, len(a.perRider))
	for id := range a.perRider {
		riders = append(riders, id)
	}
	slices.Sort(riders)
	out := make([]money.Payable, 0, len(riders))
	for _, id := range riders {
		out = append(out, money.Payable{Kind: money.RiderRefunds, OwnerID: id, Amount: a.perRider[id]})
	}
	return out
}

// take adds whole refunds to an allocation.
func take(refunds []OwedRefund) allocation {
	a := allocation{refundIDs: []string{}, perRider: map[string]decimal.Decimal{}}
	for _, r := range refunds {
		a.refundIDs = append(a.refundIDs, r.ID)
		a.perRider[r.RiderID] = a.perRider[r.RiderID].Add(r.Amount)
	}
	return a
}

// allocateChosen pays exactly the refunds staff picked: every one must still
// be owed and the amount must be their sum.
func allocateChosen(owed []OwedRefund, ids []string, amount decimal.Decimal) (allocation, error) {
	if len(owed) != len(uniq(ids)) {
		return allocation{}, ErrRefundNotOwed
	}
	sum := decimal.Zero
	for _, r := range owed {
		sum = sum.Add(r.Amount)
	}
	if !sum.Equal(amount) {
		return allocation{}, ErrAmountMismatch.WithDetails(map[string]any{"expected": sum.StringFixed(2)})
	}
	return take(owed), nil
}

// allocateOldest pays refunds oldest first; the amount must cover a whole
// number of them (a refund is paid in full or not at all).
func allocateOldest(owed []OwedRefund, amount decimal.Decimal) (allocation, error) {
	total := decimal.Zero
	for _, r := range owed {
		total = total.Add(r.Amount)
	}
	if amount.GreaterThan(total) {
		return allocation{}, ErrOverpay.WithDetails(map[string]any{"owed": total.StringFixed(2)})
	}
	sum := decimal.Zero
	for i, r := range owed {
		next := sum.Add(r.Amount)
		if next.Equal(amount) {
			return take(owed[:i+1]), nil
		}
		if next.GreaterThan(amount) {
			return allocation{}, ErrAmountMismatch.WithDetails(map[string]any{"below": sum.StringFixed(2), "above": next.StringFixed(2)})
		}
		sum = next
	}
	return allocation{}, ErrAmountMismatch.WithDetails(map[string]any{"below": sum.StringFixed(2)})
}

// allocateRider pays down one rider's whole refund balance (refunds and late
// fare money) up to what they are owed; refunds the amount fully covers,
// oldest first, are marked paid.
func allocateRider(owed []OwedRefund, riderID string, riderOwed, amount decimal.Decimal) (allocation, error) {
	if amount.GreaterThan(riderOwed) {
		return allocation{}, ErrOverpay.WithDetails(map[string]any{"owed": riderOwed.StringFixed(2)})
	}
	covered, sum := 0, decimal.Zero
	for _, r := range owed {
		if sum.Add(r.Amount).GreaterThan(amount) {
			break
		}
		sum, covered = sum.Add(r.Amount), covered+1
	}
	a := take(owed[:covered])
	a.perRider = map[string]decimal.Decimal{riderID: amount}
	return a, nil
}

// uniq drops repeated ids.
func uniq(ids []string) []string {
	out := slices.Clone(ids)
	slices.Sort(out)
	return slices.Compact(out)
}
