package fake

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/providers/payment"
)

func TestOutcomesBySuffix(t *testing.T) {
	now := time.Date(2026, 10, 21, 10, 0, 0, 0, time.UTC)
	g := &Gateway{Delay: 5 * time.Second, Now: func() time.Time { return now }}
	cases := map[string]payment.StatusResult{
		"+255700000009": {State: payment.StateCollected},
		"+255700000001": {State: payment.StateFailed, FailureCode: payment.FailInsufficientFunds},
		"+255700000002": {State: payment.StateFailed, FailureCode: payment.FailRejectedByUser},
		"+255700000003": {State: payment.StatePending},
	}
	for phone, want := range cases {
		req := payment.CollectRequest{Phone: phone, Amount: decimal.NewFromInt(1000), Currency: "TZS"}
		res := payment.CollectResult{ProviderRef: g.Reference(req)}
		if _, err := g.Collect(context.Background(), req); err != nil {
			t.Fatal(err)
		}
		if st, _ := g.Status(context.Background(), res.ProviderRef); st.State != payment.StatePending {
			t.Fatalf("%s: settled before the delay: %+v", phone, st)
		}
		// A fresh instance later answers from the reference alone (stateless).
		later := &Gateway{Delay: 5 * time.Second, Now: func() time.Time { return now.Add(6 * time.Second) }}
		st, err := later.Status(context.Background(), res.ProviderRef)
		if err != nil || st.State != want.State || st.FailureCode != want.FailureCode {
			t.Fatalf("%s: got %+v %v, want %+v", phone, st, err, want)
		}
	}
}

func TestUnavailable(t *testing.T) {
	g := New(0)
	req := payment.CollectRequest{Phone: "+255700000004"}
	if _, err := g.Collect(context.Background(), req); !errors.Is(err, payment.ErrUnavailable) {
		t.Fatalf("want ErrUnavailable, got %v", err)
	}
	// A later re-check of the stored reference settles it as a gateway failure, never as money.
	if st, _ := g.Status(context.Background(), g.Reference(req)); st.State != payment.StateFailed || st.FailureCode != payment.FailGateway {
		t.Fatalf("want FAILED/GATEWAY_ERROR, got %+v", st)
	}
}

func TestPayoutOutcomes(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	g := &Gateway{Delay: time.Second, Now: func() time.Time { return now }}
	ctx := context.Background()
	cases := map[string]payment.PayoutResult{
		"+255712000009": {State: payment.PayoutPaid},
		"+255712000001": {State: payment.PayoutFailed, FailureCode: payment.FailFloatLow},
		"+255712000003": {State: payment.PayoutPending},
		"+255712000004": {State: payment.PayoutPaid},
	}
	for phone, want := range cases {
		req := payment.DisburseRequest{PayoutID: "p", Phone: phone}
		ref := g.PayoutReference(req)
		_ = g.Disburse(ctx, req)
		if st, _ := g.PayoutStatus(ctx, ref); st.State != payment.PayoutPending {
			t.Fatalf("%s: settles only after the delay", phone)
		}
		now = now.Add(2 * time.Second)
		st, err := g.PayoutStatus(ctx, ref)
		if err != nil || st.State != want.State || st.FailureCode != want.FailureCode {
			t.Fatalf("%s: got %+v %v, want %+v", phone, st, err, want)
		}
	}
	if err := g.Disburse(ctx, payment.DisburseRequest{Phone: "+255712000002"}); !errors.Is(err, payment.ErrPayoutRejected) {
		t.Fatalf("no wallet must be a definite rejection: %v", err)
	}
	if err := g.Disburse(ctx, payment.DisburseRequest{Phone: "+255712000004"}); errors.Is(err, payment.ErrPayoutRejected) || err == nil {
		t.Fatalf("an ambiguous failure must not look definite: %v", err)
	}
}
