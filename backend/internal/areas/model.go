package areas

import (
	"net/http"
	"time"

	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
)

// Resolution is the H3 resolution areas are stored at (~5 km² cells, ~1.2 km edges).
const Resolution = 7

// Limits on what one request or area may hold.
const (
	MaxCellsPerArea    = 200_000
	MaxCellsPerRequest = 50_000
	OverlapDetailLimit = 100
	MaxDriverAreas     = 5
	CacheTTL           = 5 * time.Second
)

// Area statuses.
const (
	StatusActive   = "ACTIVE"
	StatusDraft    = "DRAFT"
	StatusArchived = "ARCHIVED"
)

// Country is one country the brand operates in, as the admin sees it.
type Country struct {
	Code           string            `json:"code"`
	Name           map[string]string `json:"name"`
	Currency       string            `json:"currency"`
	Timezone       string            `json:"timezone"`
	AreaTerm       map[string]string `json:"area_term"`
	Regulator      string            `json:"regulator"`
	PaymentGateway string            `json:"payment_gateway"`
	EmergencyPhone string            `json:"emergency_phone"`
	Map            MapView           `json:"map"`
	Status         string            `json:"status"`
	AreaCount      int               `json:"area_count"`
	LicencePattern string            `json:"licence_pattern,omitempty"`
	LicenceClasses []string          `json:"licence_classes,omitempty"`
}

// MapView is where the admin map opens for a country.
type MapView struct {
	Lat  float64 `json:"lat"`
	Lng  float64 `json:"lng"`
	Zoom int     `json:"zoom"`
}

// Area is one marked region of a country.
type Area struct {
	ID           string           `json:"id"`
	Country      string           `json:"country"`
	Code         string           `json:"code"`
	Name         string           `json:"name"`
	Status       string           `json:"status"`
	SurgeEnabled bool             `json:"surge_enabled"`
	SurgeMax     *decimal.Decimal `json:"surge_max"`
	CellCount    int              `json:"cell_count"`
	Cells        []string         `json:"cells"` // compacted H3 cells (res ≤ 7)
	BBox         [4]float64       `json:"bbox"`  // west, south, east, north
	UpdatedAt    time.Time        `json:"updated_at"`
}

// AreaRef names an area on records that belong to it.
type AreaRef struct {
	ID      string `json:"id"`
	Code    string `json:"code"`
	Name    string `json:"name"`
	Country string `json:"country"`
}

// Error codes owned by this module.
var (
	ErrCountryUnknown       = apperr.New("COUNTRY_UNKNOWN", http.StatusBadRequest, "")
	ErrCountryNotActive     = apperr.New("COUNTRY_NOT_ACTIVE", http.StatusConflict, "")
	ErrAreaNotFound         = apperr.New("AREA_NOT_FOUND", http.StatusNotFound, "")
	ErrAreaCodeTaken        = apperr.New("AREA_CODE_TAKEN", http.StatusConflict, "")
	ErrAreaNotReady         = apperr.New("AREA_NOT_READY", http.StatusConflict, "")
	ErrAreaHasDrivers       = apperr.New("AREA_HAS_DRIVERS", http.StatusConflict, "")
	ErrAreaHasActiveTrips   = apperr.New("AREA_HAS_ACTIVE_TRIPS", http.StatusConflict, "")
	ErrAreaArchived         = apperr.New("AREA_ARCHIVED", http.StatusConflict, "")
	ErrCellInvalid          = apperr.New("AREA_CELL_INVALID", http.StatusBadRequest, "")
	ErrCellResolution       = apperr.New("AREA_CELL_RESOLUTION", http.StatusBadRequest, "")
	ErrAreaTooLarge         = apperr.New("AREA_TOO_LARGE", http.StatusBadRequest, "")
	ErrCellsOverlap         = apperr.New("AREA_CELLS_OVERLAP", http.StatusConflict, "")
	ErrAreaNotInCountry     = apperr.New("AREA_NOT_IN_COUNTRY", http.StatusBadRequest, "")
	ErrOutsideServiceArea   = apperr.New("OUTSIDE_SERVICE_AREA", http.StatusConflict, "")
	ErrCrossBorder          = apperr.New("CROSS_BORDER_NOT_SUPPORTED", http.StatusConflict, "")
	ErrDriverAreasCountry   = apperr.New("DRIVER_AREAS_ONE_COUNTRY", http.StatusBadRequest, "")
	ErrPrimaryNotInAreas    = apperr.New("PRIMARY_NOT_IN_AREAS", http.StatusBadRequest, "")
	ErrOutsideDriverAreas   = apperr.New("OUTSIDE_REGISTERED_AREA", http.StatusConflict, "")
	ErrAreaRequired         = apperr.New("AREA_REQUIRED", http.StatusBadRequest, "")
	ErrReasonRequired       = apperr.New("REASON_REQUIRED", http.StatusBadRequest, "")
	ErrDriverNotFound       = apperr.New("DRIVER_NOT_FOUND", http.StatusNotFound, "")
	ErrCountryDefaultNeeded = apperr.New("COUNTRY_DEFAULT_REQUIRED", http.StatusConflict, "")
)
