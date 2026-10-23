package receipts

import (
	"net/http"
	"time"

	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
)

// Receipt is the record of what a rider paid for one trip. Never changed once issued.
type Receipt struct {
	ID       string          `json:"id"`
	Number   string          `json:"number"`
	IssuedAt time.Time       `json:"issued_at"`
	TripID   string          `json:"trip_id"`
	Currency string          `json:"currency"`
	Total    decimal.Decimal `json:"total"`
	Lines    []Line          `json:"lines"`
	Taxes    Taxes           `json:"taxes"`
	Payment  Payment         `json:"payment"`
	Issuer   Issuer          `json:"issuer"`
	Trip     TripInfo        `json:"trip"`
	Driver   DriverInfo      `json:"driver"`
	riderID  string
	driverID string
}

// Line is one part of the total, labelled in each language.
type Line struct {
	Code   string            `json:"code"`
	Label  map[string]string `json:"label"`
	Amount decimal.Decimal   `json:"amount"`
	Meta   map[string]any    `json:"meta,omitempty"`
}

// Taxes are included in the total (never added on top).
type Taxes struct {
	VAT      decimal.Decimal `json:"vat"`
	Levy     decimal.Decimal `json:"levy"`
	Included bool            `json:"included"`
}

// Payment is how the fare was settled.
type Payment struct {
	Method    string  `json:"method"` // CASH or MOBILE_MONEY
	Status    string  `json:"status"` // COLLECTED or CASH_CONFIRMED
	Reference *string `json:"reference,omitempty"`
}

// Issuer is the business issuing the receipt, as it was on the day.
type Issuer struct {
	Name         string `json:"name"`
	LegalEntity  string `json:"legal_entity,omitempty"`
	SupportPhone string `json:"support_phone,omitempty"`
	SupportEmail string `json:"support_email,omitempty"`
	Country      string `json:"country"`
	// Timezone is the country's (IANA): times on the receipt are local.
	Timezone string `json:"timezone"`
}

// TripInfo is the trip as it was completed.
type TripInfo struct {
	Code         string     `json:"code"`
	Service      string     `json:"service"`
	VehicleClass string     `json:"vehicle_class"`
	Pickup       string     `json:"pickup"`
	Dropoff      string     `json:"dropoff"`
	DistanceM    *int       `json:"distance_m,omitempty"`
	DurationS    *int       `json:"duration_s,omitempty"`
	StartedAt    *time.Time `json:"started_at,omitempty"`
	CompletedAt  time.Time  `json:"completed_at"`
}

// DriverInfo is the driver and vehicle as they were on the day.
type DriverInfo struct {
	Name  string `json:"name"`
	Plate string `json:"plate,omitempty"`
}

// FactLine is one fare line as the trip recorded it.
type FactLine struct {
	Code   string
	Amount decimal.Decimal
	Meta   map[string]any
	// Label is the line's own wording (extra charges carry theirs), used before the code table.
	Label map[string]string
}

// Facts is everything the trip provides to issue its receipt.
type Facts struct {
	TripID, RiderID, DriverID string
	Country, Currency         string
	Lines                     []FactLine
	Total                     decimal.Decimal // what the rider paid
	VAT, Levy                 decimal.Decimal
	Payment                   Payment
	Trip                      TripInfo
	Driver                    DriverInfo
}

// Error codes owned by this module.
var (
	ErrReceiptNotFound = apperr.New("RECEIPT_NOT_FOUND", http.StatusNotFound, "")
	ErrLinkInvalid     = apperr.New("RECEIPT_LINK_INVALID", http.StatusForbidden, "")
	// ErrLinesMismatch means the fare lines do not add up to what was paid; nothing is issued.
	ErrLinesMismatch = apperr.New("RECEIPT_LINES_MISMATCH", http.StatusInternalServerError, "")
)
