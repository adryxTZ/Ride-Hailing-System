package httpx

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/kalen-limited/hauler-rider/backend/internal/geofences"
	"github.com/kalen-limited/hauler-rider/backend/internal/identity"
)

// mountGeofences registers districts, zones and places. Reads are open to
// staff; writes need areas.edit and access to the geofence's country.
func mountGeofences(r chi.Router, d Deps) {
	if d.Geofences == nil {
		return
	}
	h := geofenceHandlers{deps: d}
	r.Group(func(g chi.Router) {
		g.Use(requireAuth(d), requireStaff())
		g.With(scopeGuard).Get("/admin/geofences", h.list)
		g.With(scopeGuard).Get("/admin/geofences/overlaps", h.overlaps)
		g.Get("/admin/geofences/{id}", h.get)
		w := g.With(requirePermission(identity.PermAreasEdit))
		w.Post("/admin/geofences", h.create)
		w.Post("/admin/geofences/import", h.importFeatures)
		w.Put("/admin/geofences/{id}/geometry", h.reshape)
		w.Patch("/admin/geofences/{id}", h.update)
	})
}

type geofenceHandlers struct{ deps Deps }

func (h geofenceHandlers) list(w http.ResponseWriter, req *http.Request) {
	q := req.URL.Query()
	list, err := h.deps.Geofences.List(req.Context(), scopeFrom(req, h.deps).Country, q.Get("kind"), q.Get("status"))
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": list})
}

func (h geofenceHandlers) overlaps(w http.ResponseWriter, req *http.Request) {
	list, err := h.deps.Geofences.Overlaps(req.Context(), scopeFrom(req, h.deps).Country, "")
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": list})
}

// fenceIn loads a geofence and refuses one outside the staff member's countries.
func (h geofenceHandlers) fenceIn(w http.ResponseWriter, req *http.Request) (*geofences.Geofence, bool) {
	id := chi.URLParam(req, "id")
	if _, err := uuid.Parse(id); err != nil {
		writeError(w, req, geofences.ErrNotFound, nil)
		return nil, false
	}
	f, err := h.deps.Geofences.Get(req.Context(), id)
	if err != nil {
		writeDomainError(w, req, err)
		return nil, false
	}
	if !allowedScope(claimsFrom(req.Context()), f.Country, "") {
		writeError(w, req, ErrScopeForbidden, nil)
		return nil, false
	}
	return f, true
}

func (h geofenceHandlers) get(w http.ResponseWriter, req *http.Request) {
	if f, ok := h.fenceIn(w, req); ok {
		writeJSON(w, http.StatusOK, f)
	}
}

func (h geofenceHandlers) create(w http.ResponseWriter, req *http.Request) {
	var body geofences.CreateInput
	if !decode(w, req, &body) {
		return
	}
	if !allowedScope(claimsFrom(req.Context()), strings.ToUpper(body.Country), "") {
		writeError(w, req, ErrScopeForbidden, nil)
		return
	}
	f, err := h.deps.Geofences.Create(req.Context(), claimsFrom(req.Context()).Subject, body)
	respond(w, req, http.StatusCreated, f, err)
}

type reshapeBody struct {
	geofences.ShapeInput
	Reason string `json:"reason"`
}

func (h geofenceHandlers) reshape(w http.ResponseWriter, req *http.Request) {
	var body reshapeBody
	if !decode(w, req, &body) {
		return
	}
	if _, ok := h.fenceIn(w, req); !ok {
		return
	}
	f, err := h.deps.Geofences.Reshape(req.Context(), claimsFrom(req.Context()).Subject, chi.URLParam(req, "id"), body.ShapeInput, body.Reason)
	respond(w, req, http.StatusOK, f, err)
}

func (h geofenceHandlers) update(w http.ResponseWriter, req *http.Request) {
	var body geofences.UpdateInput
	if !decode(w, req, &body) {
		return
	}
	if _, ok := h.fenceIn(w, req); !ok {
		return
	}
	f, err := h.deps.Geofences.Update(req.Context(), claimsFrom(req.Context()).Subject, chi.URLParam(req, "id"), body)
	respond(w, req, http.StatusOK, f, err)
}

func (h geofenceHandlers) importFeatures(w http.ResponseWriter, req *http.Request) {
	req.Body = http.MaxBytesReader(w, req.Body, geofences.ImportMaxBytes+importEnvelopeBytes)
	var body geofences.ImportInput
	if !decode(w, req, &body) {
		return
	}
	if !allowedScope(claimsFrom(req.Context()), strings.ToUpper(body.Country), "") {
		writeError(w, req, ErrScopeForbidden, nil)
		return
	}
	res, err := h.deps.Geofences.Import(req.Context(), claimsFrom(req.Context()).Subject, body)
	respond(w, req, http.StatusOK, map[string]any{"results": res}, err)
}

// importEnvelopeBytes allows for the request fields around the features.
const importEnvelopeBytes = 4 << 10
