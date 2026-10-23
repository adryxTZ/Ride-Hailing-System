package routing

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"
)

// OSRM routes over OpenStreetMap roads with an OSRM server. The public demo
// server (router.project-osrm.org) is for development only; production points
// BaseURL at a self-hosted OSRM/Valhalla with a Tanzania extract.
type OSRM struct {
	BaseURL string
	Client  *http.Client
}

const osrmTimeout = 4 * time.Second

func NewOSRM(baseURL string) OSRM {
	return OSRM{BaseURL: baseURL, Client: &http.Client{Timeout: osrmTimeout}}
}

type osrmResponse struct {
	Code   string `json:"code"`
	Routes []struct {
		Distance float64   `json:"distance"`
		Duration float64   `json:"duration"`
		Geometry string    `json:"geometry"` // encoded polyline, precision 5 (same as Google)
		Legs     []osrmLeg `json:"legs"`
	} `json:"routes"`
}

type osrmLeg struct {
	Steps []struct {
		Distance float64 `json:"distance"`
		Name     string  `json:"name"`
		Maneuver struct {
			Type     string     `json:"type"`
			Modifier string     `json:"modifier"`
			Exit     int        `json:"exit"`
			Location [2]float64 `json:"location"` // lng, lat
		} `json:"maneuver"`
	} `json:"steps"`
}

func (o OSRM) Route(ctx context.Context, from, to Point) (Route, error) {
	return o.RouteVia(ctx, []Point{from, to})
}

// RouteVia routes through every point in order in one OSRM request.
func (o OSRM) RouteVia(ctx context.Context, points []Point) (Route, error) {
	coords := make([]string, len(points))
	for i, p := range points {
		coords[i] = fmt.Sprintf("%f,%f", p.Lng, p.Lat)
	}
	url := fmt.Sprintf("%s/route/v1/driving/%s?overview=full&geometries=polyline&steps=true", o.BaseURL, strings.Join(coords, ";"))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Route{}, err
	}
	res, err := o.Client.Do(req)
	if err != nil {
		return Route{}, fmt.Errorf("osrm: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	var body osrmResponse
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return Route{}, fmt.Errorf("osrm decode: %w", err)
	}
	if body.Code != "Ok" || len(body.Routes) == 0 {
		return Route{}, fmt.Errorf("osrm: %s", body.Code)
	}
	r := body.Routes[0]
	out := Route{DistanceM: int(math.Round(r.Distance)), DurationS: int(math.Round(r.Duration)), Polyline: r.Geometry, Provider: "osrm"}
	for li, leg := range r.Legs {
		for _, st := range leg.Steps {
			typ := st.Maneuver.Type
			if typ == "arrive" && li < len(r.Legs)-1 {
				typ = "waypoint" // arriving at an intermediate stop
			}
			out.Steps = append(out.Steps, Step{Type: typ, Modifier: st.Maneuver.Modifier, Street: st.Name, Exit: st.Maneuver.Exit,
				DistanceM: int(math.Round(st.Distance)), Location: Point{Lat: st.Maneuver.Location[1], Lng: st.Maneuver.Location[0]}})
		}
	}
	return out, nil
}
