package profiles

import (
	"net/http"
	"time"

	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
)

// Profile statuses.
const (
	StatusDraft    = "DRAFT"
	StatusActive   = "ACTIVE"
	StatusInactive = "INACTIVE"
)

// Scope kinds, in increasing precedence (a place beats a zone beats … the country).
const (
	ScopeCountry  = "COUNTRY"
	ScopeArea     = "AREA"
	ScopeDistrict = "DISTRICT"
	ScopeZone     = "ZONE"
	ScopePlace    = "PLACE"
)

// Precedence of each scope kind when several profiles cover a point.
var Precedence = map[string]int{ScopePlace: 400, ScopeZone: 300, ScopeDistrict: 200, ScopeArea: 100, ScopeCountry: 0}

// Bounds.
const (
	MaxChargesPerVersion  = 20
	MaxProfilesPerCountry = 200
	EffectiveFromMaxAhead = 90 * 24 * time.Hour
	EffectiveFromSkew     = 5 * time.Minute
	ResolverVersion       = 1
	NameMaxLen            = 80
	DescriptionMaxLen     = 500
	PriorityMin           = -1000
	PriorityMax           = 1000
	listVersionsLimit     = 50
	listEventsLimit       = 100
)

// Profile is a pricing profile with what staff see about it.
type Profile struct {
	ID          string     `json:"id"`
	Country     string     `json:"country"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	IsDefault   bool       `json:"is_default"`
	Status      string     `json:"status"`
	ActiveFrom  *time.Time `json:"active_from"`
	ActiveUntil *time.Time `json:"active_until"`
	// CurrentVersion is the version in force now; NextVersion one dated later, if any.
	CurrentVersion *VersionRef `json:"current_version"`
	NextVersion    *VersionRef `json:"next_version"`
	ChargesCount   int         `json:"charges_count"`
	Scopes         []Scope     `json:"scopes"` // open assignments
	UpdatedAt      time.Time   `json:"updated_at"`
	// Detail (Get only).
	Versions    []Version `json:"versions,omitempty"`
	Assignments []Scope   `json:"assignments,omitempty"`
	Events      []Event   `json:"events,omitempty"`
}

// VersionRef names a version.
type VersionRef struct {
	ID            string    `json:"id"`
	Version       int       `json:"version"`
	EffectiveFrom time.Time `json:"effective_from"`
}

// Version is an immutable set of rules and charges.
type Version struct {
	ID            string       `json:"id"`
	ProfileID     string       `json:"profile_id"`
	Version       int          `json:"version"`
	EffectiveFrom time.Time    `json:"effective_from"`
	Terms         Terms        `json:"terms"`
	Charges       []ChargeRule `json:"charges"`
	ContentHash   string       `json:"content_hash"`
	PublishedBy   *string      `json:"published_by"`
	Reason        string       `json:"reason"`
	CreatedAt     time.Time    `json:"created_at"`
}

// Scope is where a profile applies (an assignment), open or closed.
type Scope struct {
	AssignmentID string     `json:"assignment_id"`
	ProfileID    string     `json:"profile_id"`
	Kind         string     `json:"kind"`
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	Priority     int        `json:"priority"`
	ValidFrom    time.Time  `json:"valid_from"`
	ValidTo      *time.Time `json:"valid_to"`
	Reason       string     `json:"reason"`
}

// Event is one entry of a profile's history.
type Event struct {
	At           time.Time      `json:"at"`
	Kind         string         `json:"kind"`
	Version      *int           `json:"version"`
	AssignmentID *string        `json:"assignment_id"`
	ActorID      *string        `json:"actor_id"`
	ActorName    *string        `json:"actor_name"`
	Reason       *string        `json:"reason"`
	Data         map[string]any `json:"data"`
}

// Event kinds.
const (
	EventCreated         = "CREATED"
	EventPublished       = "PUBLISHED"
	EventActivated       = "ACTIVATED"
	EventDeactivated     = "DEACTIVATED"
	EventAssigned        = "ASSIGNED"
	EventUnassigned      = "UNASSIGNED"
	EventMadeDefault     = "MADE_DEFAULT"
	EventDefaultReplaced = "DEFAULT_REPLACED"
	EventRenamed         = "RENAMED"
	EventWindowChanged   = "WINDOW_CHANGED"
)

// Payee is a third party a charge is owed to.
type Payee struct {
	ID        string `json:"id"`
	Country   string `json:"country"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Reference string `json:"reference"`
	Status    string `json:"status"`
}

