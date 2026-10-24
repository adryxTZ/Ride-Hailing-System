package profiles

import (
	"context"
	"time"

	"github.com/kalen-limited/hauler-rider/backend/internal/geofences"
)

// Catalog checks that service and vehicle-class keys a charge names exist.
type Catalog interface {
	ClassExists(ctx context.Context, serviceKey, classKey string) (bool, error)
	ServicesForClass(ctx context.Context, classKey string) ([]string, error)
}

// Areas reads an operating area's country and name (for assignments).
type Areas interface {
	AreaInfo(ctx context.Context, areaID string) (country, name string, ok bool, err error)
}

// Geofences gives the index of districts, zones and places (today's, cached, or
// as they were at a past time) and checks a geofence for assignments.
type Geofences interface {
	Index(ctx context.Context) (*geofences.Index, error)
	IndexAt(ctx context.Context, country string, at time.Time) (*geofences.Index, error)
	Info(ctx context.Context, id string) (country, name, kind string, ok bool, err error)
	Overlaps(ctx context.Context, country, onlyID string) ([]geofences.Overlap, error)
}
