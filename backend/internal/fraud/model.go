package fraud

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
)

// Rules.
const (
	RuleGPSSpoof         = "GPS_SPOOF"
	RuleSelfBooking      = "SELF_BOOKING"
	RuleDriverCancelRate = "DRIVER_CANCEL_RATE"
	RuleDriverNoShowRate = "DRIVER_NO_SHOW_RATE"
	RuleRiderCancelRate  = "RIDER_CANCEL_RATE"
)

// Severities.
const (
	SeverityLow    = "LOW"
	SeverityMedium = "MEDIUM"
	SeverityHigh   = "HIGH"
)

// Subject kinds and statuses.
const (
	SubjectDriver = "DRIVER"
	SubjectRider  = "RIDER"

	StatusOpen      = "OPEN"
	StatusDismissed = "DISMISSED"
	StatusConfirmed = "CONFIRMED"
)

// Review decisions.
const (
	DecisionDismiss = "DISMISS"
	DecisionConfirm = "CONFIRM"
)

// Detector settings (spec §5).
const (
	MinGapS               = 2                // pings closer in time than this are jitter
	MinJumpM              = 500              // moves shorter than this never strike
	MaxPlausibleSpeedMps  = 55.0             // 198 km/h
	GPSStrikeWindow       = 10 * time.Minute // strikes count inside this window
	GPSStrikesToFlag      = 3
	GPSEvidencePairs      = 3
	InstallLookback       = 90 * 24 * time.Hour
	CancelWindow          = 7 * 24 * time.Hour
	CancelScanHour        = 3 // local time the daily scan may run from
	DriverCancelRateFlag  = 0.30
	DriverCancelMinSample = 10
	NoShowRateFlag        = 0.30
	NoShowMinSample       = 10
	RiderCancelRateFlag   = 0.40
	RiderCancelMinSample  = 10
	evidenceTripIDs       = 20
	listLimitDefault      = 50
	listLimitMax          = 200
	reviewNoteMaxLen      = 1000
)

// Subject is who a flag is about.
type Subject struct {
	Kind  string  `json:"kind"`
	ID    string  `json:"id"`
	Name  *string `json:"name"`
	Phone string  `json:"phone"`
}

// Flag is one detector finding for staff to review.
type Flag struct {
	ID         string          `json:"id"`
	Country    *string         `json:"country"`
	Rule       string          `json:"rule"`
	Severity   string          `json:"severity"`
	Subject    Subject         `json:"subject"`
	TripID     *string         `json:"trip_id"`
	TripCode   *string         `json:"trip_code"`
	Evidence   json.RawMessage `json:"evidence"`
	Hits       int             `json:"hits"`
	Status     string          `json:"status"`
	OpenedAt   time.Time       `json:"opened_at"`
	LastSeenAt time.Time       `json:"last_seen_at"`
	ReviewedBy *string         `json:"reviewed_by"`
	ReviewedAt *time.Time      `json:"reviewed_at"`
	ReviewNote *string         `json:"review_note"`
}

// Finding is what a detector raises.
type Finding struct {
	Country     string
	Rule        string
	Severity    string
	SubjectKind string
	SubjectID   string
	TripID      string
	DedupeKey   string
	Evidence    any
}

// Filter narrows the review queue.
type Filter struct {
	Status      string
	Rule        string
	SubjectKind string
	SubjectID   string
	Country     string
	Limit       int
	Before      *time.Time // cursor: opened before this time
}

// Errors owned by this module.
var (
	ErrFlagNotFound   = apperr.New("FLAG_NOT_FOUND", http.StatusNotFound, "")
	ErrFlagReviewed   = apperr.New("FLAG_ALREADY_REVIEWED", http.StatusConflict, "")
	ErrReasonRequired = apperr.New("REASON_REQUIRED", http.StatusBadRequest, "")
	ErrDecision       = apperr.New("VALIDATION_FAILED", http.StatusBadRequest, "")
)
