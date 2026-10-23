package fleet

import (
	"context"
	"time"
)

// Classes answers whether a vehicle class exists for a service (the catalogue).
type Classes interface {
	ClassExists(ctx context.Context, serviceKey, classKey string) (bool, error)
	// ServicesForClass lists every active service a vehicle of this class may serve.
	ServicesForClass(ctx context.Context, classKey string) ([]string, error)
}

// Files is the object store fleet needs (storage.Store satisfies it).
type Files interface {
	UploadURL(ctx context.Context, key, contentType string, expires time.Duration) (string, error)
	DownloadURL(ctx context.Context, key string, expires time.Duration) (string, error)
	Size(ctx context.Context, key string) (int64, error)
}

// Offline takes a driver off the map (presence), used on suspension.
type Offline func(ctx context.Context, driverID string) error

// DriverAreas reads and sets the operating areas a driver works in (the areas module).
// Nil when areas are not wired: every driver is then eligible everywhere.
type DriverAreas interface {
	AreaIDs(ctx context.Context, driverID string) ([]string, error)
	SetSignupArea(ctx context.Context, driverID, areaID string) error
}
