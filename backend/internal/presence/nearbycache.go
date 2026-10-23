package presence

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/uber/h3-go/v4"

	"github.com/kalen-limited/hauler-rider/backend/internal/providers/routing"
)

// NearbyCache shares rider-facing "cars around me" results for a few seconds per
// small area (H3 res-8 cell, ~0.7 km²). Riders' maps poll every few seconds; riders
// close to each other see the same cars, so most polls are served from memory.
// Dispatch never uses this cache — matching always reads live presence.
type NearbyCache struct {
	p   *Service
	ttl time.Duration

	mu      sync.Mutex
	entries map[string]nearbyEntry
}

type nearbyEntry struct {
	at      time.Time
	drivers []Driver
}

const (
	nearbyCacheRes = 8
	nearbyCacheMax = 20000 // entries before a full reset (bounded memory)
)

func NewNearbyCache(p *Service, ttl time.Duration) *NearbyCache {
	return &NearbyCache{p: p, ttl: ttl, entries: map[string]nearbyEntry{}}
}

// Nearby is Service.Nearby with the per-area cache in front of it. The cell centre,
// not the caller's exact point, is searched so every rider in the cell shares it.
func (c *NearbyCache) Nearby(ctx context.Context, at routing.Point, k int, service, class string, limit int) ([]Driver, error) {
	cell, err := h3.LatLngToCell(h3.LatLng{Lat: at.Lat, Lng: at.Lng}, nearbyCacheRes)
	if err != nil {
		return nil, err
	}
	key := fmt.Sprintf("%s|%s|%s|%d|%d", cell.String(), service, class, k, limit)
	now := time.Now()
	c.mu.Lock()
	if e, ok := c.entries[key]; ok && now.Sub(e.at) < c.ttl {
		c.mu.Unlock()
		return e.drivers, nil
	}
	c.mu.Unlock()
	centre, err := cell.LatLng()
	if err != nil {
		return nil, err
	}
	drivers, err := c.p.Nearby(ctx, routing.Point{Lat: centre.Lat, Lng: centre.Lng}, k, service, class, "", limit)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	if len(c.entries) >= nearbyCacheMax {
		c.entries = map[string]nearbyEntry{}
	}
	c.entries[key] = nearbyEntry{at: now, drivers: drivers}
	c.mu.Unlock()
	return drivers, nil
}
