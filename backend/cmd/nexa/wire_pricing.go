package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/areas"
	"github.com/kalen-limited/hauler-rider/backend/internal/catalog"
	"github.com/kalen-limited/hauler-rider/backend/internal/geofences"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/httpx"
	"github.com/kalen-limited/hauler-rider/backend/internal/money"
	"github.com/kalen-limited/hauler-rider/backend/internal/pricing"
	"github.com/kalen-limited/hauler-rider/backend/internal/profiles"
	"github.com/kalen-limited/hauler-rider/backend/internal/providers/routing"
	"github.com/kalen-limited/hauler-rider/backend/internal/trips"
)

// areaInfo adapts the areas service to the profiles port (filled once areas start).
type areaInfo struct{ svc *areas.Service }

func (a *areaInfo) AreaInfo(ctx context.Context, areaID string) (string, string, bool, error) {
	if a.svc == nil {
		return "", "", false, nil
	}
	if _, err := uuid.Parse(areaID); err != nil {
		return "", "", false, nil
	}
	ar, err := a.svc.Get(ctx, areaID)
	if errors.Is(err, areas.ErrAreaNotFound) || ar == nil {
		return "", "", false, nil
	}
	if err != nil {
		return "", "", false, err
	}
	return ar.Country, ar.Name, true, nil
}

// newProfiles builds the pricing-profiles service and connects it to pricing
// (resolution, the seed) and to trips (pin at booking, split from the pin,
// seal at completion). Areas are connected later through the returned adapter.
func newProfiles(bs *bootstrap, brandID string, cat *catalog.Catalog, prices *pricing.Service, tripSvc *trips.Service) (*profiles.Service, *areaInfo) {
	areasPort := &areaInfo{}
	svc := profiles.New(bs.pool, brandID, cat, areasPort)
	prices.SetProfiles(svc, func(country string) *time.Location { return countryLocation(bs, country) })
	if tripSvc != nil {
		tripSvc.OnInsertedTx(svc.PinTx)
		tripSvc.SetFees(feeSplitter(svc))
		tripSvc.OnCompletedTx(finalizePricing(svc))
	}
	return svc, areasPort
}

// locations caches loaded timezones (looked up on every quote with charges).
var locations sync.Map

// countryLocation is a country's timezone (charges' day and time windows); an
// unknown zone falls back to UTC with a warning, once.
func countryLocation(bs *bootstrap, country string) *time.Location {
	name := countryTimezone(bs, country)
	if loc, ok := locations.Load(name); ok {
		return loc.(*time.Location)
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		slog.Warn("pricing: unknown timezone, charge time windows use UTC", "country", country, "timezone", name)
		loc = time.UTC
	}
	locations.Store(name, loc)
	return loc
}

// feeSplitter splits a trip's fare under the terms pinned on that trip — never
// the rules in force now. A trip booked before profiles existed is pinned once
// to its scope's current rules, marked FALLBACK.
func feeSplitter(p *profiles.Service) trips.FeeFunc {
	return func(ctx context.Context, tx pgx.Tx, tripID string, total, bookingFee decimal.Decimal, currency string) (json.RawMessage, error) {
		pin, ok, err := p.PinnedFor(ctx, tx, tripID)
		if err != nil {
			return nil, err
		}
		if !ok {
			if pin, err = pinFallback(ctx, tx, p, tripID); err != nil {
				return nil, err
			}
		}
		b, err := money.Split(total, bookingFee, currency, policyOf(pin), sharesOf(pin.Charges)...)
		if err != nil {
			return nil, err
		}
		return json.Marshal(b)
	}
}

func pinFallback(ctx context.Context, tx pgx.Tx, p *profiles.Service, tripID string) (*profiles.Pinned, error) {
	var country, areaID, areaName string
	if err := tx.QueryRow(ctx, `SELECT coalesce(t.country_code,''), coalesce(t.area_id::text,''), coalesce(a.name,'')
		FROM trips t LEFT JOIN areas a ON a.id=t.area_id WHERE t.id=$1`, tripID).Scan(&country, &areaID, &areaName); err != nil {
		return nil, err
	}
	slog.Warn("pricing: trip had no pinned record; pinned the rules in force", "trip", tripID)
	return p.PinFallbackTx(ctx, tx, tripID, country, areaID, areaName)
}

// policyOf is the pinned rules in the shape money.Split takes.
func policyOf(pin *profiles.Pinned) money.Policy {
	t := pin.Terms
	return money.Policy{Version: pin.Profile.Version, ProfileID: pin.Profile.ID, ProfileVersionID: pin.Profile.VersionID, DefaultVersionID: pin.Default.VersionID,
		CommissionRate: t.CommissionRate, VATRate: t.VATRate, VATBase: t.VATBase, LevyRate: t.LevyRate, LevyBorneBy: t.LevyBorneBy}
}

func sharesOf(applied []profiles.AppliedCharge) []money.ChargeShare {
	out := make([]money.ChargeShare, 0, len(applied))
	for _, c := range applied {
		out = append(out, money.ChargeShare{Code: c.Code, Amount: c.Amount, Recipient: c.Recipient, PayeeID: c.PayeeID, Taxable: c.Taxable})
	}
	return out
}

// finalizePricing seals a completed trip's pricing record with its final split, in the completing transaction.
func finalizePricing(p *profiles.Service) trips.CompletedTxHook {
	return func(ctx context.Context, tx pgx.Tx, t *trips.Trip) error {
		return p.FinalizeTx(ctx, tx, t.ID)
	}
}

// areaCountry adapts the areas index to the geofences port (which country a point is in).
type areaCountry struct{ svc *areas.Service }

func (a areaCountry) CountryAt(ctx context.Context, p geofences.Point) (string, bool, error) {
	ix, err := a.svc.Index(ctx)
	if err != nil {
		return "", false, err
	}
	ref, ok := ix.AreaOf(routing.Point{Lat: p.Lat, Lng: p.Lng})
	return ref.Country, ok, nil
}

// newGeofences builds districts, zones and places and connects them to areas and profiles.
func newGeofences(bs *bootstrap, brandID string, areaSvc *areas.Service, prof *profiles.Service) *geofences.Service {
	svc := geofences.New(bs.pool, brandID)
	svc.SetPorts(areaCountry{areaSvc}, prof)
	prof.SetGeofences(svc)
	return svc
}

// simulator adapts a saved-nothing quote to the admin simulator: each
// available class's fare is split under the terms the quote would pin, with
// the same money.Split that settles real trips.
type simulator struct{ pricing *pricing.Service }

func (s simulator) Simulate(ctx context.Context, req pricing.Request, opts pricing.PreviewOptions) (*httpx.Simulation, error) {
	q, err := s.pricing.Preview(ctx, req, opts)
	if err != nil {
		return nil, err
	}
	qp := q.Pricing()
	out := &httpx.Simulation{Country: q.Country, Area: q.Area, DistanceM: q.Route.DistanceM, DurationS: q.Route.DurationS, Polyline: q.Route.Polyline,
		Options: q.Options, Resolution: qp, Splits: map[string]money.Breakdown{}}
	for _, o := range q.Options {
		pin, ok := qp.PerClass[o.VehicleClass]
		if !o.Available || !ok {
			continue
		}
		pinned := &profiles.Pinned{Profile: qp.Profile, Default: qp.Default, Terms: pin.Terms, Charges: pin.Charges}
		b, err := money.Split(o.Fare, pin.Terms.BookingFee, o.Currency, policyOf(pinned), sharesOf(pin.Charges)...)
		if err != nil {
			return nil, err
		}
		out.Splits[o.VehicleClass] = b
	}
	return out, nil
}
