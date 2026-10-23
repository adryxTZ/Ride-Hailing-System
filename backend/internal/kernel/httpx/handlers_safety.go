package httpx

import (
	"github.com/kalen-limited/hauler-rider/backend/internal/identity"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/kalen-limited/hauler-rider/backend/internal/safety"
)

// mountSafety registers trusted contacts, trip sharing, SOS, the public share
// data, and the staff SOS queue.
func mountSafety(r chi.Router, d Deps) {
	if d.Safety == nil {
		return
	}
	h := safetyHandlers{deps: d}
	r.Get("/share/{token}", h.publicView) // public: whoever has the link
	r.Group(func(g chi.Router) {
		g.Use(requireAuth(d))
		g.Get("/me/trusted-contacts", h.contacts)
		g.Put("/me/trusted-contacts", h.saveContacts)
		g.Post("/trips/{id}/share", h.share)
		g.Post("/trips/{id}/sos", h.sos)
	})
	r.Group(func(g chi.Router) {
		g.Use(requireAuth(d), requireStaff())
		g.Get("/admin/sos", h.alerts)
		g.With(requirePermission(identity.PermSafetyResolve)).Post("/admin/sos/{id}/resolve", h.resolve)
	})
}

type safetyHandlers struct{ deps Deps }

// publicBase is where share pages live: the configured public site, or this API's own host.
func publicBase(req *http.Request, configured string) string {
	if configured != "" {
		return strings.TrimRight(configured, "/")
	}
	scheme := "http"
	if req.TLS != nil || req.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	return scheme + "://" + req.Host
}

func (h safetyHandlers) contacts(w http.ResponseWriter, req *http.Request) {
	list, err := h.deps.Safety.Contacts(req.Context(), claimsFrom(req.Context()).Subject)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, listResponse[safety.TrustedContact]{Data: list})
}

func (h safetyHandlers) saveContacts(w http.ResponseWriter, req *http.Request) {
	var body struct {
		Data []safety.TrustedContact `json:"data"`
	}
	if !decode(w, req, &body) {
		return
	}
	list, err := h.deps.Safety.SaveContacts(req.Context(), claimsFrom(req.Context()).Subject, body.Data)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, listResponse[safety.TrustedContact]{Data: list})
}

func (h safetyHandlers) share(w http.ResponseWriter, req *http.Request) {
	link, err := h.deps.Safety.ShareLink(req.Context(), chi.URLParam(req, "id"), claimsFrom(req.Context()).Subject, publicBase(req, h.deps.PublicWebURL))
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, link)
}

func (h safetyHandlers) sos(w http.ResponseWriter, req *http.Request) {
	var body safety.SOSInput
	if !decode(w, req, &body) {
		return
	}
	a, err := h.deps.Safety.SOS(req.Context(), chi.URLParam(req, "id"), claimsFrom(req.Context()).Subject, publicBase(req, h.deps.PublicWebURL), body)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusCreated, a)
}

func (h safetyHandlers) publicView(w http.ResponseWriter, req *http.Request) {
	v, err := h.deps.Safety.PublicView(req.Context(), chi.URLParam(req, "token"))
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, v)
}

func (h safetyHandlers) alerts(w http.ResponseWriter, req *http.Request) {
	list, err := h.deps.Safety.Alerts(req.Context(), req.URL.Query().Get("all") != "1")
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, listResponse[safety.Alert]{Data: list})
}

func (h safetyHandlers) resolve(w http.ResponseWriter, req *http.Request) {
	var body struct {
		Resolution string `json:"resolution"`
	}
	if !decode(w, req, &body) {
		return
	}
	if err := h.deps.Safety.Resolve(req.Context(), chi.URLParam(req, "id"), claimsFrom(req.Context()).Subject, body.Resolution); err != nil {
		writeDomainError(w, req, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
