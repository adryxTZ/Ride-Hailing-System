// Package payment is the port to mobile-money and card gateways. Adapters live
// in sub-packages (fake for local and CI, selcom for Tanzania); the brand's
// config picks one at startup.
//
// Collections only for now (a USSD push the payer approves with their PIN).
// Payouts will be a separate Payouter interface in the payouts slice.
//
// Files: payment.go (the port and shared types).
package payment

import (
	"context"
	"errors"

	"github.com/shopspring/decimal"
)

// Collection states reported by Status.
const (
	StatePending   = "PENDING"
	StateCollected = "COLLECTED"
	StateFailed    = "FAILED"
)

// Failure codes reported with StateFailed.
const (
	FailInsufficientFunds = "INSUFFICIENT_FUNDS"
	FailRejectedByUser    = "REJECTED_BY_USER"
	FailGateway           = "GATEWAY_ERROR"
)

// Errors adapters return.
var (
	ErrNotConfigured = errors.New("payment: gateway credentials are missing")
	ErrUnavailable   = errors.New("payment: gateway unavailable")
)

// Gateway collects money from a payer's mobile-money wallet.
type Gateway interface {
	// Name is the gateway's key (used in webhook paths and stored on transactions).
	Name() string
	// MinAmount is the smallest amount the gateway accepts in a currency (0 = no minimum).
	MinAmount(currency string) decimal.Decimal
	// Reference is the identifier Status will need for this request, known before
	// the push is sent. It is stored first, so a reply lost in transit (timeout,
	// cancelled request) can still be reconciled instead of losing the money.
	Reference(r CollectRequest) string
	// Collect asks the payer to approve a payment (USSD push). It must not retry
	// internally: the caller owns retries and idempotency.
	Collect(ctx context.Context, r CollectRequest) (CollectResult, error)
	// Status asks the gateway what happened. It is the only source of truth;
	// webhook payloads are never trusted.
	Status(ctx context.Context, providerRef string) (StatusResult, error)
}

// CollectRequest is one payment request. OrderID is our transaction id.
type CollectRequest struct {
	OrderID     string
	Phone       string // E.164
	Amount      decimal.Decimal
	Currency    string
	Description string
	CallbackURL string
}

// CollectResult is the gateway's answer to a push. ProviderRef is set only when
// the gateway assigned a reference different from Reference(r).
type CollectResult struct {
	ProviderRef string
}

// StatusResult is the gateway's answer.
type StatusResult struct {
	State         string // PENDING, COLLECTED or FAILED
	FailureCode   string // with FAILED
	ProviderTxnID string // the gateway's own transaction id, when collected
}
