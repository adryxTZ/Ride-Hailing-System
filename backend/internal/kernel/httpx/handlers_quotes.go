package httpx

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
	"github.com/kalen-limited/hauler-rider/backend/internal/pricing"
)

// mountQuotes registers POST /quotes. Anonymous quotes are allowed (the home
// screen prices before sign-in); a signed-in caller's quote is linked to them.
func mountQuotes(r chi.Router, d Deps) {
	h := quoteHandlers{deps: d}
	r.With(optionalAuth(d)).Post("/quotes", h.create)
	r.Get("/pricing/engines", h.engines)
}

// engines lists the pricing modules with their settings schemas (admin selector).
func (h quoteHandlers) engines(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"data": h.deps.Pricing.Engines()})
}

type quoteHandlers struct{ deps Deps }

func (h quoteHandlers) create(w http.ResponseWriter, req *http.Request) {
	var body pricing.Request
	if !decode(w, req, &body) {
		return
	}
	if body.ServiceKey == "" {
		writeError(w, req, apperr.Validation, map[string]any{"fields": []map[string]string{{"path": "service", "code": "REQUIRED"}}})
		return
	}
	if body.Pickup.Lat == 0 && body.Pickup.Lng == 0 {
		writeError(w, req, apperr.Validation, map[string]any{"fields": []map[string]string{{"path": "pickup", "code": "REQUIRED"}}})
		return
	}
	if c := claimsFrom(req.Context()); c != nil {
		body.UserID = c.Subject
	}
	q, err := h.deps.Pricing.Quote(req.Context(), body)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, q)
}
