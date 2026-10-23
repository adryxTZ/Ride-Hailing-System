package main

import (
	"context"
	"log/slog"

	"github.com/kalen-limited/hauler-rider/backend/internal/dispatch"
	"github.com/kalen-limited/hauler-rider/backend/internal/presence"
	"github.com/kalen-limited/hauler-rider/backend/internal/pricing"
	"github.com/kalen-limited/hauler-rider/backend/internal/providers/routing"
	"github.com/kalen-limited/hauler-rider/backend/internal/surge"
	"github.com/kalen-limited/hauler-rider/backend/internal/trips"
)

// Free drivers are counted within this many H3 res-9 rings (~2 km) of an area's centre.
const (
	surgeSupplyRings = 6
	surgeSupplyLimit = 50
	surgeService     = "taxi"
)

// startSurge recomputes busy-area multipliers on the dispatcher leader and
// feeds them to quotes. Without Redis or dispatch there is no surge.
func startSurge(bs *bootstrap, t *trips.Service, p *presence.Service, d *dispatch.Service, prices *pricing.Service) *surge.Service {
	if bs.redis == nil || p == nil || d == nil {
		return nil
	}
	supply := func(ctx context.Context, at routing.Point) (int, error) {
		drivers, err := p.Nearby(ctx, at, surgeSupplyRings, surgeService, "", "", surgeSupplyLimit)
		n := 0
		for _, dr := range drivers {
			if dr.State == presence.Idle {
				n++
			}
		}
		return n, err
	}
	sv := surge.New(bs.redis, bs.brand.Code, t.DemandPoints, supply)
	d.Every30s(func(ctx context.Context) {
		if _, err := sv.Recompute(ctx); err != nil {
			slog.Warn("surge recompute failed", "err", err)
		}
	})
	prices.SetSurge(sv.At)
	return sv
}
