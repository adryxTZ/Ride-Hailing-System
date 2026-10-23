package fleet

import (
	"net/http"
	"time"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
)

// Driver statuses.
const (
	StatusOnboarding    = "ONBOARDING"
	StatusPendingReview = "PENDING_REVIEW"
	StatusApproved      = "APPROVED"
	StatusRejected      = "REJECTED"
	StatusSuspended     = "SUSPENDED"
)

// Review statuses for vehicles and documents.
const (
	ReviewPending  = "PENDING"
	ReviewApproved = "APPROVED"
	ReviewRejected = "REJECTED"
)

// Document owners.
const (
	OwnerDriver  = "DRIVER"
	OwnerVehicle = "VEHICLE"
)

// Profile is a driver's identity and licensing as the Platform knows it.
type Profile struct {
	UserID        string     `json:"user_id"`
	FullName      *string    `json:"full_name,omitempty"`
	Phone         string     `json:"phone"`
	Status        string     `json:"status"`
	LicenceNumber *string    `json:"licence_number,omitempty"`
	LicenceExpiry *string    `json:"licence_expiry,omitempty"` // YYYY-MM-DD
	NationalID    *string    `json:"national_id,omitempty"`
	StatusReason  *string    `json:"status_reason,omitempty"`
	SubmittedAt   *time.Time `json:"submitted_at,omitempty"`
	ReviewedAt    *time.Time `json:"reviewed_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	LicenceClass  *string    `json:"licence_class,omitempty"`
	Country       *string    `json:"country,omitempty"`
}

// Vehicle is the car (or bajaji) a driver works with.
type Vehicle struct {
	ID           string  `json:"id"`
	DriverID     *string `json:"driver_id,omitempty"`
	ServiceKey   string  `json:"service"`
	ClassKey     string  `json:"vehicle_class"`
	Plate        string  `json:"plate"`
	Make         *string `json:"make,omitempty"`
	Model        *string `json:"model,omitempty"`
	Colour       *string `json:"colour,omitempty"`
	Year         *int    `json:"year,omitempty"`
	Status       string  `json:"status"`
	StatusReason *string `json:"status_reason,omitempty"`
}

// Document is one uploaded proof (licence, registration, insurance …).
type Document struct {
	ID           string     `json:"id"`
	OwnerKind    string     `json:"owner"`
	OwnerID      string     `json:"owner_id"`
	Kind         string     `json:"kind"`
	FileKey      string     `json:"-"`
	ContentType  string     `json:"content_type"`
	Status       string     `json:"status"`
	StatusReason *string    `json:"status_reason,omitempty"`
	ExpiresOn    *string    `json:"expires_on,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	ReviewedAt   *time.Time `json:"reviewed_at,omitempty"`
	// SizeBytes is the stored file's size (nil for files uploaded before it was recorded).
	SizeBytes *int64 `json:"size_bytes,omitempty"`
	// URL is a short-lived link to view the file (filled on read).
	URL string `json:"url,omitempty"`
	// Expiry is where the document stands (filled on read for the driver's country).
	ExpiryState string `json:"expiry_state,omitempty"`
	DaysLeft    *int   `json:"days_left,omitempty"`
	// Renewal is a newer upload waiting for review (or rejected) beside this one.
	Renewal   *Document `json:"renewal,omitempty"`
	renewalOf *string
}

// Requirement is one document the driver must provide.
type Requirement struct {
	Kind  string            `json:"kind"`
	Owner string            `json:"owner"`
	Label map[string]string `json:"label"`
	// Done is true once a document of this kind is uploaded (not rejected).
	Done bool `json:"done"`
	// Mandatory documents block sign-up; optional ones are asked for but never block.
	Mandatory      bool `json:"mandatory"`
	TracksExpiry   bool `json:"tracks_expiry"`
	ExpiryRequired bool `json:"expiry_required"`
}

// Onboarding is everything the driver app shows on the sign-up checklist.
type Onboarding struct {
	Profile      Profile       `json:"profile"`
	Vehicle      *Vehicle      `json:"vehicle,omitempty"`
	Documents    []Document    `json:"documents"`
	Requirements []Requirement `json:"requirements"`
	// Missing lists what blocks submission: "profile", "vehicle", or document kinds.
	Missing   []string `json:"missing"`
	CanSubmit bool     `json:"can_submit"`
	CanEdit   bool     `json:"can_edit"`
	// The driver's country rules for the licence form (empty = anything goes).
	LicenceClasses []string `json:"licence_classes"`
	LicencePattern string   `json:"licence_pattern,omitempty"`
}

// ProfileInput is what the driver fills in about themselves.
type ProfileInput struct {
	FullName      string `json:"full_name"`
	LicenceNumber string `json:"licence_number"`
	LicenceExpiry string `json:"licence_expiry"`
	NationalID    string `json:"national_id"`
	LicenceClass  string `json:"licence_class"` // one of the country's licence classes
	AreaID        string `json:"area_id"`       // the operating area the driver will work in
}

// VehicleInput is what the driver fills in about the vehicle.
type VehicleInput struct {
	ServiceKey string  `json:"service"`
	ClassKey   string  `json:"vehicle_class"`
	Plate      string  `json:"plate"`
	Make       *string `json:"make"`
	Model      *string `json:"model"`
	Colour     *string `json:"colour"`
	Year       *int    `json:"year"`
}

// DocumentInput attaches an uploaded file as a document.
type DocumentInput struct {
	Kind        string  `json:"kind"`
	FileKey     string  `json:"file_key"`
	ContentType string  `json:"content_type"`
	ExpiresOn   *string `json:"expires_on"`
	// sizeBytes is measured from the stored file, never taken from the client.
	sizeBytes int64
}

// UploadTicket is where the app PUTs a file before attaching it.
type UploadTicket struct {
	FileKey   string    `json:"file_key"`
	UploadURL string    `json:"upload_url"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Eligibility is what an approved driver may go online with.
type Eligibility struct {
	VehicleID string   `json:"vehicle_id"`
	Services  []string `json:"services"`
	Classes   []string `json:"vehicle_classes"`
	Plate     string   `json:"plate"`
	Areas     []string `json:"areas"` // operating area ids the driver is registered in
}

var (
	ErrNotEditable        = apperr.New("ONBOARDING_LOCKED", http.StatusConflict, "")
	ErrIncomplete         = apperr.New("ONBOARDING_INCOMPLETE", http.StatusConflict, "")
	ErrPlateTaken         = apperr.New("PLATE_TAKEN", http.StatusConflict, "")
	ErrUnknownClass       = apperr.New("UNKNOWN_VEHICLE_CLASS", http.StatusBadRequest, "")
	ErrUploadMissing      = apperr.New("UPLOAD_NOT_FOUND", http.StatusBadRequest, "")
	ErrUnknownDocument    = apperr.New("UNKNOWN_DOCUMENT_KIND", http.StatusBadRequest, "")
	ErrDriverNotApproved  = apperr.New("DRIVER_NOT_APPROVED", http.StatusForbidden, "")
	ErrVehicleNotApproved = apperr.New("VEHICLE_NOT_APPROVED", http.StatusForbidden, "")
	ErrDriverNotFound     = apperr.New("DRIVER_NOT_FOUND", http.StatusNotFound, "")
	ErrReasonRequired     = apperr.New("REASON_REQUIRED", http.StatusBadRequest, "")
)
