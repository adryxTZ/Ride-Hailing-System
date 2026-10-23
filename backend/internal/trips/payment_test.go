package trips

import (
	"errors"
	"testing"

	"github.com/shopspring/decimal"
)

var dec = decimal.RequireFromString

func status(s string) *string { return &s }

func TestDriverConfirmsCashOnlyWhenNoPromptIsOpen(t *testing.T) {
	cases := []struct {
		name string
		in   *string
		want error
	}{
		{"fare due", status(PayDue), nil},
		{"prompt failed", status(PayFailed), nil},
		{"prompt open", status(PayPending), ErrFarePromptPending},
		{"already collected", status(PayCollected), ErrInvalidTransition},
		{"already cash", status(PayCashConfirmed), ErrInvalidTransition},
		{"ended before payment states", nil, nil},
	}
	for _, c := range cases {
		if err := cashConfirmable(c.in); !errors.Is(err, c.want) && err != c.want {
			t.Errorf("%s: got %v, want %v", c.name, err, c.want)
		}
	}
}

func TestRiderMayPromptOnlyWhileTheFareIsOwed(t *testing.T) {
	cases := []struct {
		name   string
		status Status
		pay    *string
		want   error
	}{
		{"due", AwaitingPayment, status(PayDue), nil},
		{"failed", AwaitingPayment, status(PayFailed), nil},
		{"open prompt", AwaitingPayment, status(PayPending), ErrFarePromptPending},
		{"paid", Completed, status(PayCollected), ErrPaymentNotDue},
		{"still riding", InTransit, nil, ErrPaymentNotDue},
	}
	for _, c := range cases {
		err := promptable(&Trip{Status: c.status, PaymentStatus: c.pay})
		if !errors.Is(err, c.want) && err != c.want {
			t.Errorf("%s: got %v, want %v", c.name, err, c.want)
		}
	}
}

func TestFareAmountIsTheFinalFareWhenSet(t *testing.T) {
	tr := &Trip{EstimatedFare: dec("5000")}
	if !fareAmount(tr).Equal(dec("5000")) {
		t.Fatal("estimate expected before the fare is fixed")
	}
	final := dec("5600")
	tr.FinalFare = &final
	if !fareAmount(tr).Equal(final) {
		t.Fatal("the final fare (with waiting and dues) is what the rider pays")
	}
}
