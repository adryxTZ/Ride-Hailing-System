package httpx

import (
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
	"github.com/kalen-limited/hauler-rider/backend/internal/providers/geocoding"
	"github.com/kalen-limited/hauler-rider/backend/internal/providers/routing"
)

const (
	placeMinChars = 3
	placeLimit    = 8
)

// mountPlaces registers place search (autocomplete) and reverse lookup, proxied
// and cached so apps hold no provider keys and the provider sees few requests.
func mountPlaces(r chi.Router, d Deps) {
	if d.Places == nil {
		return
	}
	r.Group(func(g chi.Router) {
		g.Use(requireAuth(d))
		g.Get("/places/search", func(w http.ResponseWriter, req *http.Request) {
			q := strings.TrimSpace(req.URL.Query().Get("q"))
			if utf8.RuneCountInString(q) < placeMinChars {
				writeJSON(w, http.StatusOK, listResponse[geocoding.Place]{Data: []geocoding.Place{}})
				return
			}
			var near *routing.Point
			if p, ok := pointFromQuery(req); ok {
				near = &p
			}
			places, err := d.Places.Search(req.Context(), q, near, placeLimit)
			if err != nil {
				writeError(w, req, apperr.New("PLACES_UNAVAILABLE", http.StatusServiceUnavailable, ""), nil)
				return
			}
			writeJSON(w, http.StatusOK, listResponse[geocoding.Place]{Data: places})
		})
		g.Get("/places/reverse", func(w http.ResponseWriter, req *http.Request) {
			p, ok := pointFromQuery(req)
			if !ok {
				writeError(w, req, apperr.Validation, map[string]any{"fields": []string{"lat", "lng"}})
				return
			}
			place, err := d.Places.Reverse(req.Context(), p)
			if err != nil || place == nil {
				writeJSON(w, http.StatusOK, map[string]any{"place": nil})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"place": place})
		})
	})
}
