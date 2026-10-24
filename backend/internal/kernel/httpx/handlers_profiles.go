package httpx

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/kalen-limited/hauler-rider/backend/internal/approvals"
	"github.com/kalen-limited/hauler-rider/backend/internal/approvals/actions"
	"github.com/kalen-limited/hauler-rider/backend/internal/identity"
	"github.com/kalen-limited/hauler-rider/backend/internal/profiles"
)

// mountProfiles registers pricing profiles, payees and trip pricing records.
// Reads are open to staff; writes need fees.edit and country-level access.
func mountProfiles(r chi.Router, d Deps) {
	if d.Profiles == nil {
		return
	}
	h := profileHandlers{deps: d}
	r.Group(func(g chi.Router) {
		g.Use(requireAuth(d), requireStaff())
		g.With(scopeGuard).Get("/admin/pricing/profiles", h.list)
		g.Get("/admin/pricing/profiles/{id}", h.get)
		g.With(scopeGuard).Get("/admin/pricing/payees", h.payees)
		g.Get("/admin/trips/{id}/pricing-record", h.record)
		w := g.With(requirePermission(identity.PermFeesEdit))
		w.Post("/admin/pricing/profiles", h.create)
		w.Patch("/admin/pricing/profiles/{id}", h.update)
		w.Post("/admin/pricing/profiles/{id}/versions", h.publish)
		w.Post("/admin/pricing/profiles/{id}/status", h.status)
		w.Post("/admin/pricing/profiles/{id}/assignments", h.assign)
		w.Delete("/admin/pricing/profiles/{id}/assignments/{assignmentID}", h.unassign)
		w.Post("/admin/pricing/profiles/{id}/make-default", h.makeDefault)
		w.Post("/admin/pricing/payees", h.createPayee)
		w.Patch("/admin/pricing/payees/{id}", h.updatePayee)
	})
}

type profileHandlers struct{ deps Deps }

func (h profileHandlers) list(w http.ResponseWriter, req *http.Request) {
	list, err := h.deps.Profiles.List(req.Context(), scopeFrom(req, h.deps).Country)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": list})
}

// profileIn loads a profile and refuses one outside the staff member's countries.
func (h profileHandlers) profileIn(w http.ResponseWriter, req *http.Request) (*profiles.Profile, bool) {
	id := chi.URLParam(req, "id")
	if _, err := uuid.Parse(id); err != nil {
		writeError(w, req, profiles.ErrProfileNotFound, nil)
		return nil, false
	}
	p, err := h.deps.Profiles.Get(req.Context(), id)
	if err != nil {
		writeDomainError(w, req, err)
		return nil, false
	}
	if !allowedScope(claimsFrom(req.Context()), p.Country, "") {
		writeError(w, req, ErrScopeForbidden, nil)
		return nil, false
	}
	return p, true
}

func (h profileHandlers) get(w http.ResponseWriter, req *http.Request) {
	if p, ok := h.profileIn(w, req); ok {
		writeJSON(w, http.StatusOK, p)
	}
}

func (h profileHandlers) create(w http.ResponseWriter, req *http.Request) {
	var body profiles.CreateInput
	if !decode(w, req, &body) {
		return
	}
	if !allowedScope(claimsFrom(req.Context()), strings.ToUpper(body.Country), "") {
		writeError(w, req, ErrScopeForbidden, nil)
		return
	}
	p, err := h.deps.Profiles.Create(req.Context(), claimsFrom(req.Context()).Subject, body)
	respond(w, req, http.StatusCreated, p, err)
}

func (h profileHandlers) update(w http.ResponseWriter, req *http.Request) {
	var body profiles.UpdateInput
	if !decode(w, req, &body) {
		return
	}
	if _, ok := h.profileIn(w, req); !ok {
		return
	}
	p, err := h.deps.Profiles.Update(req.Context(), claimsFrom(req.Context()).Subject, chi.URLParam(req, "id"), body)
	respond(w, req, http.StatusOK, p, err)
}

func (h profileHandlers) publish(w http.ResponseWriter, req *http.Request) {
	var body profiles.VersionInput
	if !decode(w, req, &body) {
		return
	}
	p, ok := h.profileIn(w, req)
	if !ok || sentForApproval(w, req, h.deps, approvals.Submission{Action: actions.ProfilePublish, Reason: body.Reason,
		Country: p.Country, Target: "pricing_profile:" + p.ID, Payload: actions.ProfileVersionPayload{ProfileID: p.ID, Version: body}}) {
		return
	}
	v, err := h.deps.Profiles.Publish(req.Context(), claimsFrom(req.Context()).Subject, chi.URLParam(req, "id"), body)
	respond(w, req, http.StatusCreated, v, err)
}

type statusBody struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
}

