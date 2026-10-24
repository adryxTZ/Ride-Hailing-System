package main

import (
	"github.com/kalen-limited/hauler-rider/backend/internal/dispatch"
	"github.com/kalen-limited/hauler-rider/backend/internal/presence"
)

// startTripQueue connects the trip queue: per-country dispatch settings for
// the brand's countries; a driver going offline (or swept as stale) releasing
// their queued next trip back to matching; a driver coming back online put
// back on the trip they are still on.
func startTripQueue(bs *bootstrap, brandID string, d *dispatch.Service, p *presence.Service) {
	if d == nil || p == nil {
		return
	}
	codes := make([]string, 0, len(bs.brand.Countries))
	for _, c := range bs.brand.Countries {
		codes = append(codes, c.Code)
	}
	d.UseSettings(dispatch.NewSettings(bs.pool, brandID, codes))
	p.OnOffline(d.DriverOffline)
	p.OnOnline(d.DriverOnline)
}
