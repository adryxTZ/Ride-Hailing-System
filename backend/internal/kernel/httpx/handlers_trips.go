package httpx

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
	"github.com/kalen-limited/hauler-rider/backend/internal/trips"
)

// mountTrips registers the rider's booking routes. All require a signed-in caller.
func mountTrips(r chi.Router, d Deps) {
	h := tripHandlers{deps: d}
	r.Group(func(g chi.Router) {
		g.Use(requireAuth(d))
		g.Post("/trips", h.create)
		g.Get("/trips", h.list)
		g.Get("/trips/active", h.active)
		g.Get("/trips/{id}", h.get)
		g.Get("/trips/{id}/events", h.events)
		g.Post("/trips/{id}/cancel", h.cancel)
	})
}

type tripHandlers struct{ deps Deps }

// create books a trip from a quote option. Idempotency-Key is accepted and
// echoed; full replay protection lands with the idempotency store.
func (h tripHandlers) create(w http.ResponseWriter, req *http.Request) {
	var body trips.CreateInput
	if !decode(w, req, &body) || !consentGate(w, req, h.deps) {
		return
	}
	body.RiderID = claimsFrom(req.Context()).Subject
	t, err := h.deps.Trips.Create(req.Context(), body)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusCreated, t)
}

func (h tripHandlers) list(w http.ResponseWriter, req *http.Request) {
	list, err := h.deps.Trips.List(req.Context(), claimsFrom(req.Context()).Subject)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, listResponse[trips.Trip]{Data: list})
}

// active returns the rider's in-progress trip or `{"trip": null}`.
func (h tripHandlers) active(w http.ResponseWriter, req *http.Request) {
	t, err := h.deps.Trips.Active(req.Context(), claimsFrom(req.Context()).Subject)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	withDriverLocation(req, h.deps, t)
	writeJSON(w, http.StatusOK, map[string]any{"trip": t})
}

func (h tripHandlers) get(w http.ResponseWriter, req *http.Request) {
	t, err := h.deps.Trips.Get(req.Context(), chi.URLParam(req, "id"), claimsFrom(req.Context()).Subject)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	withDriverLocation(req, h.deps, t)
	writeJSON(w, http.StatusOK, t)
}

func (h tripHandlers) events(w http.ResponseWriter, req *http.Request) {
	after, _ := strconv.Atoi(req.URL.Query().Get("after_seq"))
	ev, err := h.deps.Trips.Events(req.Context(), chi.URLParam(req, "id"), claimsFrom(req.Context()).Subject, after)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, listResponse[trips.Event]{Data: ev})
}

type cancelBody struct {
	Reason string  `json:"reason"`
	Note   *string `json:"note"`
}

func (h tripHandlers) cancel(w http.ResponseWriter, req *http.Request) {
	var body cancelBody
	if !decode(w, req, &body) {
		return
	}
	if body.Reason == "" {
		writeError(w, req, apperr.Validation, map[string]any{"fields": []map[string]string{{"path": "reason", "code": "REQUIRED"}}})
		return
	}
	t, err := h.deps.Trips.CancelByRider(req.Context(), chi.URLParam(req, "id"), claimsFrom(req.Context()).Subject, body.Reason, body.Note)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

// withDriverLocation adds the assigned driver's last ping (Redis presence) to a
// trip response. Read-only and cheap; absent when presence is off or stale.
func withDriverLocation(req *http.Request, d Deps, t *trips.Trip) {
	if t == nil || t.DriverID == nil || d.Presence == nil || trips.IsTerminal(t.Status) {
		return
	}
	if p, err := d.Presence.Get(req.Context(), *t.DriverID); err == nil && p != nil {
		t.DriverLocation = &trips.DriverLocation{Location: p.Location, Heading: p.Heading, At: p.At}
	}
}
