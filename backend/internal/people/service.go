package people

import (
	"context"
	"slices"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/kalen-limited/hauler-rider/backend/internal/trips"
)

// Bounds for the money lists on the driver page.
const (
	walletEntriesLimit = 30
	payinsLimit        = 30
)

// Allowed reports whether the staff member may see a country; nil means
// every country (an unlimited staff member).
type Allowed func(country string) bool

// Service builds the rider and driver pages.
type Service struct{ d Deps }

// New wires the ports.
func New(d Deps) *Service { return &Service{d: d} }

// Rider returns the staff rider page, or ErrOutOfScope / identity.ErrPersonNotFound.
func (s *Service) Rider(ctx context.Context, riderID string, allowed Allowed) (*Rider, error) {
	countries, err := s.visible(ctx, trips.AsRider, riderID, "", allowed)
	if err != nil {
		return nil, err
	}
	r := &Rider{}
	if r.Account, err = s.d.Accounts.PersonAccount(ctx, riderID, trips.AsRider); err != nil {
		return nil, err
	}
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() (err error) { r.Trips, err = s.d.Trips.PersonStats(gctx, trips.AsRider, riderID); return })
	g.Go(func() (err error) { r.Ratings, err = s.d.Ratings.PersonRatings(gctx, riderID); return })
	g.Go(func() (err error) { r.SOS, err = s.d.Alerts.AlertsFor(gctx, riderID); return })
	g.Go(func() (err error) { r.Dues, err = s.d.Money.RiderDues(gctx, riderID); return })
	g.Go(func() (err error) { r.Places, err = s.places(gctx, trips.AsRider, riderID, first(countries)); return })
	return r, g.Wait()
}

// Driver returns the activity half of the staff driver page.
func (s *Service) Driver(ctx context.Context, driverID string, allowed Allowed) (*Driver, error) {
	home, err := s.d.DriverCountry(ctx, driverID)
	if err != nil {
		return nil, err
	}
	if _, err := s.visible(ctx, trips.AsDriver, driverID, home, allowed); err != nil {
		return nil, err
	}
	d := &Driver{}
	if d.Account, err = s.d.Accounts.PersonAccount(ctx, driverID, trips.AsDriver); err != nil {
		return nil, err
	}
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() (err error) { d.Trips, err = s.d.Trips.PersonStats(gctx, trips.AsDriver, driverID); return })
	if s.d.Offers != nil {
		g.Go(func() (err error) { d.Offers, err = s.d.Offers.OfferStats(gctx, driverID); return })
	}
	g.Go(func() (err error) { d.Ratings, err = s.d.Ratings.PersonRatings(gctx, driverID); return })
	g.Go(func() (err error) { d.SOS, err = s.d.Alerts.AlertsFor(gctx, driverID); return })
	g.Go(func() (err error) { d.Places, err = s.places(gctx, trips.AsDriver, driverID, home); return })
	g.Go(func() (err error) { d.Money, err = s.driverMoney(gctx, driverID); return })
	if err := g.Wait(); err != nil {
		return nil, err
	}
	s.withLive(ctx, driverID, &d.Places)
	return d, nil
}

// Charts reads the series for a person page's Charts tab over the last `days` days.
func (s *Service) Charts(ctx context.Context, role, userID string, days int, allowed Allowed) (*Charts, error) {
	home := ""
	if role == trips.AsDriver {
		var err error
		if home, err = s.d.DriverCountry(ctx, userID); err != nil {
			return nil, err
		}
	}
	countries, err := s.visible(ctx, role, userID, home, allowed)
	if err != nil {
		return nil, err
	}
	if home == "" {
		home = first(countries)
	}
	tz := s.d.Timezone(home)
	c := &Charts{}
	if c.PersonSeries, err = s.d.Trips.PersonSeries(ctx, role, userID, tz, days); err != nil {
		return nil, err
	}
	if role == trips.AsDriver && s.d.Offers != nil {
		since := time.Now().AddDate(0, 0, -(c.Days - 1))
		c.Offers, err = s.d.Offers.OfferSeries(ctx, userID, tz, since)
	}
	return c, err
}

