// Package routing defines the RoutingProvider port (road distance and ETA
// between two points) and the development implementation. Google Directions
// and a self-hosted Valhalla are separate implementations selected per brand.
package routing

import (
	"context"
	"math"
)

// Point is a WGS84 coordinate.
type Point struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

// Route is the provider's answer.
type Route struct {
	DistanceM int    `json:"distance_m"`
	DurationS int    `json:"duration_s"`
	Polyline  string `json:"polyline,omitempty"`
	Provider  string `json:"provider"`
	// Steps are the turn-by-turn maneuvers (providers that have them); used for driver navigation.
	Steps []Step `json:"steps,omitempty"`
}

// Step is one maneuver: "turn left onto Makongo Road", then drive DistanceM.
type Step struct {
	Type      string `json:"type"`               // depart, turn, continue, roundabout, arrive, fork, merge, end of road…
	Modifier  string `json:"modifier,omitempty"` // left, right, slight left, sharp right, straight, uturn
	Street    string `json:"street,omitempty"`
	DistanceM int    `json:"distance_m"`     // length of this step (to the next maneuver)
	Exit      int    `json:"exit,omitempty"` // roundabout exit number
	Location  Point  `json:"location"`       // where the maneuver happens
}

// Provider computes a drivable route between two points.
type Provider interface {
	Route(ctx context.Context, from, to Point) (Route, error)
}

// Estimate is the development provider: straight-line distance scaled by a
// road-winding factor and an average urban speed. Deterministic and offline.
type Estimate struct {
	RoadFactor float64 // 1.3 is a common urban average
	SpeedKmh   float64 // 25 km/h for Dar es Salaam traffic
}

// DefaultEstimate returns the development provider with sensible constants.
func DefaultEstimate() Estimate { return Estimate{RoadFactor: 1.3, SpeedKmh: 25} }

func (e Estimate) Route(_ context.Context, from, to Point) (Route, error) {
	meters := HaversineM(from, to) * e.RoadFactor
	seconds := meters / 1000 / e.SpeedKmh * 3600
	return Route{DistanceM: int(math.Round(meters)), DurationS: int(math.Round(seconds)), Provider: "estimate"}, nil
}

// HaversineM is the great-circle distance in metres.
func HaversineM(a, b Point) float64 {
	const r = 6371000.0
	toRad := func(d float64) float64 { return d * math.Pi / 180 }
	dLat := toRad(b.Lat - a.Lat)
	dLng := toRad(b.Lng - a.Lng)
	h := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(toRad(a.Lat))*math.Cos(toRad(b.Lat))*math.Sin(dLng/2)*math.Sin(dLng/2)
	return 2 * r * math.Asin(math.Sqrt(h))
}
