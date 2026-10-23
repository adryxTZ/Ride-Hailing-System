package httpx

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
	"github.com/kalen-limited/hauler-rider/backend/internal/providers/routing"
)

// mountRoutes registers GET /v1/routes?from=lat,lng&to=lat,lng[&via=lat,lng|lat,lng]: a road route
// (distance, duration, encoded polyline) for drawing lines and showing ETAs.
// Answers come from the routing cache (Redis, ~11 m cells) most of the time.
func mountRoutes(r chi.Router, d Deps) {
	if d.Routing == nil {
		return
	}
	r.Group(func(g chi.Router) {
		g.Use(requireAuth(d))
		g.Get("/routes", func(w http.ResponseWriter, req *http.Request) {
			from, ok1 := parseLatLng(req.URL.Query().Get("from"))
			to, ok2 := parseLatLng(req.URL.Query().Get("to"))
			if !ok1 || !ok2 {
				writeError(w, req, apperr.Validation, map[string]any{"fields": []string{"from", "to"}})
				return
			}
			points := []routing.Point{from}
			if via := req.URL.Query().Get("via"); via != "" {
				for _, part := range strings.Split(via, "|") {
					p, ok := parseLatLng(part)
					if !ok || len(points) > routing.MaxWaypoints {
						writeError(w, req, apperr.Validation, map[string]any{"fields": []string{"via"}})
						return
					}
					points = append(points, p)
				}
			}
			route, err := routing.Via(req.Context(), d.Routing, append(points, to))
			if err != nil {
				writeDomainError(w, req, err)
				return
			}
			w.Header().Set("Cache-Control", "private, max-age=60")
			writeJSON(w, http.StatusOK, route)
		})
	})
}

func parseLatLng(s string) (routing.Point, bool) {
	parts := strings.Split(s, ",")
	if len(parts) != 2 {
		return routing.Point{}, false
	}
	lat, e1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	lng, e2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	p := routing.Point{Lat: lat, Lng: lng}
	return p, e1 == nil && e2 == nil && validPoint(p)
}
