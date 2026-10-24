package refunds

import (
	"net/http"
	"time"

	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
)

// Bearers.
const (
	BorneByPlatform = "PLATFORM"
	BorneByDriver   = "DRIVER"
)

const reasonMaxLen = 1000

// Refund is one refund on a trip.
type Refund struct {
	ID        string          `json:"id"`
	TripID    string          `json:"trip_id"`
	Amount    decimal.Decimal `json:"amount"`
	Currency  string          `json:"currency"`
	BorneBy   string          `json:"borne_by"`
	Reason    string          `json:"reason"`
	CreatedBy *string         `json:"created_by"`
	CreatedAt time.Time       `json:"created_at"`
	Status    string          `json:"status"` // OWED | PAID
	PaidAt    *time.Time      `json:"paid_at"`
}

// TripRefunds is what was paid, refunded and is still refundable on a trip.
type TripRefunds struct {
	Paid       decimal.Decimal `json:"paid"`
	Refunded   decimal.Decimal `json:"refunded"`
	Refundable decimal.Decimal `json:"refundable"`
	Currency   string          `json:"currency"`
	Refunds    []Refund        `json:"refunds"`
}

// Errors owned by this module.
var (
	ErrNotRefundable  = apperr.New("TRIP_NOT_REFUNDABLE", http.StatusConflict, "")
	ErrAmountTooLarge = apperr.New("REFUND_TOO_LARGE", http.StatusConflict, "")
	ErrInvalid        = apperr.New("VALIDATION_FAILED", http.StatusBadRequest, "")
	ErrTripNotFound   = apperr.New("TRIP_NOT_FOUND", http.StatusNotFound, "")
)
