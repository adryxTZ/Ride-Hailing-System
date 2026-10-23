// Package fake is a payment gateway for local development and CI. It needs no
// network and no state: the outcome is encoded in the provider reference, so a
// restart or another process can still answer Status.
//
// The outcome is chosen by the last four digits of the payer's phone:
//
//	0001  fails, insufficient funds
//	0002  fails, rejected by the payer
//	0003  never completes (the pay-in expires)
//	0004  Collect fails: gateway unavailable (Status later says FAILED/GATEWAY_ERROR)
//	else  collected
//
// Every outcome takes effect after Delay.
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

// Name is this adapter's key.
const Name = "fake"

// minTZS mirrors the real gateway's minimum so local behaviour matches production.
var minTZS = decimal.NewFromInt(1000)

// outcomes by phone suffix.
const (
	outcomeCollected = "ok"
	outcomeNoFunds   = "nofunds"
	outcomeRejected  = "rejected"
	outcomeNever     = "never"
	outcomeDown      = "down"
)

// Gateway is the fake adapter.
type Gateway struct {
	Delay time.Duration
	Now   func() time.Time
}

// New returns a fake gateway that settles after delay.
func New(delay time.Duration) *Gateway { return &Gateway{Delay: delay, Now: time.Now} }

// Name implements payment.Gateway.
func (g *Gateway) Name() string { return Name }

// MinAmount implements payment.Gateway.
func (g *Gateway) MinAmount(currency string) decimal.Decimal {
	if currency == "TZS" {
		return minTZS
	}
	return decimal.Zero
}

// outcomeFor picks the scripted outcome from the payer's phone.
func outcomeFor(phone string) string {
	switch {
	case strings.HasSuffix(phone, "0001"):
		return outcomeNoFunds
	case strings.HasSuffix(phone, "0002"):
		return outcomeRejected
	case strings.HasSuffix(phone, "0003"):
		return outcomeNever
	case strings.HasSuffix(phone, "0004"):
		return outcomeDown
	}
	return outcomeCollected
}

// Reference implements payment.Gateway: the outcome and the clock, so Status needs no state.
func (g *Gateway) Reference(r payment.CollectRequest) string {
	return fmt.Sprintf("fake_%d_%s", g.Now().UnixMilli(), outcomeFor(r.Phone))
}

// Collect implements payment.Gateway.
func (g *Gateway) Collect(_ context.Context, r payment.CollectRequest) (payment.CollectResult, error) {
	if outcomeFor(r.Phone) == outcomeDown {
		return payment.CollectResult{}, payment.ErrUnavailable
	}
	return payment.CollectResult{}, nil
}

// Status implements payment.Gateway.
func (g *Gateway) Status(_ context.Context, ref string) (payment.StatusResult, error) {
	parts := strings.Split(ref, "_")
	if len(parts) != 3 || parts[0] != Name {
		return payment.StatusResult{}, fmt.Errorf("fake: unknown reference %q", ref)
	}
	ms, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return payment.StatusResult{}, fmt.Errorf("fake: bad reference %q", ref)
	}
	if g.Now().Before(time.UnixMilli(ms).Add(g.Delay)) {
		return payment.StatusResult{State: payment.StatePending}, nil
	}
	switch parts[2] {
	case outcomeNoFunds:
		return payment.StatusResult{State: payment.StateFailed, FailureCode: payment.FailInsufficientFunds}, nil
	case outcomeRejected:
		return payment.StatusResult{State: payment.StateFailed, FailureCode: payment.FailRejectedByUser}, nil
	case outcomeNever:
		return payment.StatusResult{State: payment.StatePending}, nil
	case outcomeDown:
		return payment.StatusResult{State: payment.StateFailed, FailureCode: payment.FailGateway}, nil
	}
	return payment.StatusResult{State: payment.StateCollected, ProviderTxnID: "FK" + parts[1]}, nil
}
