package payouts

import (
	"net/http"
	"time"

	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
)

// Payout statuses (driver_payouts.status).
const (
	StatusRequested = "REQUESTED"
	StatusPending   = "PENDING"
	StatusPaid      = "PAID"
	StatusFailed    = "FAILED"
	StatusRejected  = "REJECTED"
	StatusCancelled = "CANCELLED"
)

// FailBalanceChanged: the driver's balance dropped below the payout before it was sent.
const FailBalanceChanged = "BALANCE_CHANGED"

// Limits and timing.
const (
	MaxPerDay = 3 // cash-outs per driver per day (rejected and cancelled ones do not count)
	// StuckAfter flags a payout still pending at the gateway for Finance to look at.
	StuckAfter = 30 * time.Minute
	FirstCheck = 10 * time.Second
	CheckEvery = 15 * time.Second
	// ReconcileInterval is how often the sweep looks at open payouts nobody checked lately.
	ReconcileInterval = 2 * time.Minute
	staleAfter        = time.Minute
	// lateWindow is how long a refused payout is re-checked in case it paid after all.
	lateWindow      = 24 * time.Hour
	reconcileBatch  = 100
	keyMinLen       = 8
	keyMaxLen       = 64
	driverListLimit = 20
	adminListLimit  = 200
)

// Payout is one cash-out request.
type Payout struct {
	ID             string          `json:"id"`
	DriverID       string          `json:"driver_id"`
	Amount         decimal.Decimal `json:"amount"`
	Currency       string          `json:"currency"`
	Phone          string          `json:"phone"`
	Gateway        string          `json:"gateway"`
	Status         string          `json:"status"`
	FailureCode    *string         `json:"failure_code"`
	ProviderRef    *string         `json:"provider_ref,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	DecidedAt      *time.Time      `json:"decided_at"`
	DecisionReason *string         `json:"decision_reason"`
	SentAt         *time.Time      `json:"sent_at"`
	CompletedAt    *time.Time      `json:"completed_at"`
}

// AdminPayout is a payout as Finance sees it: who, and whether it is stuck.
type AdminPayout struct {
	Payout
	Name    *string         `json:"name"`
	Balance decimal.Decimal `json:"balance"` // what the platform owes the driver now
	Stuck   bool            `json:"stuck"`
}

// Wallet is the cash-out part of a driver's wallet screen.
type Wallet struct {
	Enabled   bool            `json:"payouts_enabled"`
	Available decimal.Decimal `json:"available_payout"`
	Min       decimal.Decimal `json:"payout_min"`
	Open      *Payout         `json:"open_payout"`
}

// Totals are the payout figures on the finance page.
type Totals struct {
	InProgress decimal.Decimal `json:"payouts_in_progress"`
	PaidToday  decimal.Decimal `json:"payouts_paid_today"`
}

// Error codes owned by this module.
var (
	ErrUnavailable    = apperr.New("PAYOUTS_UNAVAILABLE", http.StatusServiceUnavailable, "")
	ErrKeyRequired    = apperr.New("IDEMPOTENCY_KEY_REQUIRED", http.StatusBadRequest, "")
	ErrBelowMinimum   = apperr.New("PAYOUT_BELOW_MINIMUM", http.StatusBadRequest, "")
	ErrAboveBalance   = apperr.New("PAYOUT_ABOVE_BALANCE", http.StatusBadRequest, "")
	ErrNothingToPay   = apperr.New("NOTHING_TO_PAY_OUT", http.StatusConflict, "")
	ErrInProgress     = apperr.New("PAYOUT_IN_PROGRESS", http.StatusConflict, "")
	ErrBlockedByHold  = apperr.New("PAYOUT_BLOCKED_BY_HOLD", http.StatusConflict, "")
	ErrRateLimited    = apperr.New("PAYOUT_RATE_LIMITED", http.StatusTooManyRequests, "")
	ErrNotFound       = apperr.New("PAYOUT_NOT_FOUND", http.StatusNotFound, "")
	ErrNotActionable  = apperr.New("PAYOUT_NOT_ACTIONABLE", http.StatusConflict, "")
	ErrReasonRequired = apperr.New("REASON_REQUIRED", http.StatusBadRequest, "")
)
