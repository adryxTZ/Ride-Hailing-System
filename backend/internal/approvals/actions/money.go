package actions

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/approvals"
	"github.com/kalen-limited/hauler-rider/backend/internal/identity"
	"github.com/kalen-limited/hauler-rider/backend/internal/payouts"
	"github.com/kalen-limited/hauler-rider/backend/internal/refunds"
)

// approvalKeyPrefix makes an executed refund idempotent per approval request.
const approvalKeyPrefix = "approval:"

// shortID is the first characters of an id, for titles.
const shortID = 8

func short(id string) string {
	if len(id) > shortID {
		return id[:shortID]
	}
	return id
}

// whole reads an amount in whole units with thousands separators ("15,000").
func whole(d decimal.Decimal) string {
	digits := d.Round(0).Abs().String()
	var b strings.Builder
	if d.IsNegative() {
		b.WriteByte('-')
	}
	for i, c := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	return b.String()
}

func refundAction(svc *refunds.Service) approvals.Action {
	return approvals.Action{Key: TripRefund, Group: groupMoney, Permission: identity.PermTripsRefund, Scoped: true,
		Condition:   approvals.ConditionAmount,
		Label:       "Refund a trip",
		Description: "Give back part or all of a completed trip's fare (owed to the rider).",
		Describe:    describeRefund(svc), Execute: executeRefund(svc)}
}

// describeRefund shows the refund against what is left, and refuses one that
// cannot apply (not refundable, or more than what is left).
func describeRefund(svc *refunds.Service) approvals.Describer {
	return func(ctx context.Context, raw json.RawMessage) (approvals.Summary, error) {
		in, err := decode[RefundPayload](raw)
		if err != nil {
			return approvals.Summary{}, err
		}
		t, err := svc.ForTrip(ctx, in.TripID)
		if err != nil {
			return approvals.Summary{}, err
		}
		if !in.Amount.IsPositive() {
			return approvals.Summary{}, refunds.ErrInvalid
		}
		if in.Amount.GreaterThan(t.Refundable) {
			return approvals.Summary{}, refunds.ErrAmountTooLarge.WithDetails(map[string]any{"refundable": t.Refundable.String()})
		}
		return approvals.Summary{
			Title: fmt.Sprintf("Refund %s %s on trip %s", whole(in.Amount), t.Currency, short(in.TripID)),
			Changes: []approvals.Change{
				{Field: "refunded", Before: t.Refunded.String(), After: t.Refunded.Add(in.Amount).String()},
				{Field: "refundable", Before: t.Refundable.String(), After: t.Refundable.Sub(in.Amount).String()},
				{Field: "borne_by", Before: "", After: in.BorneBy},
			}}, nil
	}
}

func executeRefund(svc *refunds.Service) approvals.Executor {
	return func(ctx context.Context, x approvals.Execution) (any, error) {
		in, err := decode[RefundPayload](x.Payload)
		if err != nil {
			return nil, err
		}
		return svc.Create(ctx, x.RequesterID, in.TripID, in.Amount, in.BorneBy, x.Reason, approvalKeyPrefix+x.RequestID)
	}
}

func payoutAction(svc *payouts.Service) approvals.Action {
	return approvals.Action{Key: PayoutApprove, Group: groupMoney, Permission: identity.PermPayoutsApprove, Scoped: true,
		Condition:   approvals.ConditionAmount,
		Label:       "Approve a driver payout",
		Description: "Send a requested cash-out to the driver's mobile money account.",
		Describe:    describePayout(svc), Execute: executePayout(svc)}
}

func describePayout(svc *payouts.Service) approvals.Describer {
	return func(ctx context.Context, raw json.RawMessage) (approvals.Summary, error) {
		in, err := decode[PayoutPayload](raw)
		if err != nil {
			return approvals.Summary{}, err
		}
		p, err := svc.AdminGet(ctx, in.PayoutID)
		if err != nil {
			return approvals.Summary{}, err
		}
		if p.Status != payouts.StatusRequested {
			return approvals.Summary{}, payouts.ErrNotActionable
		}
		return approvals.Summary{
			Title: fmt.Sprintf("Pay out %s %s to %s", whole(p.Amount), p.Currency, p.Phone),
			Changes: []approvals.Change{
				{Field: "status", Before: p.Status, After: "APPROVED"},
				{Field: "amount", Before: "", After: p.Amount.String() + " " + p.Currency},
			}}, nil
	}
}

func executePayout(svc *payouts.Service) approvals.Executor {
	return func(ctx context.Context, x approvals.Execution) (any, error) {
		in, err := decode[PayoutPayload](x.Payload)
		if err != nil {
			return nil, err
		}
		return svc.Approve(ctx, x.RequesterID, in.PayoutID, x.Reason)
	}
}
