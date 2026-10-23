package httpx

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/kalen-limited/hauler-rider/backend/api"
	"github.com/kalen-limited/hauler-rider/backend/internal/branding"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
)

// mountPublic registers the unauthenticated routes: liveness, readiness,
// app configuration, theme presets and the service catalogue.
func mountPublic(r chi.Router, d Deps) {
	h := publicHandlers{deps: d}
	r.Get("/health", h.health)
	r.Get("/ready", h.ready)
	r.Get("/app/config", h.appConfig)
	r.Get("/theme/presets", h.themePresets)
	r.Get("/openapi.yaml", h.openapiSpec)
}

// publicHandlers groups the public endpoints so they share Deps without
// package-level state.
type publicHandlers struct {
	deps Deps
}

// health answers liveness: the process is up. It never touches dependencies.
func (h publicHandlers) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{Status: "ok", Version: h.deps.Version})
}

// ready answers readiness: dependencies reachable and migrations applied.
// Load balancers use this, not /health, to decide whether to route traffic.
func (h publicHandlers) ready(w http.ResponseWriter, req *http.Request) {
	if h.deps.Ready != nil {
		if err := h.deps.Ready(req.Context()); err != nil {
			writeError(w, req, apperr.NotReady, map[string]any{"reason": err.Error()})
			return
		}
	}
	writeJSON(w, http.StatusOK, healthResponse{Status: "ok", Version: h.deps.Version})
}

// appConfig returns brand identity, the published theme, feature flags and the
// minimum app version. It is ETag-cached so apps can poll it cheaply.
func (h publicHandlers) appConfig(w http.ResponseWriter, req *http.Request) {
	payload := h.baseConfig()
	if h.deps.Branding != nil {
		live, err := h.deps.Branding.Published(req.Context())
		if err != nil {
			writeDomainError(w, req, err)
			return
		}
		applyIdentity(&payload.Brand, live.Identity)
		payload.Theme = live.Theme
	}
	writeJSONWithETag(w, req, payload, 60)
}

// applyIdentity puts the identity staff set on the Branding page over the brand file's.
func applyIdentity(b *brandResponse, id branding.Identity) {
	b.Name, b.LegalEntity, b.SupportPhone, b.SupportEmail = id.Name, id.LegalEntity, id.SupportPhone, id.SupportEmail
	if id.EmergencyPhone != "" {
		b.EmergencyPhone = id.EmergencyPhone
	}
}

// baseConfig is app config from the brand file (used as is without the branding module).
func (h publicHandlers) baseConfig() appConfigResponse {
	return appConfigResponse{
		Brand: brandResponse{
			Code:           h.deps.Brand.Code,
			Name:           h.deps.Brand.DisplayName,
			LegalEntity:    h.deps.Brand.LegalEntity,
			Country:        h.deps.Brand.Country,
			Currency:       h.deps.Brand.Currency,
			Locale:         h.deps.Brand.DefaultLocale,
			SupportPhone:   h.deps.Brand.SupportPhone,
			SupportEmail:   h.deps.Brand.SupportEmail,
			EmergencyPhone: h.deps.Brand.EmergencyPhone,
		},
		Theme:         h.deps.Theme,
		Features:      h.deps.Features,
		MinAppVersion: h.deps.MinApp,
	}
}

// themePresets lists the curated palette gallery the admin offers.
func (h publicHandlers) themePresets(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, listResponse[any]{Data: toAnySlice(h.deps.Presets.List())})
}

// openapiSpec serves the embedded contract so SDKs can be generated from a
// running instance (keeps apps and frontend deployable without this repo).
func (h publicHandlers) openapiSpec(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/yaml")
	w.Header().Set("Cache-Control", "public, max-age=300")
	_, _ = w.Write(api.Spec)
}
