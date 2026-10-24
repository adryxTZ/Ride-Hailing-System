package refunds

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
)

func TestRefundable(t *testing.T) {
	ptr := func(s string) *string { return &s }
	for name, c := range map[string]struct {
		t    paidTrip
		want bool
	}{
		"completed, cash confirmed": {paidTrip{status: "COMPLETED", paymentStatus: ptr("CASH_CONFIRMED")}, true},
		"completed, mobile money":   {paidTrip{status: "COMPLETED", paymentStatus: ptr("COLLECTED")}, true},
		"completed, not paid":       {paidTrip{status: "COMPLETED", paymentStatus: ptr("DUE")}, false},
		"still awaiting payment":    {paidTrip{status: "AWAITING_PAYMENT", paymentStatus: ptr("CASH_CONFIRMED")}, false},
		"cancelled":                 {paidTrip{status: "CANCELLED_BY_RIDER"}, false},
	} {
		if got := refundable(&c.t); got != c.want {
			t.Errorf("%s: got %v", name, got)
		}
	}
}

type noLedger struct{}

func (noLedger) PostRefundTx(context.Context, pgx.Tx, string, string, string, string, decimal.Decimal, string) (bool, error) {
	return true, nil
}

func TestCreateRefusesBadInput(t *testing.T) {
	s := New(nil, "b", noLedger{}, nil)
	ctx := context.Background()
	trip := "6b425043-f04b-4a83-8a4e-c769168ea86a"
	for name, c := range map[string]struct {
		amount         decimal.Decimal
		bearer, reason string
	}{
		"no reason":      {decimal.NewFromInt(100), BorneByPlatform, " "},
		"zero amount":    {decimal.Zero, BorneByPlatform, "x"},
		"negative":       {decimal.NewFromInt(-5), BorneByDriver, "x"},
		"unknown bearer": {decimal.NewFromInt(100), "RIDER", "x"},
	} {
		if _, err := s.Create(ctx, "staff", trip, c.amount, c.bearer, c.reason, ""); err != ErrInvalid {
			t.Errorf("%s: got %v", name, err)
		}
	}
	if _, err := s.Create(ctx, "staff", "not-a-uuid", decimal.NewFromInt(1), BorneByPlatform, "x", ""); err != ErrTripNotFound {
		t.Errorf("a malformed trip id must be not found, got %v", err)
	}
}
