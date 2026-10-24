package httpx

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/kalen-limited/hauler-rider/backend/internal/fleet"
)

// mountOnboarding registers the driver's sign-up checklist (driver app only).
func mountOnboarding(r chi.Router, d Deps) {
	if d.Fleet == nil {
		return
	}
	h := onboardingHandlers{deps: d}
	r.Group(func(g chi.Router) {
		g.Use(requireAuth(d), requireApp("driver"))
		g.Get("/driver/onboarding", h.get)
		g.Put("/driver/onboarding/profile", h.profile)
		g.Put("/driver/onboarding/vehicle", h.vehicle)
		g.Post("/driver/onboarding/uploads", h.upload)
		g.Post("/driver/onboarding/documents", h.document)
		g.Post("/driver/onboarding/submit", h.submit)
	})
}

type onboardingHandlers struct{ deps Deps }

// respond writes the checklist or the error.
func respondOnboarding(w http.ResponseWriter, req *http.Request, ob *fleet.Onboarding, err error) {
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, ob)
}

func (h onboardingHandlers) get(w http.ResponseWriter, req *http.Request) {
	ob, err := h.deps.Fleet.Onboarding(req.Context(), claimsFrom(req.Context()).Subject)
	respondOnboarding(w, req, ob, err)
}

func (h onboardingHandlers) profile(w http.ResponseWriter, req *http.Request) {
	var body fleet.ProfileInput
	if !decode(w, req, &body) {
		return
	}
	ob, err := h.deps.Fleet.SaveProfile(req.Context(), claimsFrom(req.Context()).Subject, body)
	respondOnboarding(w, req, ob, err)
}

func (h onboardingHandlers) vehicle(w http.ResponseWriter, req *http.Request) {
	var body fleet.VehicleInput
	if !decode(w, req, &body) {
		return
	}
	ob, err := h.deps.Fleet.SaveVehicle(req.Context(), claimsFrom(req.Context()).Subject, body)
	respondOnboarding(w, req, ob, err)
}

func (h onboardingHandlers) upload(w http.ResponseWriter, req *http.Request) {
	var body struct {
		ContentType string `json:"content_type"`
	}
	if !decode(w, req, &body) {
		return
	}
	t, err := h.deps.Fleet.UploadURL(req.Context(), claimsFrom(req.Context()).Subject, body.ContentType)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusCreated, t)
}

func (h onboardingHandlers) document(w http.ResponseWriter, req *http.Request) {
	var body fleet.DocumentInput
	if !decode(w, req, &body) {
		return
	}
	ob, err := h.deps.Fleet.AddDocument(req.Context(), claimsFrom(req.Context()).Subject, body)
	respondOnboarding(w, req, ob, err)
}

func (h onboardingHandlers) submit(w http.ResponseWriter, req *http.Request) {
	if !consentGate(w, req, h.deps) {
		return
	}
	ob, err := h.deps.Fleet.Submit(req.Context(), claimsFrom(req.Context()).Subject)
	respondOnboarding(w, req, ob, err)
}
