package profiles

import (
	"context"

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

// Geofences finds the districts, zones and places at a point or along a route
// (in memory), and checks a geofence for assignments.
type Geofences interface {
	Contains(ctx context.Context, p geofences.Point) ([]geofences.Hit, error)
	Crossed(ctx context.Context, route []geofences.Point) ([]geofences.Hit, error)
	Info(ctx context.Context, id string) (country, name, kind string, ok bool, err error)
	Overlaps(ctx context.Context, country, onlyID string) ([]geofences.Overlap, error)
}
