package trips

import "testing"

func TestPriceChangeable(t *testing.T) {
	ptr := func(s string) *string { return &s }
	for name, c := range map[string]struct {
		status  Status
		payment *string
		want    error
	}{
		"before pickup":        {Accepted, nil, nil},
		"in transit":           {InTransit, nil, nil},
		"fare due":             {AwaitingPayment, ptr(PayDue), nil},
		"last prompt failed":   {AwaitingPayment, ptr(PayFailed), nil},
		"prompt open":          {AwaitingPayment, ptr(PayPending), ErrPaymentInProgress},
		"paid by mobile money": {AwaitingPayment, ptr(PayCollected), ErrPriceLocked},
		"cash confirmed":       {AwaitingPayment, ptr(PayCashConfirmed), ErrPriceLocked},
		"completed":            {Completed, ptr(PayCashConfirmed), ErrPriceLocked},
		"cancelled":            {CancelledByRider, nil, ErrPriceLocked},
	} {
		if got := priceChangeable(&Trip{Status: c.status, PaymentStatus: c.payment}); got != c.want {
			t.Errorf("%s: got %v, want %v", name, got, c.want)
		}
	}
}
