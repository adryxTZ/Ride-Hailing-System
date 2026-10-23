package httpx

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/kalen-limited/hauler-rider/backend/internal/messaging"
)

// mountMessaging registers chat, ratings and push devices. Rider and driver use
// the same trip routes; the service decides who is who from the trip.
func mountMessaging(r chi.Router, d Deps) {
	if d.Messaging == nil {
		return
	}
	h := messagingHandlers{deps: d}
	r.Group(func(g chi.Router) {
		g.Use(requireAuth(d))
		g.Get("/trips/{id}/messages", h.listMessages)
		g.Post("/trips/{id}/messages", h.send)
		g.Get("/trips/{id}/ratings", h.ratings)
		g.Post("/trips/{id}/rating", h.rate)
		g.Get("/me/rating", h.myRating)
		g.Post("/me/devices", h.registerDevice)
		g.Delete("/me/devices/{token}", h.unregisterDevice)
	})
}

type messagingHandlers struct{ deps Deps }

// chatPage is the chat plus the canned replies for the caller's side.
type chatPage struct {
	Data         []messaging.Message    `json:"data"`
	QuickReplies []messaging.QuickReply `json:"quick_replies"`
}

func (h messagingHandlers) listMessages(w http.ResponseWriter, req *http.Request) {
	c := claimsFrom(req.Context())
	list, err := h.deps.Messaging.List(req.Context(), chi.URLParam(req, "id"), c.Subject)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	role := messaging.RoleRider
	if strings.EqualFold(c.App, messaging.RoleDriver) {
		role = messaging.RoleDriver
	}
	writeJSON(w, http.StatusOK, chatPage{Data: list, QuickReplies: messaging.QuickReplies(role)})
}

func (h messagingHandlers) send(w http.ResponseWriter, req *http.Request) {
	var body messaging.SendInput
	if !decode(w, req, &body) {
		return
	}
	m, err := h.deps.Messaging.Send(req.Context(), chi.URLParam(req, "id"), claimsFrom(req.Context()).Subject, body)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusCreated, m)
}

func (h messagingHandlers) ratings(w http.ResponseWriter, req *http.Request) {
	list, err := h.deps.Messaging.Ratings(req.Context(), chi.URLParam(req, "id"), claimsFrom(req.Context()).Subject)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, listResponse[messaging.Rating]{Data: list})
}

func (h messagingHandlers) rate(w http.ResponseWriter, req *http.Request) {
	var body messaging.RateInput
	if !decode(w, req, &body) {
		return
	}
	r, err := h.deps.Messaging.Rate(req.Context(), chi.URLParam(req, "id"), claimsFrom(req.Context()).Subject, body)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusCreated, r)
}

func (h messagingHandlers) myRating(w http.ResponseWriter, req *http.Request) {
	s, err := h.deps.Messaging.Summary(req.Context(), claimsFrom(req.Context()).Subject)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, s)
}

func (h messagingHandlers) registerDevice(w http.ResponseWriter, req *http.Request) {
	var body messaging.DeviceInput
	if !decode(w, req, &body) {
		return
	}
	if err := h.deps.Messaging.RegisterDevice(req.Context(), claimsFrom(req.Context()).Subject, body); err != nil {
		writeDomainError(w, req, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h messagingHandlers) unregisterDevice(w http.ResponseWriter, req *http.Request) {
	if err := h.deps.Messaging.UnregisterDevice(req.Context(), claimsFrom(req.Context()).Subject, chi.URLParam(req, "token")); err != nil {
		writeDomainError(w, req, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
