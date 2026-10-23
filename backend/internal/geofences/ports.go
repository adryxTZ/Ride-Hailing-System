package geofences

import "context"

// Areas tells which country's operating area a point is in (a geofence must not sit in another country).
type Areas interface {
	CountryAt(ctx context.Context, p Point) (country string, ok bool, err error)
}

// Profiles reports the pricing profile open on geofences (shown on the page; blocks archiving one in use).
type Profiles interface {
	OpenOnScopes(ctx context.Context, scopeIDs []string) (map[string]Assigned, error)
}
