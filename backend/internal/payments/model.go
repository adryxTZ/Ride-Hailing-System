package payments

import (
	"net/http"
	"time"

	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
)

// Pay-in statuses (gateway_transactions.status).
const (
	StatusPending   = "PENDING"
	StatusCollected = "COLLECTED"
	StatusFailed    = "FAILED"
)

// FailExpired is our own failure code: nobody approved the prompt in time.
const FailExpired = "EXPIRED"

// Purposes of a gateway collection: a driver paying their debt, or a rider paying a fare.
const (
	purposeDriverDebt = "DRIVER_DEBT"
	purposeRideFare   = "RIDE_FARE"
)

// Timing rules.
const (
	// ExpireAfter is how long a USSD prompt may stay unanswered (the prompt itself times out in 1–2 min).
	ExpireAfter = 3 * time.Minute
	// FirstCheck is when the first status check runs after the push.
	FirstCheck = 8 * time.Second
	// CheckEvery is the gap between status checks while pending.
	CheckEvery = 10 * time.Second
	// ReconcileInterval is how often the sweep re-checks pending rows nobody checked lately.
	ReconcileInterval = 2 * time.Minute
	// ReconcileStaleAfter is how long since the last check before the sweep re-checks a row.
	ReconcileStaleAfter = 30 * time.Second
	// LateSuccessWindow is how long after expiry a gateway success is still credited (audited).
	LateSuccessWindow = 24 * time.Hour
	reconcileBatch    = 100
	// Push limits: a prompt goes to a real phone, so pushes are capped per driver and per number.
	PushesPerDriverPerHour = 5
	PushesPerPhonePerDay   = 3
	// Idempotency keys are client-chosen; bounds keep them sane.
	keyMinLen, keyMaxLen = 8, 64
)

// Payin is one debt payment request.
type Payin struct {
	ID          string          `json:"id"`
	DriverID    string          `json:"driver_id"`
	Status      string          `json:"status"`
	FailureCode *string         `json:"failure_code"`
	Amount      decimal.Decimal `json:"amount"`
	Currency    string          `json:"currency"`
	Phone       string          `json:"phone"`
	Gateway     string          `json:"gateway"`
	ProviderRef *string         `json:"provider_ref,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	CompletedAt *time.Time      `json:"completed_at"`
	ExpiresAt   time.Time       `json:"expires_at"`
	Purpose     string          `json:"purpose"`           // DRIVER_DEBT or RIDE_FARE
	TripID      *string         `json:"trip_id,omitempty"` // the trip a fare belongs to
}

// isFare reports whether this collection pays a ride fare (the payer is a rider).
func (p *Payin) isFare() bool { return p.Purpose == purposeRideFare && p.TripID != nil }

// AdminPayin is a pay-in with the driver's name, for the finance list.
type AdminPayin struct {
	Payin
	Name *string `json:"name"`
}

// Error codes owned by this module.
var (
	ErrKeyRequired   = apperr.New("IDEMPOTENCY_KEY_REQUIRED", http.StatusBadRequest, "")
	ErrBelowMinimum  = apperr.New("PAYIN_BELOW_MINIMUM", http.StatusBadRequest, "")
	ErrAboveDebt     = apperr.New("PAYIN_ABOVE_DEBT", http.StatusBadRequest, "")
	ErrNothingToPay  = apperr.New("NOTHING_TO_PAY", http.StatusConflict, "")
	ErrInProgress    = apperr.New("PAYIN_IN_PROGRESS", http.StatusConflict, "")
	ErrGatewayDown   = apperr.New("GATEWAY_UNAVAILABLE", http.StatusBadGateway, "")
	ErrPayinNotFound = apperr.New("PAYIN_NOT_FOUND", http.StatusNotFound, "")
	ErrRateLimited   = apperr.New("PAYIN_RATE_LIMITED", http.StatusTooManyRequests, "")
)