// Check refuses a person outside the staff member's countries (before a
// write such as ending their sessions, or the fleet application page).
func (s *Service) Check(ctx context.Context, role, userID string, allowed Allowed) error {
	if allowed == nil {
		return nil
	}
	home := ""
	if role == trips.AsDriver {
		var err error
		if home, err = s.d.DriverCountry(ctx, userID); err != nil {
			return err
		}
	}
	_, err := s.visible(ctx, role, userID, home, allowed)
	return err
}

// visible checks the person against the staff member's countries: their
// registration country (drivers) or any country they have trips in. It
// returns the trip countries.
func (s *Service) visible(ctx context.Context, role, userID, home string, allowed Allowed) ([]string, error) {
	countries, err := s.d.Trips.Countries(ctx, role, userID)
	if err != nil || allowed == nil {
		return countries, err
	}
	if (home != "" && allowed(home)) || slices.ContainsFunc(countries, allowed) {
		return countries, nil
	}
	return nil, ErrOutOfScope
}

// places reads saved places, the likely home, frequent routes and the last trip stop.
func (s *Service) places(ctx context.Context, role, userID, country string) (Places, error) {
	p := Places{}
	var err error
	if p.Saved, err = s.d.Places.ListPlaces(ctx, userID); err != nil {
		return p, err
	}
	p.Saved = nonNil(p.Saved)
	if p.LikelyHome, err = s.d.Trips.LikelyHome(ctx, role, userID, s.d.Timezone(country)); err != nil {
		return p, err
	}
	if p.Routes, err = s.d.Trips.TopRoutes(ctx, role, userID); err != nil {
		return p, err
	}
	last, err := s.d.Trips.LastPlace(ctx, role, userID)
	if last != nil && last.At != nil {
		p.Last = &Location{Lat: last.Lat, Lng: last.Lng, At: *last.At, Source: SourceTrip, Address: last.Address, TripCode: last.TripCode}
	}
	return p, err
}

// withLive replaces the last trip stop with the live position when the driver
// app reported one more recently (presence expires when pings stop).
func (s *Service) withLive(ctx context.Context, driverID string, p *Places) {
	if s.d.Live == nil {
		return
	}
	live, err := s.d.Live.Get(ctx, driverID)
	if err != nil || live == nil || (p.Last != nil && live.At.Before(p.Last.At)) {
		return
	}
	p.Last = &Location{Lat: live.Location.Lat, Lng: live.Location.Lng, At: live.At, Source: SourceLive, State: string(live.State)}
}

// driverMoney reads the wallet, holds, cash-outs and pay-ins (each optional).
func (s *Service) driverMoney(ctx context.Context, driverID string) (DriverMoney, error) {
	var m DriverMoney
	var err error
	if m.Wallet, err = s.d.Money.DriverWallet(ctx, driverID, walletEntriesLimit); err != nil {
		return m, err
	}
	if s.d.Holds != nil {
		if m.Holds, err = s.d.Holds.Status(ctx, driverID); err != nil {
			return m, err
		}
	}
	if s.d.Payouts != nil {
		if m.Payouts, err = s.d.Payouts.List(ctx, driverID); err != nil {
			return m, err
		}
	}
	if s.d.Payins != nil {
		m.Payins, err = s.d.Payins.List(ctx, "", driverID, "", "", payinsLimit)
	}
	// Lists are never null on the wire.
	m.Payouts = nonNil(m.Payouts)
	m.Payins = nonNil(m.Payins)
	m.Holds.Holds = nonNil(m.Holds.Holds)
	return m, err
}

func nonNil[T any](list []T) []T {
	if list == nil {
		return []T{}
	}
	return list
}

func first(list []string) string {
	if len(list) == 0 {
		return ""
	}
	return list[0]
}
