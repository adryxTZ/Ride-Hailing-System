// Package geocoding is the place-search port: find places by text near the user
// (autocomplete) and name a coordinate (reverse). Implementations: Nominatim
// (OpenStreetMap; development and low volume), Google Places later (needs a key).
// Apps never call a provider directly — the API proxies and caches it.
package geocoding

import (
	"context"

	"github.com/kalen-limited/hauler-rider/backend/internal/providers/routing"
)

// Place is one search result or the name of a coordinate.
type Place struct {
	Name     string        `json:"name"`              // short title: "Mlimani City"
	Address  string        `json:"address,omitempty"` // the rest: "Sam Nujoma Road, Ubungo"
	Location routing.Point `json:"location"`
	Provider string        `json:"provider"`
}

// Provider searches and names places.
type Provider interface {
	Search(ctx context.Context, q string, near *routing.Point, limit int) ([]Place, error)
	Reverse(ctx context.Context, at routing.Point) (*Place, error)
}
