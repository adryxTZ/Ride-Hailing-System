package routing

import "context"

// MaxWaypoints is how many intermediate stops a route may have.
const MaxWaypoints = 3

// MultiProvider routes through several points in order in one call.
type MultiProvider interface {
	RouteVia(ctx context.Context, points []Point) (Route, error)
}

// Via routes through points in order (pickup, stops…, drop-off). Providers that
// support it answer in one call; otherwise the legs are routed and summed.
func Via(ctx context.Context, p Provider, points []Point) (Route, error) {
	if len(points) == 2 {
		return p.Route(ctx, points[0], points[1])
	}
	if m, ok := p.(MultiProvider); ok {
		return m.RouteVia(ctx, points)
	}
	total := Route{}
	for i := 1; i < len(points); i++ {
		leg, err := p.Route(ctx, points[i-1], points[i])
		if err != nil {
			return Route{}, err
		}
		total.DistanceM += leg.DistanceM
		total.DurationS += leg.DurationS
		total.Provider = leg.Provider
	}
	return total, nil
}
