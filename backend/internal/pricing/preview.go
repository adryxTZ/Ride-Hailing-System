package pricing

import (
	"context"
	"errors"
	"time"

	"github.com/kalen-limited/hauler-rider/backend/internal/geofences"
	"github.com/kalen-limited/hauler-rider/backend/internal/profiles"
	"github.com/kalen-limited/hauler-rider/backend/internal/providers/routing"
)

// PreviewOptions are what a simulated quote may change: when it is priced
// (within ±7 days; a past time uses the profile history) and a profile that
// takes part as if it were active.
type PreviewOptions struct {
	At               time.Time
	PreviewProfileID string
}

// Preview prices a trip like Quote but saves nothing (the admin simulator).
// Tariffs and fixed routes are today's; profiles follow opts.At.
func (s *Service) Preview(ctx context.Context, req Request, opts PreviewOptions) (*Quote, error) {
	if s.profiles == nil {
		return nil, ErrNoTariff
	}
	at, now := opts.At, s.now()
	if at.IsZero() {
		at = now
	}
	if at.Before(now.Add(-profiles.SimulateMaxOffset)) || at.After(now.Add(profiles.SimulateMaxOffset)) {
		return nil, profiles.ErrSimulateAtRange // before routing: no road lookup for a refused time
	}
	resolve := func(ctx context.Context, in profiles.Input) (*profiles.Resolution, error) {
		return s.profiles.Preview(ctx, in, opts.PreviewProfileID)
	}
	return s.price(ctx, req, resolve, at)
}

// Pricing is the profile resolution and per-class pins of a quote (staff only).
func (q *Quote) Pricing() profiles.QuotePricing { return q.pricing }

// AsOf answers which profile priced a point (and a drop-off) at a past time,
// from history. Regions are located with today's shapes; the result says when
// one has changed since.
func (s *Service) AsOf(ctx context.Context, country string, pickup routing.Point, dropoff *routing.Point, at time.Time) (*profiles.AsOfResult, error) {
	if s.profiles == nil {
		return nil, ErrNoTariff
	}
	in := profiles.Input{Country: country, At: at, Pickup: &geofences.Point{Lat: pickup.Lat, Lng: pickup.Lng}}
	in.AreaID, in.AreaName = s.areaIn(ctx, country, pickup)
	if dropoff != nil {
		in.Dropoff = &geofences.Point{Lat: dropoff.Lat, Lng: dropoff.Lng}
		in.DropAreaID, in.DropAreaName = s.areaIn(ctx, country, *dropoff)
	}
	res, err := s.profiles.AsOf(ctx, in)
	if errors.Is(err, profiles.ErrNoDefault) {
		return nil, ErrNoTariff
	}
	return res, err
}

// areaIn is the region at a point when it is in the given country.
func (s *Service) areaIn(ctx context.Context, country string, p routing.Point) (id, name string) {
	if s.areas == nil {
		return "", ""
	}
	if ref, ok, err := s.areas.AreaOf(ctx, p); err == nil && ok && ref.Country == country {
		return ref.ID, ref.Name
	}
	return "", ""
}
