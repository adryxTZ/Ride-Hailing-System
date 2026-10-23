package holds

import (
	"net/http"
	"time"

	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
)

// Hold kinds.
const (
	KindDebt   = "DEBT"
	KindManual = "MANUAL"
	// KindDocument is placed and lifted by the system while a required document is expired.
	KindDocument = "DOCUMENT"
)

// Hold causes.
const (
	CauseHardLimit        = "HARD_LIMIT"
	CauseSoftLimitOverdue = "SOFT_LIMIT_OVERDUE"
	CauseStaff            = "STAFF"
	CauseDocumentExpired  = "DOCUMENT_EXPIRED"
)

// Hold is one active reason a driver cannot go online.
type Hold struct {
	ID     string    `json:"id"`
	Kind   string    `json:"kind"`
	Cause  string    `json:"cause"`
	Reason *string   `json:"reason"`
	Since  time.Time `json:"since"`
}

// Status is a driver's debt position against the rule, for the wallet screen.
type Status struct {
	Debt        decimal.Decimal  `json:"debt"`
	Limits      Limits           `json:"limits"`
	Holds       []Hold           `json:"holds"`
	SoftHoldAt  *time.Time       `json:"soft_hold_at"`
	PayToUnhold *decimal.Decimal `json:"pay_to_unhold"`
}

// ActiveHold is a hold with the driver it belongs to, for the finance list.
type ActiveHold struct {
	Hold     Hold            `json:"hold"`
	DriverID string          `json:"driver_id"`
	Name     *string         `json:"name"`
	Phone    string          `json:"phone"`
	Debt     decimal.Decimal `json:"debt"`
}

// Error codes owned by this module.
var (
	ErrHoldExists     = apperr.New("HOLD_EXISTS", http.StatusConflict, "")
	ErrNoManualHold   = apperr.New("NO_MANUAL_HOLD", http.StatusNotFound, "")
	ErrReasonRequired = apperr.New("REASON_REQUIRED", http.StatusBadRequest, "")
	ErrOnHold         = apperr.New("DRIVER_ON_HOLD", http.StatusConflict, "")
)

// ErrDriverNotFound is returned when a hold targets someone who is not a driver.
var ErrDriverNotFound = apperr.New("DRIVER_NOT_FOUND", http.StatusNotFound, "")
