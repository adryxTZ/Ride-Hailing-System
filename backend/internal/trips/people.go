package trips

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

// Party roles a person can hold on a trip, for the per-person figures.
const (
	AsRider  = "RIDER"
	AsDriver = "DRIVER"
)

// Bounds for the per-person reads (staff rider and driver pages).
const (
	// routeGridDecimals rounds coordinates to ~110 m so the same street corner groups together.
	routeGridDecimals = 3
	// routeSampleTrips is how many recent completed trips the route and home figures read.
	routeSampleTrips = 500
	// TopRoutesLimit is how many frequent routes a person page shows.
	TopRoutesLimit = 5
	// homeMorningHour: pickups requested before this local hour count as "leaving home".
	homeMorningHour = 10
	// homeEveningHour: drop-offs completed from this local hour count as "coming home".
	homeEveningHour = 19
	// homeMinSightings is how often a place must recur before it is called home.
	homeMinSightings = 2
	// homeLookback bounds the trips the home guess reads.
	homeLookback = 180 * 24 * time.Hour
)

// PersonStats are a rider's or driver's trip figures over their whole history.
type PersonStats struct {
	Total            int             `json:"total"`
	Completed        int             `json:"completed"`
	Active           int             `json:"active"`
	Scheduled        int             `json:"scheduled"`
	CancelledByRider int             `json:"cancelled_by_rider"`
	CancelledByDrv   int             `json:"cancelled_by_driver"`
	CancelledByStaff int             `json:"cancelled_by_staff"`
	NoDriver         int             `json:"no_driver"`
	Currency         string          `json:"currency"`
	FaresTotal       decimal.Decimal `json:"fares_total"` // completed fares (rider: spent; driver: gross fares driven)
	AvgFare          decimal.Decimal `json:"avg_fare"`
	DistanceM        int64           `json:"distance_m"`
	PaidCash         int             `json:"paid_cash"`
	PaidMobile       int             `json:"paid_mobile"`
	UnpaidFares      int             `json:"unpaid_fares"`
	// AvgPickupS is accepted → arrived at pickup (how long riders wait for this driver, or this rider waited).
	AvgPickupS *float64 `json:"avg_pickup_s"`
	// AvgRideS is in transit → completed.
	AvgRideS    *float64   `json:"avg_ride_s"`
	FirstTripAt *time.Time `json:"first_trip_at"`
	LastTripAt  *time.Time `json:"last_trip_at"`
}

// Place is a point on a person's map, with the address the trip carried.
type Place struct {
	Address *string    `json:"address"`
	Lat     float64    `json:"lat"`
	Lng     float64    `json:"lng"`
	At      *time.Time `json:"at,omitempty"`
	// TripCode is the trip the point came from (last known place).
	TripCode *string `json:"trip_code,omitempty"`
	// Sightings is how many trips started or ended there (likely home).
	Sightings int `json:"sightings,omitempty"`
}

// Route is a pickup → drop-off pair a person travels often.
type Route struct {
	From   Place     `json:"from"`
	To     Place     `json:"to"`
	Trips  int       `json:"trips"`
	LastAt time.Time `json:"last_at"`
}

// partyColumn maps a role to the trips column; anything else is refused, so
// the column name never comes from input.
func partyColumn(role string) (string, bool) {
	switch role {
	case AsRider:
		return "rider_id", true
	case AsDriver:
		return "driver_id", true
	}
	return "", false
}

// PersonStats counts a person's trips by outcome with the money and timings.
func (s *Service) PersonStats(ctx context.Context, role, userID string) (*PersonStats, error) {
	col, ok := partyColumn(role)
	if !ok {
		return nil, ErrTripNotFound
	}
	return s.store.personStats(ctx, col, userID)
}

// TopRoutes lists the pickup → drop-off pairs a person completes most often.
func (s *Service) TopRoutes(ctx context.Context, role, userID string) ([]Route, error) {
	col, ok := partyColumn(role)
	if !ok {
		return nil, ErrTripNotFound
	}
	return s.store.topRoutes(ctx, col, userID, TopRoutesLimit)
}

// LikelyHome guesses home from where morning trips start and evening trips
// end (nil when no place recurs). tz is the person's country timezone.
func (s *Service) LikelyHome(ctx context.Context, role, userID, tz string) (*Place, error) {
	col, ok := partyColumn(role)
	if !ok {
		return nil, ErrTripNotFound
	}
	return s.store.likelyHome(ctx, col, userID, tz, s.now().Add(-homeLookback))
}

// LastPlace is the last trip stop the person reached (nil before any).
func (s *Service) LastPlace(ctx context.Context, role, userID string) (*Place, error) {
	col, ok := partyColumn(role)
	if !ok {
		return nil, ErrTripNotFound
	}
	return s.store.lastPlace(ctx, col, userID)
}

// Countries lists the countries a person has trips in (for staff limited to one).
func (s *Service) Countries(ctx context.Context, role, userID string) ([]string, error) {
	col, ok := partyColumn(role)
	if !ok {
		return nil, ErrTripNotFound
	}
	return s.store.personCountries(ctx, col, userID)
}
