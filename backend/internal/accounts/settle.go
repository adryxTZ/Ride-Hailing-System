package accounts

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/audit"
	"github.com/kalen-limited/hauler-rider/backend/internal/money"
)

// moneyScale is the decimals a payment may carry.
const moneyScale = 2

// dateLayout is the paid-on date format.
const dateLayout = "2006-01-02"

// RecordPayment records a payment against a payable account: one balanced
// ledger posting (the payable debited, PLATFORM_BANK credited), audited, in
// one transaction under the ledger lock. More than is owed is refused; a
// repeated Idempotency-Key returns the first payment without posting again.
func (s *Service) RecordPayment(ctx context.Context, in PaymentInput) (*Payment, error) {
	kind, payee, err := ParseKey(in.Key)
	if err != nil {
		return nil, err
	}
	if !info(kind).settleable {
		return nil, ErrNotSettleable
	}
	if in, err = s.normalise(in, kind); err != nil {
		return nil, err
	}
	var out *Payment
	err = pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		out, err = s.recordTx(ctx, tx, in, kind, payee)
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// recordTx is RecordPayment inside its transaction.
func (s *Service) recordTx(ctx context.Context, tx pgx.Tx, in PaymentInput, kind, payee string) (*Payment, error) {
	if err := s.ledger.LockTx(ctx, tx); err != nil {
		return nil, err
	}
	if prev, err := s.store.byIdemKey(ctx, tx, in.IdemKey); err != nil || prev != nil {
		return replay(prev, in, kind, payee, err)
	}
	paid, refundIDs, err := s.plan(ctx, tx, in, kind, payee)
	if err != nil {
		return nil, err
	}
	id, err := s.store.insert(ctx, tx, in, kind, payee)
	if err != nil {
		return nil, err
	}
	if err := s.store.markPaid(ctx, tx, refundIDs, id); err != nil {
		return nil, err
	}
	memo := fmt.Sprintf("Payment %s by %s (ref %s), paid on %s", in.Amount.StringFixed(moneyScale), methodWords[in.Method], in.Reference, in.PaidOn)
	if _, err := s.ledger.PostSettlementTx(ctx, tx, id, memo, in.Currency, paid); err != nil {
		return nil, err
	}
	if err := audit.Record(ctx, tx, s.store.brandID, in.StaffID, "finance.payment", "account:"+in.Key, in.Reason,
		map[string]any{"payment": id, "amount": in.Amount, "currency": in.Currency, "method": in.Method, "reference": in.Reference, "refunds": refundIDs}); err != nil {
		return nil, err
	}
	return s.store.byIdemKey(ctx, tx, in.IdemKey)
}

// replay answers a repeated key: the same payment again, or a conflict when
// the key was used for a different one.
func replay(prev *Payment, in PaymentInput, kind, payee string, err error) (*Payment, error) {
	if err != nil {
		return nil, err
	}
	same := prev.Kind == kind && deref(prev.PayeeID) == payee && prev.Currency == in.Currency && prev.Amount.Equal(in.Amount)
	if !same {
		return nil, ErrKeyReused
	}
	prev.Replayed = true
	return prev, nil
}

// plan is what the payment pays: the ledger legs and, for rider refunds, the
// refunds it marks paid. Overpaying is refused here, under the ledger lock.
func (s *Service) plan(ctx context.Context, tx pgx.Tx, in PaymentInput, kind, payee string) ([]money.Payable, []string, error) {
	if isRefunds(kind) {
		return s.planRefunds(ctx, tx, in)
	}
	if payee != "" {
		ok, err := s.store.payeeExists(ctx, tx, payee, in.Country)
		if err != nil || !ok {
			return nil, nil, cmpErr(err, ErrAccountNotFound)
		}
	}
	owed, err := s.store.owed(ctx, tx, kind, payee, in.Currency)
	if err != nil {
		return nil, nil, err
	}
	if in.Amount.GreaterThan(owed) {
		return nil, nil, ErrOverpay.WithDetails(map[string]any{"owed": decimal.Max(owed, decimal.Zero).StringFixed(moneyScale)})
	}
	return []money.Payable{{Kind: kind, OwnerID: payee, Amount: in.Amount}}, nil, nil
}

// planRefunds allocates a rider-refunds payment: chosen refunds, one rider's
// balance, or refunds oldest first; every rider's balance must cover their part.
func (s *Service) planRefunds(ctx context.Context, tx pgx.Tx, in PaymentInput) ([]money.Payable, []string, error) {
	f := refundFilter{currency: in.Currency, ids: in.RefundIDs, riderID: in.RiderID}
	owed, err := s.store.owedRefunds(ctx, tx, f, true)
	if err != nil {
		return nil, nil, err
	}
	var a allocation
	switch {
	case len(in.RefundIDs) > 0:
		a, err = allocateChosen(owed, in.RefundIDs, in.Amount)
	case in.RiderID != "":
		var balances map[string]decimal.Decimal
		if balances, err = s.store.ridersOwed(ctx, tx, in.Currency, []string{in.RiderID}); err == nil {
			a, err = allocateRider(owed, in.RiderID, balances[in.RiderID], in.Amount)
		}
	default:
		a, err = allocateOldest(owed, in.Amount)
	}
	if err != nil {
		return nil, nil, err
	}
	if err := s.checkRiderBalances(ctx, tx, in.Currency, a); err != nil {
		return nil, nil, err
	}
	return a.payables(), a.refundIDs, nil
}

// checkRiderBalances refuses an allocation that would pay a rider more than
// their RIDER_REFUNDS balance (one query for all riders).
func (s *Service) checkRiderBalances(ctx context.Context, tx pgx.Tx, currency string, a allocation) error {
	riders := make([]string, 0, len(a.perRider))
	for id := range a.perRider {
		riders = append(riders, id)
	}
	balances, err := s.store.ridersOwed(ctx, tx, currency, riders)
	if err != nil {
		return err
	}
	for id, amount := range a.perRider {
		if amount.GreaterThan(balances[id]) {
			return ErrOverpay.WithDetails(map[string]any{"owed": balances[id].StringFixed(moneyScale), "rider_id": id})
		}
	}
	return nil
}

// normalise trims and checks the input; the payment date may not be in the future.
func (s *Service) normalise(in PaymentInput, kind string) (PaymentInput, error) {
	in.Method, in.Reference, in.Reason = strings.TrimSpace(in.Method), strings.TrimSpace(in.Reference), strings.TrimSpace(in.Reason)
	in.IdemKey, in.PaidOn, in.RiderID = strings.TrimSpace(in.IdemKey), strings.TrimSpace(in.PaidOn), strings.TrimSpace(in.RiderID)
	if in.IdemKey == "" {
		return in, ErrKeyRequired
	}
	if len(in.IdemKey) > keyMaxLen {
		return in, invalid("idempotency_key")
	}
	if !in.Amount.IsPositive() || !in.Amount.Equal(in.Amount.Round(moneyScale)) {
		return in, invalid("amount") // positive, at most cents
	}
	if !slices.Contains(Methods, in.Method) {
		return in, invalid("method")
	}
	if in.Reference == "" || len(in.Reference) > referenceMaxLen {
		return in, invalid("reference")
	}
	if in.Reason == "" || len(in.Reason) > reasonMaxLen {
		return in, invalid("reason")
	}
	day, err := time.Parse(dateLayout, in.PaidOn)
	if err != nil || day.After(s.now().Add(paidOnSlack)) {
		return in, invalid("paid_on")
	}
	return in, checkRefundTargets(in, kind)
}

// checkRefundTargets allows refund ids or a rider only on rider refunds, never both.
func checkRefundTargets(in PaymentInput, kind string) error {
	if !isRefunds(kind) && (len(in.RefundIDs) > 0 || in.RiderID != "") {
		return invalid("refund_ids")
	}
	if len(in.RefundIDs) > 0 && in.RiderID != "" {
		return invalid("rider_id")
	}
	for _, id := range append(slices.Clone(in.RefundIDs), in.RiderID) {
		if _, err := uuid.Parse(id); id != "" && err != nil {
			return invalid("refund_ids")
		}
	}
	return nil
}

// invalid is VALIDATION_FAILED naming the field.
func invalid(field string) error {
	return ErrInvalid.WithDetails(map[string]any{"field": field})
}

// cmpErr returns err when set, else the fallback.
func cmpErr(err, fallback error) error {
	if err != nil {
		return err
	}
	return fallback
}
