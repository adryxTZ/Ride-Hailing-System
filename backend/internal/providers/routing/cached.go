package routing

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// Cached wraps a provider with a Redis cache keyed by coordinates rounded to
// ~11 m, and falls back to Fallback when the provider fails, so a routing
// outage degrades to estimates instead of failing quotes and ETAs.
type Cached struct {
	Inner    Provider
	Fallback Provider
	Redis    *redis.Client
	TTL      time.Duration
	Prefix   string
}

const coordPrecision = 4 // decimal places ≈ 11 m

func (c Cached) key(points []Point) string {
	var b strings.Builder
	b.WriteString(c.Prefix)
	for _, p := range points {
		fmt.Fprintf(&b, ":%.*f,%.*f", coordPrecision, p.Lat, coordPrecision, p.Lng)
	}
	return b.String()
}

func (c Cached) Route(ctx context.Context, from, to Point) (Route, error) {
	return c.RouteVia(ctx, []Point{from, to})
}

// RouteVia serves multi-stop routes from the same cache.
func (c Cached) RouteVia(ctx context.Context, points []Point) (Route, error) {
	key := c.key(points)
	if c.Redis != nil {
		if raw, err := c.Redis.Get(ctx, key).Bytes(); err == nil {
			var r Route
			if json.Unmarshal(raw, &r) == nil {
				return r, nil
			}
		}
	}
	r, err := Via(ctx, c.Inner, points)
	if err != nil {
		slog.Warn("routing provider failed; using fallback", "err", err)
		return Via(ctx, c.Fallback, points)
	}
	if c.Redis != nil {
		if raw, err := json.Marshal(r); err == nil {
			c.Redis.Set(ctx, key, raw, c.TTL)
		}
	}
	return r, nil
}
