package httpx

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/kalen-limited/hauler-rider/backend/internal/identity"
)

// mountAdminStaff registers roles (named sets of permissions) and the staff
// user page: details, sign-in sessions, ending sessions, deactivating, and
// limiting a person to countries or regions. All need the staff.manage permission.
func mountAdminStaff(r chi.Router, d Deps) {
	h := adminStaffHandlers{deps: d}
	r.Group(func(g chi.Router) {
		g.Use(requireAuth(d), requireStaff(), requirePermission(identity.PermStaffManage))
		g.Get("/admin/roles", h.roles)
		g.Post("/admin/roles", h.createRole)
		g.Put("/admin/roles/{id}", h.updateRole)
		g.Delete("/admin/roles/{id}", h.deleteRole)
		g.Get("/admin/staff/{id}", h.detail)
		g.Post("/admin/staff/{id}/sessions/end", h.endSessions)
		g.Post("/admin/staff/{id}/deactivate", h.setStatus("DISABLED"))
		g.Post("/admin/staff/{id}/reactivate", h.setStatus("ACTIVE"))
		g.Put("/admin/staff/{id}/access", h.setAccess)
	})
}

type adminStaffHandlers struct{ deps Deps }

func (h adminStaffHandlers) roles(w http.ResponseWriter, req *http.Request) {
	list, err := h.deps.Identity.ListRoles(req.Context())
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": list, "permissions": identity.Permissions})
}

func (h adminStaffHandlers) createRole(w http.ResponseWriter, req *http.Request) {
	var body identity.RoleInput
	if !decode(w, req, &body) {
		return
	}
	r, err := h.deps.Identity.CreateRole(req.Context(), claimsFrom(req.Context()).Subject, body)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusCreated, r)
}

func (h adminStaffHandlers) updateRole(w http.ResponseWriter, req *http.Request) {
	var body identity.RoleInput
	if !decode(w, req, &body) {
		return
	}
	r, err := h.deps.Identity.UpdateRole(req.Context(), claimsFrom(req.Context()).Subject, chi.URLParam(req, "id"), body)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, r)
}

func (h adminStaffHandlers) deleteRole(w http.ResponseWriter, req *http.Request) {
	if err := h.deps.Identity.DeleteRole(req.Context(), claimsFrom(req.Context()).Subject, chi.URLParam(req, "id"), req.URL.Query().Get("reason")); err != nil {
		writeDomainError(w, req, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h adminStaffHandlers) detail(w http.ResponseWriter, req *http.Request) {
	d, err := h.deps.Identity.StaffDetail(req.Context(), chi.URLParam(req, "id"))
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

// endSessionsBody ends one session (session_id) or every open one (empty).
type endSessionsBody struct {
	SessionID string `json:"session_id"`
	Reason    string `json:"reason"`
}

func (h adminStaffHandlers) endSessions(w http.ResponseWriter, req *http.Request) {
	var body endSessionsBody
	if !decode(w, req, &body) {
		return
	}
	userID := chi.URLParam(req, "id")
	if err := h.deps.Identity.EndSessions(req.Context(), claimsFrom(req.Context()).Subject, userID, body.SessionID, body.Reason); err != nil {
		writeDomainError(w, req, err)
		return
	}
	gates.forget(userID)
	w.WriteHeader(http.StatusNoContent)
}

func (h adminStaffHandlers) setStatus(status string) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		var body reasonBody
		if !decode(w, req, &body) {
			return
		}
		userID, actor := chi.URLParam(req, "id"), claimsFrom(req.Context()).Subject
		if userID == actor { // nobody locks themselves out
			writeError(w, req, identity.ErrNotYourself, nil)
			return
		}
		if err := h.deps.Identity.SetStatus(req.Context(), actor, userID, status, body.Reason); err != nil {
			writeDomainError(w, req, err)
			return
		}
		gates.forget(userID)
		w.WriteHeader(http.StatusNoContent)
	}
}

// accessBody limits a person to countries/regions (empty = everywhere).
type accessBody struct {
	Access []identity.Access `json:"access"`
	Reason string            `json:"reason"`
}

func (h adminStaffHandlers) setAccess(w http.ResponseWriter, req *http.Request) {
	var body accessBody
	if !decode(w, req, &body) {
		return
	}
	if err := h.deps.Identity.SetStaffAccess(req.Context(), claimsFrom(req.Context()).Subject, chi.URLParam(req, "id"), body.Access, body.Reason); err != nil {
		writeDomainError(w, req, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
