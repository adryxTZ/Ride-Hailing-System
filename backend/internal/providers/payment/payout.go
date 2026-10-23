package payment

import (
	"context"
	"errors"

	"github.com/shopspring/decimal"
)

// Payout states reported by PayoutStatus.
const (
	PayoutPending = "PENDING"
	PayoutPaid    = "PAID"
	PayoutFailed  = "FAILED"
)

// Payout failure codes reported with PayoutFailed.
const (
	FailWalletInvalid = "WALLET_INVALID" // the number has no wallet the gateway can pay
	FailFloatLow      = "FLOAT_TOO_LOW"  // our account at the gateway cannot cover it
	FailPayoutRefused = "PAYOUT_REFUSED" // the gateway refused for another reason
	FailUnsupported   = "WALLET_UNSUPPORTED"
)

// ErrPayoutRejected means the gateway definitely did not send the money (bad
// number, unsupported wallet, refused): the payout may be failed at once.
// Any other error from Disburse is ambiguous: the money may have gone, so the
// payout stays pending until PayoutStatus answers.
var ErrPayoutRejected = errors.New("payment: payout rejected")

// Payouter sends money from the platform's gateway account to a mobile wallet.
type Payouter interface {
	// Name is the gateway's key (stored on payouts).
	Name() string
	// MinPayout is the smallest payout the gateway sends in a currency (0 = none).
	MinPayout(currency string) decimal.Decimal
	// PayoutReference is what PayoutStatus will need, known before sending (stored first).
	PayoutReference(r DisburseRequest) string
	// Disburse sends the money. It must not retry internally; the caller owns
	// retries, and a repeated PayoutID must never pay twice.
	Disburse(ctx context.Context, r DisburseRequest) error
	// PayoutStatus is the gateway's answer for a payout: the only source of truth.
	PayoutStatus(ctx context.Context, ref string) (PayoutResult, error)
}

// DisburseRequest is one payout. PayoutID is our id (unique, used as the gateway's transaction id).
type DisburseRequest struct {
	PayoutID      string
	Phone         string // E.164
	RecipientName string
	Amount        decimal.Decimal
	Currency      string
	Note          string
}

// PayoutResult is the gateway's answer about a payout.
type PayoutResult struct {
	State         string // PENDING, PAID or FAILED
	FailureCode   string // with FAILED
	ProviderTxnID string // the gateway's own id, when paid
}
