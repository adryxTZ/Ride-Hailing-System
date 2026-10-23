package geofences

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
)

// Kinds.
const (
	KindDistrict = "DISTRICT"
	KindZone     = "ZONE"
	KindPlace    = "PLACE"
)

// Statuses.
const (
	StatusActive   = "ACTIVE"
	StatusInactive = "INACTIVE"
	StatusArchived = "ARCHIVED"
)

// Bounds.
const (
	IndexResolution   = 7
	MaxVertices       = 2000
	MinAreaM2         = 1_000
	PlaceRadiusMinM   = 50
	PlaceRadiusMaxM   = 5_000
	ImportMaxFeatures = 200
	ImportMaxBytes    = 2 << 20
	MaxRoutePoints    = 5_000
	// cellEdgeM is roughly a res-7 cell's edge, for sizing a place's cover.
	cellEdgeM     = 1_220
	versionsLimit = 50
)

// MaxAreaKm2 bounds a shape's area per kind.
var MaxAreaKm2 = map[string]float64{KindDistrict: 50_000, KindZone: 1_000}

// Point is a WGS84 coordinate.
type Point struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

// Ring is a closed loop of [lng, lat] pairs (GeoJSON order).
type Ring [][2]float64

// Polygon is an outer ring and its holes.
type Polygon []Ring

// Geometry is a GeoJSON Polygon or MultiPolygon, normalised to many polygons.
type Geometry struct {
	Type        string    `json:"type"` // always MultiPolygon on the way out
	Coordinates []Polygon `json:"coordinates"`
}

// Geofence is one shape as staff see it.
type Geofence struct {
	ID          string     `json:"id"`
	Country     string     `json:"country"`
	Kind        string     `json:"kind"`
	Code        string     `json:"code"`
	Name        string     `json:"name"`
	Status      string     `json:"status"`
	Version     int        `json:"version"`
	Geometry    *Geometry  `json:"geometry,omitempty"`
	Center      *Point     `json:"center,omitempty"`
	RadiusM     *int       `json:"radius_m,omitempty"`
	AreaM2      float64    `json:"area_m2"`
	VertexCount int        `json:"vertex_count"`
	BBox        [4]float64 `json:"bbox"` // west, south, east, north
	Profile     *Assigned  `json:"profile,omitempty"`
	UpdatedAt   time.Time  `json:"updated_at"`
	Versions    []Version  `json:"versions,omitempty"`
}

// Assigned names the pricing profile open on a geofence.
type Assigned struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

// Version is one saved shape.
type Version struct {
	Version   int       `json:"version"`
	Geometry  *Geometry `json:"geometry,omitempty"`
	Center    *Point    `json:"center,omitempty"`
	RadiusM   *int      `json:"radius_m,omitempty"`
	CreatedBy *string   `json:"created_by"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"created_at"`
}

// Hit is a geofence containing a point (or crossed by a route).
type Hit struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	Name    string `json:"name"`
	Version int    `json:"version"`
	Country string `json:"country"`
}

// ShapeInput is a polygon (districts, zones) or a centre and radius (places).
type ShapeInput struct {
	Geometry json.RawMessage `json:"geometry,omitempty"`
	Center   *Point          `json:"center,omitempty"`
	RadiusM  int             `json:"radius_m,omitempty"`
}

// Error codes owned by this module.
var (
	ErrNotFound      = apperr.New("GEOFENCE_NOT_FOUND", http.StatusNotFound, "")
	ErrInvalid       = apperr.New("GEOFENCE_INVALID", http.StatusBadRequest, "")
	ErrCodeTaken     = apperr.New("GEOFENCE_CODE_TAKEN", http.StatusConflict, "")
	ErrOtherCountry  = apperr.New("GEOFENCE_IN_OTHER_COUNTRY", http.StatusConflict, "")
	ErrArchived      = apperr.New("GEOFENCE_ARCHIVED", http.StatusConflict, "")
	ErrInUse         = apperr.New("GEOFENCE_IN_USE", http.StatusConflict, "")
	ErrImportInvalid = apperr.New("IMPORT_INVALID", http.StatusBadRequest, "")
	ErrReason        = apperr.New("REASON_REQUIRED", http.StatusBadRequest, "")
)

// invalid wraps a shape problem with its reason (SELF_INTERSECTION, TOO_LARGE …).
func invalid(reason, detail string) error {
	return ErrInvalid.WithDetails(map[string]any{"reason": reason, "detail": detail})
}
