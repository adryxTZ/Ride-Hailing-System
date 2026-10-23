package pricing

import (
	"time"

	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/providers/routing"
)

// FixedRoute is a route-based fixed price (part of the cached pricing snapshot).
type FixedRoute struct {
	ID                 string           `json:"id"`
	Country            string           `json:"country"`
	AreaID             string           `json:"area_id,omitempty"` // "" = the whole country
	ServiceKey         string           `json:"service"`
	Name               string           `json:"name"`
	Origin             routing.Point    `json:"origin"`
	OriginRadiusM      int              `json:"origin_radius_m"`
	Destination        routing.Point    `json:"destination"`
	DestinationRadiusM int              `json:"destination_radius_m"`
	Bidirectional      bool             `json:"bidirectional"`
	VehicleClasses     []string         `json:"vehicle_classes"` // empty = all
	Price              decimal.Decimal  `json:"price"`
	ReversePrice       *decimal.Decimal `json:"reverse_price,omitempty"` // destination → origin; nil = same as Price
	Currency           string           `json:"currency"`
	IncludesLoaders    bool             `json:"-"`
	SurgeApplies       bool             `json:"surge_applies"`
	Priority           int              `json:"priority"`
	ValidFrom          *time.Time       `json:"valid_from,omitempty"`
	ValidTo            *time.Time       `json:"valid_to,omitempty"`
}

// LineFixedRoute is the fare line code for a fixed-price match.
const LineFixedRoute = "FIXED_ROUTE"

// Direction of a match.
type Direction int

const (
	NoMatch Direction = iota
	Forward           // pickup at origin, dropoff at destination
	Reverse           // pickup at destination, dropoff at origin (bidirectional routes only)
)

// matches reports in which direction this route applies to the trip and class.
func (r FixedRoute) matches(service string, pickup, dropoff routing.Point, class string, now time.Time) Direction {
	if r.ServiceKey != service {
		return NoMatch
	}
	if r.ValidFrom != nil && now.Before(*r.ValidFrom) || r.ValidTo != nil && now.After(*r.ValidTo) {
		return NoMatch
	}
	if len(r.VehicleClasses) > 0 && !contains(r.VehicleClasses, class) {
		return NoMatch
	}
	if within(pickup, r.Origin, r.OriginRadiusM) && within(dropoff, r.Destination, r.DestinationRadiusM) {
		return Forward
	}
	if r.Bidirectional && within(pickup, r.Destination, r.DestinationRadiusM) && within(dropoff, r.Origin, r.OriginRadiusM) {
		return Reverse
	}
	return NoMatch
}

// PriceFor returns the price for the matched direction.
func (r FixedRoute) PriceFor(dir Direction) decimal.Decimal {
	if dir == Reverse && r.ReversePrice != nil {
		return *r.ReversePrice
	}
	return r.Price
}

// matchFixedRoute returns the highest-priority matching route and its direction, or nil.
func matchFixedRoute(routes []FixedRoute, service string, pickup, dropoff routing.Point, class string, now time.Time) (*FixedRoute, Direction) {
	var best *FixedRoute
	bestDir := NoMatch
	for i := range routes {
		r := &routes[i]
		if dir := r.matches(service, pickup, dropoff, class, now); dir != NoMatch && (best == nil || r.Priority > best.Priority) {
			best, bestDir = r, dir
		}
	}
	return best, bestDir
}

func within(p, centre routing.Point, radiusM int) bool {
	return routing.HaversineM(p, centre) <= float64(radiusM)
}

func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}
