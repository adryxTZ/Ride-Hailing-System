package branding

import (
	"net/http"
	"time"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/theme"
)

// Identity is who the brand is to riders and drivers.
type Identity struct {
	Name           string     `json:"name"`
	LegalEntity    string     `json:"legal_entity"`
	SupportPhone   string     `json:"support_phone"`
	SupportEmail   string     `json:"support_email"`
	EmergencyPhone string     `json:"emergency_phone"`
	EditedAt       *time.Time `json:"edited_at"`
}

// IdentityInput is an identity change with its reason (audited).
type IdentityInput struct {
	Name           string `json:"name"`
	LegalEntity    string `json:"legal_entity"`
	SupportPhone   string `json:"support_phone"`
	SupportEmail   string `json:"support_email"`
	EmergencyPhone string `json:"emergency_phone"`
	Reason         string `json:"reason"`
}

// Theme statuses.
const (
	StatusDraft     = "DRAFT"
	StatusPublished = "PUBLISHED"
	StatusArchived  = "ARCHIVED"
)

// ThemeVersion is one version of the brand's theme.
type ThemeVersion struct {
	ID          string            `json:"id"`
	Version     int               `json:"version"`
	Status      string            `json:"status"`
	Name        string            `json:"name"`
	Source      string            `json:"source"` // PRESET:<key> | IMPORTED:<url> | CUSTOM
	Roles       Roles             `json:"roles"`
	Palette     theme.Palette     `json:"palette"`
	Dark        *theme.Palette    `json:"dark"`
	Typography  map[string]string `json:"typography"`
	Shape       map[string]int    `json:"shape"`
	Assets      map[string]string `json:"assets"`     // slot → storage key
	AssetURLs   map[string]string `json:"asset_urls"` // slot → public path (filled on read)
	Report      theme.Report      `json:"report"`
	Reason      *string           `json:"change_reason"`
	CreatedBy   *string           `json:"created_by"`
	CreatedAt   time.Time         `json:"created_at"`
	PublishedAt *time.Time        `json:"published_at"`
	PublishedBy *string           `json:"published_by"`
}

// Roles are the colours staff chose, before derivation, for light and dark.
type Roles struct {
	Light map[string]string `json:"light"`
	Dark  map[string]string `json:"dark"`
}

// DraftInput creates or replaces a draft.
type DraftInput struct {
	Name       string            `json:"name"`
	Source     string            `json:"source"`
	Roles      Roles             `json:"roles"`
	Typography map[string]string `json:"typography"`
	Shape      map[string]int    `json:"shape"`
	Assets     map[string]string `json:"assets"`
}

// Preview is a palette derived from roles, with its report (nothing saved).
type Preview struct {
	Palette theme.Palette `json:"palette"`
	Dark    theme.Palette `json:"dark"`
	Report  theme.Report  `json:"report"`
}

// Allow-lists and bounds (fonts are bundled in the apps; only these can be chosen).
var (
	Fonts      = []string{"Figtree", "Plus Jakarta Sans", "Inter", "Outfit", "Poppins", "Noto Sans"}
	FontKeys   = []string{"heading_font", "body_font"}
	RoleKeys   = []string{"primary", "secondary", "accent", "background", "surface", "text", "muted", "border", "success", "warning", "danger", "info", "highlight"}
	AssetSlots = []string{"logo_full", "logo_mark", "logo_on_dark", "app_icon", "splash", "favicon"}
	// ShapeMax bounds each corner radius (px).
	ShapeMax = map[string]int{"radius_sm": 16, "radius_md": 24, "radius_lg": 32}
)

// Limits.
const (
	NameMaxLen     = 60
	MaxAssetBytes  = 2 << 20
	assetKeyPrefix = "branding/"
)

// Error codes owned by this module.
var (
	ErrThemeNotFound   = apperr.New("THEME_NOT_FOUND", http.StatusNotFound, "")
	ErrNotDraft        = apperr.New("THEME_NOT_DRAFT", http.StatusConflict, "")
	ErrAlreadyLive     = apperr.New("THEME_ALREADY_PUBLISHED", http.StatusConflict, "")
	ErrContrastFails   = apperr.New("THEME_CONTRAST_FAILS", http.StatusUnprocessableEntity, "")
	ErrReasonRequired  = apperr.New("REASON_REQUIRED", http.StatusBadRequest, "")
	ErrAssetMissing    = apperr.New("ASSET_MISSING", http.StatusBadRequest, "")
	ErrAssetType       = apperr.New("ASSET_TYPE_NOT_ALLOWED", http.StatusBadRequest, "")
	ErrNotColorHuntURL = apperr.New("NOT_A_COLORHUNT_LINK", http.StatusBadRequest, "")
)
