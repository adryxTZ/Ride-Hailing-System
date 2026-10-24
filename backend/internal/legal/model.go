package legal

import (
	"net/http"
	"time"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
)

// Apps a document belongs to (the same values consent uses).
const (
	AppRider  = "RIDER"
	AppDriver = "DRIVER"
)

// Document kinds.
const (
	KindPrivacyNotice   = "PRIVACY_NOTICE"
	KindTerms           = "TERMS"
	KindDriverAgreement = "DRIVER_AGREEMENT"
)

// Version statuses: a draft is edited, a published version never changes and
// is archived when the next one goes live.
const (
	StatusDraft     = "DRAFT"
	StatusPublished = "PUBLISHED"
	StatusArchived  = "ARCHIVED"
)

// Where a version came from.
const (
	SourceStarter = "STARTER"
	SourceStaff   = "STAFF"
)

// Limits and settings.
const (
	TitleMaxLen   = 120
	SummaryMaxLen = 600
	BodyMaxBytes  = 200 << 10
	FieldMaxLen   = 300
	ReasonMaxLen  = 1000
	// starterReason is the change reason recorded on the starter pack's versions.
	starterReason = "Starter pack (template, to be reviewed by a lawyer)"
)

// Text is a value per locale ("en", "sw").
type Text map[string]string

// Document is one version of an app's legal document.
type Document struct {
	ID          string     `json:"id"`
	App         string     `json:"app"`
	Kind        string     `json:"kind"`
	Version     int        `json:"version"`
	Status      string     `json:"status"`
	Title       Text       `json:"title"`
	Summary     Text       `json:"summary"`
	Body        Text       `json:"body"`
	Source      string     `json:"source"`
	Reason      *string    `json:"change_reason"`
	CreatedBy   *string    `json:"created_by"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	PublishedBy *string    `json:"published_by"`
	PublishedAt *time.Time `json:"published_at"`
}

// VersionRef is a version as the overview lists it.
type VersionRef struct {
	ID          string     `json:"id"`
	Version     int        `json:"version"`
	Source      string     `json:"source"`
	UpdatedAt   time.Time  `json:"updated_at"`
	PublishedAt *time.Time `json:"published_at"`
}

// Overview is one app's document with its live version and pending draft.
type Overview struct {
	App     string      `json:"app"`
	Kind    string      `json:"kind"`
	Slug    string      `json:"slug"`
	Consent bool        `json:"consent"`
	Title   string      `json:"title"`
	Live    *VersionRef `json:"live"`
	Draft   *VersionRef `json:"draft"`
}

// DraftInput is what staff save in the editor.
type DraftInput struct {
	Title   Text `json:"title"`
	Summary Text `json:"summary"`
	Body    Text `json:"body"`
}

// Details are the placeholder values staff fill in on the Legal page.
type Details struct {
	Fields    map[string]string `json:"fields"`
	UpdatedBy *string           `json:"updated_by"`
	UpdatedAt *time.Time        `json:"updated_at"`
}

// DetailsInput is a change to the details with its reason (audited).
type DetailsInput struct {
	Fields map[string]string `json:"fields"`
	Reason string            `json:"reason"`
}

// Page is a document rendered for reading: Markdown to sanitised HTML, placeholders filled.
type Page struct {
	App           string    `json:"app"`
	Kind          string    `json:"kind"`
	Version       int       `json:"version"`
	Lang          string    `json:"lang"`
	Title         string    `json:"title"`
	HTML          string    `json:"html"`
	EffectiveDate string    `json:"effective_date"`
	PublishedAt   time.Time `json:"published_at"`
	Brand         string    `json:"brand"`
}

// Preview is a draft rendered in the editor, with what its placeholders lack.
type Preview struct {
	HTML    string   `json:"html"`
	Missing []string `json:"missing"`
	Unknown []string `json:"unknown"`
}

// Errors owned by this module.
var (
	ErrNotFound       = apperr.New("LEGAL_DOCUMENT_NOT_FOUND", http.StatusNotFound, "")
	ErrNoDraft        = apperr.New("LEGAL_NO_DRAFT", http.StatusConflict, "")
	ErrReasonRequired = apperr.New("REASON_REQUIRED", http.StatusBadRequest, "")
	ErrInvalid        = apperr.Validation
)
