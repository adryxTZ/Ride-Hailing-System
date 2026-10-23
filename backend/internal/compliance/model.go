package compliance

import (
	"net/http"
	"time"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
)

// Submission kinds.
const (
	KindDriver      = "DRIVER"
	KindVehicle     = "VEHICLE"
	KindTrip        = "TRIP"
	KindDailyReport = "DAILY_REPORT"
)

// Submission statuses.
const (
	StatusPending  = "PENDING"
	StatusAccepted = "ACCEPTED"
	StatusSkipped  = "SKIPPED" // nothing to send (e.g. a zero-trip day)
)

// codeLocalInvalid is recorded when our own data fails the regulator's rules before sending.
const codeLocalInvalid = "LOCAL_INVALID"

// Timing.
const (
	SweepInterval      = time.Minute
	SweepBatch         = 200
	ClaimLease         = 2 * time.Minute // a claimed row is not claimed again while it is being sent
	SendConcurrency    = 4
	StuckAfterAttempts = 5
	DailyInterval      = time.Hour
	// DailyReportAt is the local time after which yesterday's report is due.
	DailyReportAt = 30 * time.Minute
	gateTTL       = time.Minute // the fail-closed answer is cached per driver this long
	listLimit     = 200
)

// RetryLadder is the wait after each failed attempt; the last rung repeats.
var RetryLadder = []time.Duration{time.Minute, 5 * time.Minute, 30 * time.Minute, 3 * time.Hour, 24 * time.Hour}

// Submission is one thing the regulator must hear about.
type Submission struct {
	ID            string     `json:"id"`
	Country       string     `json:"country"`
	Regulator     string     `json:"regulator"`
	Kind          string     `json:"kind"`
	SubjectKey    string     `json:"subject_key"`
	SubjectLabel  string     `json:"subject_label"`
	DriverID      *string    `json:"driver_id,omitempty"`
	TripID        *string    `json:"trip_id,omitempty"`
	Status        string     `json:"status"`
	Attempts      int        `json:"attempts"`
	NextAttemptAt time.Time  `json:"next_attempt_at"`
	LastCode      *string    `json:"last_code,omitempty"`
	LastError     *string    `json:"last_error,omitempty"`
	Reference     *string    `json:"reference,omitempty"`
	AcceptedAt    *time.Time `json:"accepted_at,omitempty"`
	Stuck         bool       `json:"stuck"`
	CreatedAt     time.Time  `json:"created_at"`
}

// Settings are a country's switches.
type Settings struct {
	Enabled    bool `json:"enabled"`
	FailClosed bool `json:"fail_closed"`
}

// Status is a regulated country as the admin page shows it.
type Status struct {
	Country        string     `json:"country"`
	Regulator      string     `json:"regulator"`
	Adapter        string     `json:"adapter"` // fake | cets | pending
	Configured     bool       `json:"configured"`
	Enabled        bool       `json:"enabled"`
	FailClosed     bool       `json:"fail_closed"`
	Pending        int        `json:"pending"`
	Stuck          int        `json:"stuck"`
	AcceptedToday  int        `json:"accepted_today"`
	SkippedToday   int        `json:"skipped_today"`
	LastAcceptedAt *time.Time `json:"last_accepted_at,omitempty"`
	LastError      *string    `json:"last_error,omitempty"`
}

// Badge is a driver's or trip's standing with the regulator.
type Badge struct {
	Status    string  `json:"status"`
	Reference *string `json:"reference,omitempty"`
	LastError *string `json:"last_error,omitempty"`
}

// Filter narrows the submissions list.
type Filter struct {
	Country, Kind, Status, Q string
	Stuck                    bool
	Limit                    int
}

// Error codes owned by this module.
var (
	ErrNoRegulator      = apperr.New("REGULATOR_NONE", http.StatusConflict, "")
	ErrNotConfigured    = apperr.New("REGULATOR_NOT_CONFIGURED", http.StatusConflict, "")
	ErrNotPending       = apperr.New("SUBMISSION_NOT_PENDING", http.StatusConflict, "")
	ErrNotFound         = apperr.New("SUBMISSION_NOT_FOUND", http.StatusNotFound, "")
	ErrReasonRequired   = apperr.New("REASON_REQUIRED", http.StatusBadRequest, "")
	ErrRegistrationOpen = apperr.New("REGULATOR_REGISTRATION_PENDING", http.StatusConflict, "")
)
