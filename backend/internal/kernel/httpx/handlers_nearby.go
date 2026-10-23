package httpx

import (
	"time"

	"github.com/kalen-limited/hauler-rider/backend/internal/presence"
	"math"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
	"github.com/kalen-limited/hauler-rider/backend/internal/providers/routing"
)

const (
	nearbyRings = 8  // H3 res-9 rings ≈ 2.8 km around the rider
	nearbyLimit = 30 // enough to draw cars and pick the nearest per class
	// nearbyCacheTTL: riders in the same ~0.7 km² share one result for this long.
	nearbyCacheTTL = 3 * time.Second
)

// mountNearby registers the rider-facing supply view: idle drivers around a point,
// used to draw cars on the map and estimate pickup time per vehicle class.
// Reads Redis only (presence); never touches Postgres.
func mountNearby(r chi.Router, d Deps) {
	if d.Presence == nil {
		return
	}
	cache := presence.NewNearbyCache(d.Presence, nearbyCacheTTL)
	r.Group(func(g chi.Router) {
		g.Use(requireAuth(d), requirePresence(d))
		g.Get("/drivers/nearby", nearbyHandler(cache))
	})
}

// nearbyVehicle is what a rider may see of a driver: position and class, never identity.
type nearbyVehicle struct {
	Location       routing.Point `json:"location"`
	Heading        float64       `json:"heading"`
	VehicleClasses []string      `json:"vehicle_classes"`
	DistanceM      int           `json:"distance_m"`
}

type nearbyResponse struct {
	Data []nearbyVehicle `json:"data"`
}

func nearbyHandler(cache *presence.NearbyCache) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		at, ok := pointFromQuery(req)
		if !ok {
			writeError(w, req, apperr.Validation, map[string]any{"fields": []string{"lat", "lng"}})
			return
		}
		q := req.URL.Query()
		drivers, err := cache.Nearby(req.Context(), at, nearbyRings, q.Get("service"), q.Get("vehicle_class"), nearbyLimit)
		if err != nil {
			writeDomainError(w, req, err)
			return
		}
		out := make([]nearbyVehicle, 0, len(drivers))
		for _, dr := range drivers {
			if dr.State != "IDLE" { // busy drivers are not available to this rider
				continue
			}
			out = append(out, nearbyVehicle{
				Location:       dr.Location,
				Heading:        dr.Heading,
				VehicleClasses: dr.Classes,
				DistanceM:      int(math.Round(routing.HaversineM(at, dr.Location))),
			})
		}
		writeJSON(w, http.StatusOK, nearbyResponse{Data: out})
	}
}

// pointFromQuery reads ?lat=&lng= and rejects missing, malformed or out-of-range values.
func pointFromQuery(req *http.Request) (routing.Point, bool) {
	lat, err1 := strconv.ParseFloat(req.URL.Query().Get("lat"), 64)
	lng, err2 := strconv.ParseFloat(req.URL.Query().Get("lng"), 64)
	p := routing.Point{Lat: lat, Lng: lng}
	return p, err1 == nil && err2 == nil && validPoint(p)
}
