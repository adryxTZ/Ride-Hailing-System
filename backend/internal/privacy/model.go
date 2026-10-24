package privacy

import (
	"net/http"
	"time"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
)

// Apps (consent is per app).
const (
	AppRider  = "RIDER"
	AppDriver = "DRIVER"
)

// Request kinds and statuses.
const (
	KindExport = "EXPORT"
	KindDelete = "DELETE"

	StatusRequested  = "REQUESTED"
	StatusInProgress = "IN_PROGRESS"
	StatusReady      = "READY"
	StatusDone       = "DONE"
	StatusRejected   = "REJECTED"
	StatusCancelled  = "CANCELLED"
)

// Deletion blockers.
const (
	BlockActiveTrip    = "ACTIVE_TRIP"
	BlockDriverDebt    = "DRIVER_DEBT"
	BlockDriverBalance = "DRIVER_BALANCE"
	BlockOpenPayout    = "OPEN_PAYOUT"
	BlockRiderDues     = "RIDER_DUES"
	BlockOwedToRider   = "OWED_TO_RIDER"
)

// Settings.
const (
	ConsentCacheTTL     = 24 * time.Hour
	ExportLinkTTL       = 24 * time.Hour
	DownloadURLTTL      = 15 * time.Minute
	RequestsPerDay      = 3
	ExportMaxBytes      = 50 << 20
	RetentionHour       = 2 // local time the daily retention may run from
	RetentionBatch      = 1000
	RetentionCheckEvery = time.Hour
	reasonMaxLen        = 1000
	listLimitDefault    = 50
	listLimitMax        = 200
	removedMessage      = "[removed]"
)

// ConsentDocument is the notice an app shows (texts in the caller's language).
type ConsentDocument struct {
	App         string    `json:"app"`
	Version     int       `json:"version"`
	Title       string    `json:"title"`
	Summary     string    `json:"summary"`
	URL         string    `json:"url"`
	PublishedAt time.Time `json:"published_at"`
}

// ConsentStatus is where a person stands with an app's notice.
type ConsentStatus struct {
	App             string `json:"app"`
	CurrentVersion  int    `json:"current_version"`
	AcceptedVersion *int   `json:"accepted_version"`
	Required        bool   `json:"required"`
}

// AcceptInput is where an acceptance came from (kept as evidence).
type AcceptInput struct {
	App, InstallID, IP, UserAgent string
	Version                       int
}

// Request is a privacy request as the person and staff see it.
type Request struct {
	ID                string     `json:"id"`
	UserID            string     `json:"user_id,omitempty"`
	UserName          *string    `json:"user_name,omitempty"`
	UserPhone         string     `json:"user_phone,omitempty"`
	App               string     `json:"app"`
	Kind              string     `json:"kind"`
	Status            string     `json:"status"`
	Reason            *string    `json:"reason"`
	CreatedAt         time.Time  `json:"created_at"`
	DueAt             time.Time  `json:"due_at"`
	HandledBy         *string    `json:"handled_by,omitempty"`
	HandledAt         *time.Time `json:"handled_at,omitempty"`
	DownloadExpiresAt *time.Time `json:"download_expires_at"`
	Blockers          []string   `json:"blockers"`
	Overdue           bool       `json:"overdue"`
	Country           *string    `json:"country"`
}

// Filter narrows the staff queue.
type Filter struct {
	Status, Kind, Country string // Country "" = every country
	Limit                 int
}

// RetentionStatus is one rule and its last run.
type RetentionStatus struct {
	Name      string     `json:"name"`
	Days      int        `json:"days"`
	LastRunAt *time.Time `json:"last_run_at"`
	RowsLast  int        `json:"rows_last_run"`
	Error     *string    `json:"error"`
}

// Errors owned by this module.
var (
	ErrConsentRequired = apperr.New("CONSENT_REQUIRED", http.StatusForbidden, "")
	ErrConsentStale    = apperr.New("CONSENT_VERSION_STALE", http.StatusConflict, "")
	ErrRequestOpen     = apperr.New("PRIVACY_REQUEST_OPEN", http.StatusConflict, "")
	ErrRateLimited     = apperr.New("PRIVACY_RATE_LIMITED", http.StatusTooManyRequests, "")
	ErrNotOpen         = apperr.New("PRIVACY_REQUEST_NOT_OPEN", http.StatusConflict, "")
	ErrNotFound        = apperr.New("PRIVACY_REQUEST_NOT_FOUND", http.StatusNotFound, "")
	ErrExportNotReady  = apperr.New("EXPORT_NOT_READY", http.StatusConflict, "")
	ErrExportExpired   = apperr.New("EXPORT_EXPIRED", http.StatusGone, "")
	ErrDeleteBlocked   = apperr.New("DELETE_BLOCKED", http.StatusConflict, "")
	ErrReasonRequired  = apperr.New("REASON_REQUIRED", http.StatusBadRequest, "")
	ErrInvalid         = apperr.New("VALIDATION_FAILED", http.StatusBadRequest, "")
)
