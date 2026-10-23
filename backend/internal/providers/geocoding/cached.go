package geocoding

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/kalen-limited/hauler-rider/backend/internal/providers/routing"
)

// Cached keeps search results (per query and ~5 km area) and reverse names
// (per ~11 m point) in Redis, so the provider sees each question once a day.
type Cached struct {
	Inner  Provider
	Redis  *redis.Client
	TTL    time.Duration
	Prefix string
}

func (c Cached) Search(ctx context.Context, q string, near *routing.Point, limit int) ([]Place, error) {
	area := "-"
	if near != nil {
		area = fmt.Sprintf("%.2f,%.2f", near.Lat, near.Lng) // ~1 km grid: nearby users share results
	}
	key := fmt.Sprintf("%s:s:%s:%s:%d", c.Prefix, area, strings.ToLower(strings.TrimSpace(q)), limit)
	var out []Place
	if c.load(ctx, key, &out) {
		return out, nil
	}
	out, err := c.Inner.Search(ctx, q, near, limit)
	if err == nil {
		c.save(ctx, key, out)
	}
	return out, err
}

func (c Cached) Reverse(ctx context.Context, at routing.Point) (*Place, error) {
	key := fmt.Sprintf("%s:r:%.4f,%.4f", c.Prefix, at.Lat, at.Lng)
	var out *Place
	if c.load(ctx, key, &out) {
		return out, nil
	}
	out, err := c.Inner.Reverse(ctx, at)
	if err == nil {
		c.save(ctx, key, out)
	}
	return out, err
}

func (c Cached) load(ctx context.Context, key string, into any) bool {
	if c.Redis == nil {
		return false
	}
	raw, err := c.Redis.Get(ctx, key).Bytes()
	return err == nil && json.Unmarshal(raw, into) == nil
}

func (c Cached) save(ctx context.Context, key string, v any) {
	if c.Redis == nil {
		return
	}
	if raw, err := json.Marshal(v); err == nil {
		c.Redis.Set(ctx, key, raw, c.TTL)
	}
}