func (h profileHandlers) status(w http.ResponseWriter, req *http.Request) {
	var body statusBody
	if !decode(w, req, &body) {
		return
	}
	cur, ok := h.profileIn(w, req)
	if !ok || sentForApproval(w, req, h.deps, approvals.Submission{Action: actions.ProfileStatus, Reason: body.Reason,
		Country: cur.Country, Target: "pricing_profile:" + cur.ID, Payload: actions.ProfileStatusPayload{ProfileID: cur.ID, Status: body.Status}}) {
		return
	}
	p, err := h.deps.Profiles.SetStatus(req.Context(), claimsFrom(req.Context()).Subject, chi.URLParam(req, "id"), body.Status, body.Reason)
	respond(w, req, http.StatusOK, p, err)
}

func (h profileHandlers) assign(w http.ResponseWriter, req *http.Request) {
	var body profiles.AssignInput
	if !decode(w, req, &body) {
		return
	}
	p, ok := h.profileIn(w, req)
	if !ok || sentForApproval(w, req, h.deps, approvals.Submission{Action: actions.ProfileAssign, Reason: body.Reason,
		Country: p.Country, Target: "pricing_profile:" + p.ID, Payload: actions.ProfileAssignPayload{ProfileID: p.ID, Assign: body}}) {
		return
	}
	sc, err := h.deps.Profiles.Assign(req.Context(), claimsFrom(req.Context()).Subject, chi.URLParam(req, "id"), body)
	respond(w, req, http.StatusCreated, sc, err)
}

func (h profileHandlers) unassign(w http.ResponseWriter, req *http.Request) {
	if _, ok := h.profileIn(w, req); !ok {
		return
	}
	err := h.deps.Profiles.Unassign(req.Context(), claimsFrom(req.Context()).Subject, chi.URLParam(req, "id"), chi.URLParam(req, "assignmentID"), req.URL.Query().Get("reason"))
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h profileHandlers) makeDefault(w http.ResponseWriter, req *http.Request) {
	var body reasonBody
	if !decode(w, req, &body) {
		return
	}
	cur, ok := h.profileIn(w, req)
	if !ok || sentForApproval(w, req, h.deps, approvals.Submission{Action: actions.ProfileDefault, Reason: body.Reason,
		Country: cur.Country, Target: "pricing_profile:" + cur.ID, Payload: actions.ProfileDefaultPayload{ProfileID: cur.ID}}) {
		return
	}
	p, err := h.deps.Profiles.MakeDefault(req.Context(), claimsFrom(req.Context()).Subject, chi.URLParam(req, "id"), body.Reason)
	respond(w, req, http.StatusOK, p, err)
}

func (h profileHandlers) payees(w http.ResponseWriter, req *http.Request) {
	list, err := h.deps.Profiles.Payees(req.Context(), scopeFrom(req, h.deps).Country)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": list})
}

func (h profileHandlers) createPayee(w http.ResponseWriter, req *http.Request) {
	var body profiles.PayeeInput
	if !decode(w, req, &body) {
		return
	}
	if !allowedScope(claimsFrom(req.Context()), strings.ToUpper(body.Country), "") {
		writeError(w, req, ErrScopeForbidden, nil)
		return
	}
	p, err := h.deps.Profiles.CreatePayee(req.Context(), claimsFrom(req.Context()).Subject, body)
	respond(w, req, http.StatusCreated, p, err)
}

func (h profileHandlers) updatePayee(w http.ResponseWriter, req *http.Request) {
	var body profiles.PayeeInput
	if !decode(w, req, &body) {
		return
	}
	claims := claimsFrom(req.Context())
	allowed := func(country string) bool { return allowedScope(claims, country, "") }
	p, err := h.deps.Profiles.UpdatePayee(req.Context(), claims.Subject, chi.URLParam(req, "id"), body, allowed)
	respond(w, req, http.StatusOK, p, err)
}

// recordCacheS: a final record never changes, so it may be cached for a day.
const recordCacheS = "private, max-age=86400, immutable"

func (h profileHandlers) record(w http.ResponseWriter, req *http.Request) {
	id := chi.URLParam(req, "id")
	if _, err := uuid.Parse(id); err != nil {
		writeError(w, req, profiles.ErrRecordNotFound, nil)
		return
	}
	r, err := h.deps.Profiles.Record(req.Context(), id)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	country := "" // a record without a country is visible only to staff with no country limit
	if r.Country != nil {
		country = *r.Country
	}
	if !allowedScope(claimsFrom(req.Context()), country, r.AreaID) {
		writeError(w, req, ErrScopeForbidden, nil)
		return
	}
	if r.RecordHash != nil {
		etag := `"` + *r.RecordHash + `"`
		w.Header().Set("ETag", etag)
		w.Header().Set("Cache-Control", recordCacheS)
		if req.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}
	writeJSON(w, http.StatusOK, r)
}

// respond writes v with status, or the domain error.
func respond(w http.ResponseWriter, req *http.Request, status int, v any, err error) {
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, status, v)
}
