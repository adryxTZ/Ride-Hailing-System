package main

import (
	"github.com/kalen-limited/hauler-rider/backend/internal/dispatch"
	"github.com/kalen-limited/hauler-rider/backend/internal/fleet"
	"github.com/kalen-limited/hauler-rider/backend/internal/holds"
	"github.com/kalen-limited/hauler-rider/backend/internal/identity"
	"github.com/kalen-limited/hauler-rider/backend/internal/messaging"
	"github.com/kalen-limited/hauler-rider/backend/internal/money"
	"github.com/kalen-limited/hauler-rider/backend/internal/payments"
	"github.com/kalen-limited/hauler-rider/backend/internal/payouts"
	"github.com/kalen-limited/hauler-rider/backend/internal/people"
	"github.com/kalen-limited/hauler-rider/backend/internal/presence"
	"github.com/kalen-limited/hauler-rider/backend/internal/profile"
	"github.com/kalen-limited/hauler-rider/backend/internal/safety"
	"github.com/kalen-limited/hauler-rider/backend/internal/trips"
)

// peopleModules are the services the staff rider and driver pages read from.
type peopleModules struct {
	ident    *identity.Service
	trips    *trips.Service
	dispatch *dispatch.Service
	msg      *messaging.Service
	safety   *safety.Service
	profile  *profile.Service
	presence *presence.Service
	ledger   *money.Ledger
	holds    *holds.Service
	payouts  *payouts.Service
	payins   *payments.Service
	fleet    *fleet.Service
}

// newPeople wires the staff person pages; nil when a module they need is off
// (messaging, safety or fleet). Optional modules are only set when present, so
// a nil service never hides inside a non-nil interface.
func newPeople(bs *bootstrap, m peopleModules) *people.Service {
	if m.msg == nil || m.safety == nil || m.fleet == nil {
		return nil
	}
	d := people.Deps{
		Accounts: m.ident, Trips: m.trips, Ratings: m.msg, Alerts: m.safety, Places: m.profile, Money: m.ledger,
		Timezone:      func(country string) string { return countryTimezone(bs, country) },
		DriverCountry: m.fleet.CountryOf,
	}
	if m.dispatch != nil {
		d.Offers = m.dispatch
	}
	if m.presence != nil {
		d.Live = m.presence
	}
	if m.holds != nil {
		d.Holds = m.holds
	}
	if m.payouts != nil {
		d.Payouts = m.payouts
	}
	if m.payins != nil {
		d.Payins = m.payins
	}
	return people.New(d)
}

// countryTimezone is a country's timezone from the brand file (brand default otherwise).
func countryTimezone(bs *bootstrap, country string) string {
	if c := bs.brand.CountryByCode(country); c != nil && c.Timezone != "" {
		return c.Timezone
	}
	return bs.brand.Timezone
}
