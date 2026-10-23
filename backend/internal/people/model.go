package people

import (
	"net/http"
	"time"

	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/dispatch"
	"github.com/kalen-limited/hauler-rider/backend/internal/holds"
	"github.com/kalen-limited/hauler-rider/backend/internal/identity"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
	"github.com/kalen-limited/hauler-rider/backend/internal/messaging"
	"github.com/kalen-limited/hauler-rider/backend/internal/money"
	"github.com/kalen-limited/hauler-rider/backend/internal/payments"
	"github.com/kalen-limited/hauler-rider/backend/internal/payouts"
	"github.com/kalen-limited/hauler-rider/backend/internal/profile"
	"github.com/kalen-limited/hauler-rider/backend/internal/safety"
	"github.com/kalen-limited/hauler-rider/backend/internal/trips"
)

// ErrOutOfScope hides a person whose trips or registration are outside the
// staff member's countries (same answer as not found).
var ErrOutOfScope = apperr.New("PERSON_NOT_FOUND", http.StatusNotFound, "")

// Location is where a person was last seen: live from the driver app when
// fresh, otherwise the last trip stop they reached.
type Location struct {
	Lat      float64   `json:"lat"`
	Lng      float64   `json:"lng"`
	At       time.Time `json:"at"`
	Source   string    `json:"source"` // LIVE | TRIP
	State    string    `json:"state,omitempty"`
	Address  *string   `json:"address,omitempty"`
	TripCode *string   `json:"trip_code,omitempty"`
}

// Location sources.
const (
	SourceLive = "LIVE"
	SourceTrip = "TRIP"
)

// Places is where a person goes: saved places, the inferred home and frequent routes.
type Places struct {
	Saved      []profile.Place `json:"saved"`
	LikelyHome *trips.Place    `json:"likely_home"`
	Routes     []trips.Route   `json:"routes"`
	Last       *Location       `json:"last_location"`
}

// Rider is the staff rider page.
type Rider struct {
	Account *identity.PersonAccount  `json:"account"`
	Trips   *trips.PersonStats       `json:"trips"`
	Ratings *messaging.PersonRatings `json:"ratings"`
	SOS     []safety.Alert           `json:"sos"`
	Places  Places                   `json:"places"`
	// Dues is what the rider owes from cancellation fees not yet collected.
	Dues decimal.Decimal `json:"dues"`
}

// DriverMoney is the driver's wallet, holds, cash-outs and pay-ins.
type DriverMoney struct {
	Wallet  money.Wallet          `json:"wallet"`
	Holds   holds.Status          `json:"holds"`
	Payouts []payouts.Payout      `json:"payouts"`
	Payins  []payments.AdminPayin `json:"payins"`
}

// Charts are the series behind the Charts tab of a person page.
type Charts struct {
	*trips.PersonSeries
	// Offers is a driver's offer answers per day (nil for riders, or with dispatch off).
	Offers []dispatch.OfferDay `json:"offers"`
}

// Driver is the staff driver page's activity half (the application half is fleet.DriverDetail).
type Driver struct {
	Account *identity.PersonAccount  `json:"account"`
	Trips   *trips.PersonStats       `json:"trips"`
	Offers  *dispatch.OfferStats     `json:"offers"`
	Ratings *messaging.PersonRatings `json:"ratings"`
	SOS     []safety.Alert           `json:"sos"`
	Places  Places                   `json:"places"`
	Money   DriverMoney              `json:"money"`
}
