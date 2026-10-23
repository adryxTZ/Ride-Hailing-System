package httpx

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/kalen-limited/hauler-rider/backend/internal/profile"
)

// mountMe registers the signed-in person's own resources: profile and saved places.
func mountMe(r chi.Router, d Deps) {
	h := meHandlers{deps: d}
	r.Group(func(g chi.Router) {
		g.Use(requireAuth(d))
		g.Get("/me", h.get)
		g.Patch("/me", h.update)
		g.Get("/me/places", h.listPlaces)
		g.Post("/me/places", h.savePlace)
		g.Delete("/me/places/{id}", h.deletePlace)
		g.Get("/me/dues", h.dues)
	})
}

type meHandlers struct{ deps Deps }

func (h meHandlers) get(w http.ResponseWriter, req *http.Request) {
	p, err := h.deps.Profile.Get(req.Context(), claimsFrom(req.Context()).Subject)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, withClaims(p, req))
}

func (h meHandlers) update(w http.ResponseWriter, req *http.Request) {
	var body profile.ProfileUpdate
	if !decode(w, req, &body) {
		return
	}
	p, err := h.deps.Profile.Update(req.Context(), claimsFrom(req.Context()).Subject, body)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, withClaims(p, req))
}

func (h meHandlers) listPlaces(w http.ResponseWriter, req *http.Request) {
	places, err := h.deps.Profile.ListPlaces(req.Context(), claimsFrom(req.Context()).Subject)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, listResponse[profile.Place]{Data: places})
}

func (h meHandlers) savePlace(w http.ResponseWriter, req *http.Request) {
	var body profile.PlaceInput
	if !decode(w, req, &body) {
		return
	}
	p, err := h.deps.Profile.SavePlace(req.Context(), claimsFrom(req.Context()).Subject, body)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (h meHandlers) deletePlace(w http.ResponseWriter, req *http.Request) {
	if err := h.deps.Profile.DeletePlace(req.Context(), claimsFrom(req.Context()).Subject, chi.URLParam(req, "id")); err != nil {
		writeDomainError(w, req, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// meResponse is the profile plus the token's brand/app so clients need one call.
type meResponse struct {
	*profile.Profile
	Brand string `json:"brand"`
	App   string `json:"app"`
}

func withClaims(p *profile.Profile, req *http.Request) meResponse {
	c := claimsFrom(req.Context())
	return meResponse{Profile: p, Brand: c.Brand, App: c.App}
}

// dues is what the rider owes from earlier cancellations (added to their next trip).
func (h meHandlers) dues(w http.ResponseWriter, req *http.Request) {
	if h.deps.Ledger == nil {
		writeJSON(w, http.StatusOK, map[string]string{"amount": "0"})
		return
	}
	owed, err := h.deps.Ledger.RiderDues(req.Context(), claimsFrom(req.Context()).Subject)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"amount": owed.StringFixed(0)})
}
