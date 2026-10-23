package surge

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"github.com/uber/h3-go/v4"

	"github.com/kalen-limited/hauler-rider/backend/internal/providers/routing"
)

// Resolution is the H3 resolution surge areas use (~5 km² hexagons).
const Resolution = 7

// RecomputeInterval is how often the dispatcher leader recomputes surge.
const RecomputeInterval = 30 * time.Second

// mapTTL lets surge fade out if recomputation stops.
const mapTTL = 2 * time.Minute

// Demand returns the pickups of requests waiting for a driver.
type Demand func(ctx context.Context) ([]routing.Point, error)

// Supply counts free drivers around a point.
type Supply func(ctx context.Context, at routing.Point) (int, error)

// Service computes and serves area multipliers.
type Service struct {
	r      *redis.Client
	brand  string
	demand Demand
	supply Supply
}

// New wires the surge service.
func New(r *redis.Client, brand string, d Demand, s Supply) *Service {
	return &Service{r: r, brand: brand, demand: d, supply: s}
}

func (s *Service) key() string { return "surge:" + s.brand }

// Recompute rebuilds the area → multiplier map from current demand and supply.
// The new map replaces the old one atomically.
func (s *Service) Recompute(ctx context.Context) (int, error) {
	points, err := s.demand(ctx)
	if err != nil {
		return 0, err
	}
	byCell := map[h3.Cell]int{}
	for _, p := range points {
		if c, err := h3.LatLngToCell(h3.LatLng{Lat: p.Lat, Lng: p.Lng}, Resolution); err == nil {
			byCell[c]++
		}
	}
	fields := map[string]any{}
	for cell, demand := range byCell {
		if demand < MinDemand {
			continue
		}
		centre, err := cell.LatLng()
		if err != nil {
			continue
		}
		supply, err := s.supply(ctx, routing.Point{Lat: centre.Lat, Lng: centre.Lng})
		if err != nil {
			return 0, err
		}
		if m := multiplierFor(demand, supply); m.GreaterThan(decimal.NewFromInt(1)) {
			fields[cell.String()] = m.String()
		}
	}
	return len(fields), s.replace(ctx, fields)
}

func (s *Service) replace(ctx context.Context, fields map[string]any) error {
	if len(fields) == 0 {
		return s.r.Del(ctx, s.key()).Err()
	}
	tmp := s.key() + ":next"
	pipe := s.r.TxPipeline()
	pipe.Del(ctx, tmp)
	pipe.HSet(ctx, tmp, fields)
	pipe.Rename(ctx, tmp, s.key())
	pipe.Expire(ctx, s.key(), mapTTL)
	_, err := pipe.Exec(ctx)
	return err
}

// At is the multiplier for a pickup point (1 when the area is calm or unknown).
func (s *Service) At(ctx context.Context, p routing.Point) decimal.Decimal {
	one := decimal.NewFromInt(1)
	cell, err := h3.LatLngToCell(h3.LatLng{Lat: p.Lat, Lng: p.Lng}, Resolution)
	if err != nil {
		return one
	}
	v, err := s.r.HGet(ctx, s.key(), cell.String()).Result()
	if err != nil {
		return one
	}
	m, err := decimal.NewFromString(v)
	if err != nil || m.LessThan(one) {
		return one
	}
	return m
}

// Areas returns the current surging areas (H3 cell → multiplier) for the live map.
func (s *Service) Areas(ctx context.Context) (map[string]string, error) {
	return s.r.HGetAll(ctx, s.key()).Result()
}