// PayeeKinds are the accepted payee kinds.
var PayeeKinds = []string{"TAX_AUTHORITY", "REGULATOR", "AIRPORT", "MUNICIPALITY", "PARKING_OPERATOR", "OTHER"}

// AppliedCharge is a charge that matched a trip, with its amount.
type AppliedCharge struct {
	Code        string            `json:"code"`
	Label       map[string]string `json:"label"`
	Category    string            `json:"category"`
	Recipient   string            `json:"recipient"`
	PayeeID     string            `json:"payee_id,omitempty"`
	PayeeName   string            `json:"payee_name,omitempty"`
	Taxable     bool              `json:"taxable"`
	AppliesWhen string            `json:"applies_when"`
	MatchedAt   string            `json:"matched_at"` // PICKUP | DROPOFF | ROUTE
	ProfileID   string            `json:"profile_id"`
	Version     int               `json:"version"`
	Amount      decimal.Decimal   `json:"amount"`
}

// Error codes owned by this module.
var (
	ErrProfileNotFound      = apperr.New("PROFILE_NOT_FOUND", http.StatusNotFound, "")
	ErrNameTaken            = apperr.New("PROFILE_NAME_TAKEN", http.StatusConflict, "")
	ErrLimitReached         = apperr.New("PROFILE_LIMIT_REACHED", http.StatusConflict, "")
	ErrWindowInvalid        = apperr.New("PROFILE_WINDOW_INVALID", http.StatusBadRequest, "")
	ErrIsDefault            = apperr.New("PROFILE_IS_DEFAULT", http.StatusConflict, "")
	ErrTie                  = apperr.New("PROFILE_PRECEDENCE_TIE", http.StatusConflict, "")
	ErrNotPublished         = apperr.New("PROFILE_NOT_PUBLISHED", http.StatusConflict, "")
	ErrStatusUnchanged      = apperr.New("PROFILE_STATUS_UNCHANGED", http.StatusConflict, "")
	ErrNotActive            = apperr.New("PROFILE_NOT_ACTIVE", http.StatusConflict, "")
	ErrIncomplete           = apperr.New("PROFILE_INCOMPLETE", http.StatusConflict, "")
	ErrHasWindow            = apperr.New("PROFILE_HAS_WINDOW", http.StatusConflict, "")
	ErrHasScopes            = apperr.New("PROFILE_HAS_SCOPES", http.StatusConflict, "")
	ErrCountryOnly          = apperr.New("POLICY_FIELD_COUNTRY_ONLY", http.StatusConflict, "")
	ErrChargeInvalid        = apperr.New("CHARGE_INVALID", http.StatusBadRequest, "")
	ErrChargeDuplicate      = apperr.New("CHARGE_CODE_DUPLICATE", http.StatusConflict, "")
	ErrTooManyCharges       = apperr.New("TOO_MANY_CHARGES", http.StatusBadRequest, "")
	ErrEffectiveFrom        = apperr.New("EFFECTIVE_FROM_INVALID", http.StatusBadRequest, "")
	ErrPayeeInvalid         = apperr.New("PAYEE_INVALID", http.StatusConflict, "")
	ErrConditionUnavailable = apperr.New("CHARGE_CONDITION_UNAVAILABLE", http.StatusConflict, "")
	ErrScopeNotFound        = apperr.New("SCOPE_NOT_FOUND", http.StatusNotFound, "")
	ErrScopeCountryMismatch = apperr.New("SCOPE_COUNTRY_MISMATCH", http.StatusConflict, "")
	ErrScopeAssigned        = apperr.New("SCOPE_ALREADY_ASSIGNED", http.StatusConflict, "")
	ErrScopeKindUnavailable = apperr.New("SCOPE_KIND_UNAVAILABLE", http.StatusBadRequest, "")
	ErrAssignmentClosed     = apperr.New("ASSIGNMENT_CLOSED", http.StatusConflict, "")
	ErrPayeeInUse           = apperr.New("PAYEE_IN_USE", http.StatusConflict, "")
	ErrPayeeNotFound        = apperr.New("PAYEE_NOT_FOUND", http.StatusNotFound, "")
	ErrRecordNotFound       = apperr.New("PRICING_RECORD_NOT_FOUND", http.StatusNotFound, "")
	ErrNoDefault            = apperr.New("PRICING_NOT_CONFIGURED", http.StatusConflict, "")
	ErrReasonRequired       = apperr.New("REASON_REQUIRED", http.StatusBadRequest, "")
)
