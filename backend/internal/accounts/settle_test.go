package accounts

import (
	"context"
	"errors"
	"testing"
	"time"
)

func goodInput() PaymentInput {
	return PaymentInput{Key: "VAT_PAYABLE", Currency: "TZS", Amount: d("1000"), PaidOn: "2026-10-24",
		Method: "BANK_TRANSFER", Reference: "TRA-1", Reason: "October VAT", IdemKey: "k1", StaffID: "s"}
}

func TestRecordPaymentRefusesBadInputBeforeTheDatabase(t *testing.T) {
	s := New(nil, "b", nil)
	s.now = func() time.Time { return time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC) }
	for name, c := range map[string]struct {
		edit func(*PaymentInput)
		want error
	}{
		"no idempotency key":  {func(p *PaymentInput) { p.IdemKey = " " }, ErrKeyRequired},
		"zero amount":         {func(p *PaymentInput) { p.Amount = d("0") }, ErrInvalid},
		"fractions of a cent": {func(p *PaymentInput) { p.Amount = d("10.001") }, ErrInvalid},
		"unknown method":      {func(p *PaymentInput) { p.Method = "GOLD" }, ErrInvalid},
		"no reference":        {func(p *PaymentInput) { p.Reference = "" }, ErrInvalid},
		"no reason":           {func(p *PaymentInput) { p.Reason = "  " }, ErrInvalid},
		"bad date":            {func(p *PaymentInput) { p.PaidOn = "24/10/2026" }, ErrInvalid},
		"future date":         {func(p *PaymentInput) { p.PaidOn = "2026-10-27" }, ErrInvalid},
		"refunds on VAT":      {func(p *PaymentInput) { p.RefundIDs = []string{"6b425043-f04b-4a83-8a4e-c769168ea86a"} }, ErrInvalid},
		"revenue":             {func(p *PaymentInput) { p.Key = "PLATFORM_REVENUE" }, ErrNotSettleable},
		"unknown account":     {func(p *PaymentInput) { p.Key = "SOMETHING" }, ErrAccountNotFound},
		"rider and refunds": {func(p *PaymentInput) {
			p.Key, p.RiderID, p.RefundIDs = "RIDER_REFUNDS", "6b425043-f04b-4a83-8a4e-c769168ea86a", []string{"6b425043-f04b-4a83-8a4e-c769168ea86b"}
		}, ErrInvalid},
	} {
		in := goodInput()
		c.edit(&in)
		if _, err := s.RecordPayment(context.Background(), in); !errors.Is(err, c.want) {
			t.Errorf("%s: got %v, want %v", name, err, c.want)
		}
	}
}
