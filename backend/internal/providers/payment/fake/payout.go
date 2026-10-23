package fake

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/providers/payment"
)

// Payout outcomes by the last four digits of the recipient's phone:
//
//	0001  fails: our float at the gateway is too low
//	0002  refused at once: no wallet on that number (Disburse returns ErrPayoutRejected)
//	0003  never completes (stays pending: the payout is flagged as stuck)
//	0004  Disburse errors (ambiguous), but the money went: Status later says PAID
//	else  paid
const (
	payoutPaid      = "paid"
	payoutFloatLow  = "float"
	payoutNoWallet  = "nowallet"
	payoutNever     = "never"
	payoutAmbiguous = "ambiguous"
)

func payoutOutcome(phone string) string {
	switch {
	case strings.HasSuffix(phone, "0001"):
		return payoutFloatLow
	case strings.HasSuffix(phone, "0002"):
		return payoutNoWallet
	case strings.HasSuffix(phone, "0003"):
		return payoutNever
	case strings.HasSuffix(phone, "0004"):
		return payoutAmbiguous
	}
	return payoutPaid
}

// MinPayout implements payment.Payouter (same minimum as collections).
func (g *Gateway) MinPayout(currency string) decimal.Decimal { return g.MinAmount(currency) }

// PayoutReference implements payment.Payouter: outcome and clock, so status needs no state.
func (g *Gateway) PayoutReference(r payment.DisburseRequest) string {
	return fmt.Sprintf("fakepo_%d_%s", g.Now().UnixMilli(), payoutOutcome(r.Phone))
}

// Disburse implements payment.Payouter.
func (g *Gateway) Disburse(_ context.Context, r payment.DisburseRequest) error {
	switch payoutOutcome(r.Phone) {
	case payoutNoWallet:
		return fmt.Errorf("%w: %s", payment.ErrPayoutRejected, payment.FailWalletInvalid)
	case payoutAmbiguous:
		return payment.ErrUnavailable
	}
	return nil
}

// PayoutStatus implements payment.Payouter.
func (g *Gateway) PayoutStatus(_ context.Context, ref string) (payment.PayoutResult, error) {
	parts := strings.Split(ref, "_")
	if len(parts) != 3 || parts[0] != "fakepo" {
		return payment.PayoutResult{}, fmt.Errorf("fake: unknown payout reference %q", ref)
	}
	ms, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return payment.PayoutResult{}, fmt.Errorf("fake: bad payout reference %q", ref)
	}
	if g.Now().Before(time.UnixMilli(ms).Add(g.Delay)) {
		return payment.PayoutResult{State: payment.PayoutPending}, nil
	}
	switch parts[2] {
	case payoutFloatLow:
		return payment.PayoutResult{State: payment.PayoutFailed, FailureCode: payment.FailFloatLow}, nil
	case payoutNoWallet:
		return payment.PayoutResult{State: payment.PayoutFailed, FailureCode: payment.FailWalletInvalid}, nil
	case payoutNever:
		return payment.PayoutResult{State: payment.PayoutPending}, nil
	}
	return payment.PayoutResult{State: payment.PayoutPaid, ProviderTxnID: "FKPO" + parts[1]}, nil
}
