package httpx

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/kalen-limited/hauler-rider/backend/internal/approvals"
	"github.com/kalen-limited/hauler-rider/backend/internal/identity"
)

// mountApprovals registers the approvals inbox (every staff member: what they
// asked for and what they may decide, inside their countries) and the
// workflow editor (approvals.manage).
func mountApprovals(r chi.Router, d Deps) {
	if d.Approvals == nil {
		return
	}
	h := approvalHandlers{deps: d}
	r.Group(func(g chi.Router) {
		g.Use(requireAuth(d), requireStaff())
		g.Get("/admin/approvals/requests", h.list)
		g.Get("/admin/approvals/requests/summary", h.summary)
		g.Get("/admin/approvals/requests/{id}", h.get)
		g.Post("/admin/approvals/requests/{id}/approve", h.approve)
		g.Post("/admin/approvals/requests/{id}/reject", h.reject)
		g.Post("/admin/approvals/requests/{id}/cancel", h.cancel)
		m := g.With(requirePermission(identity.PermApprovalsManage))
		m.Get("/admin/approvals/workflows", h.workflows)
		m.Get("/admin/approvals/workflows/{action}", h.workflow)
		m.Put("/admin/approvals/workflows/{action}", h.saveWorkflow)
	})
}

type approvalHandlers struct{ deps Deps }

func (h approvalHandlers) list(w http.ResponseWriter, req *http.Request) {
	list, err := h.deps.Approvals.List(req.Context(), approvalViewer(req), req.URL.Query().Get("box"), queryInt(req, "limit"))
	respond(w, req, http.StatusOK, map[string]any{"data": list}, err)
}

// summary is the nav badge: pending requests the caller could decide now.
func (h approvalHandlers) summary(w http.ResponseWriter, req *http.Request) {
	n, err := h.deps.Approvals.Actionable(req.Context(), approvalViewer(req))
	respond(w, req, http.StatusOK, map[string]any{"actionable": n}, err)
}

// requestID reads the {id} path parameter; a malformed id is not found.
func requestID(w http.ResponseWriter, req *http.Request) (string, bool) {
	id := chi.URLParam(req, "id")
	if _, err := uuid.Parse(id); err != nil {
		writeError(w, req, approvals.ErrNotFound, nil)
		return "", false
	}
	return id, true
}

func (h approvalHandlers) get(w http.ResponseWriter, req *http.Request) {
	if id, ok := requestID(w, req); ok {
		r, err := h.deps.Approvals.Get(req.Context(), approvalViewer(req), id)
		respond(w, req, http.StatusOK, r, err)
	}
}

// approvalNoteBody is an approver's note (required to reject).
type approvalNoteBody struct {
	Note string `json:"note"`
}

func (h approvalHandlers) approve(w http.ResponseWriter, req *http.Request) {
	var body approvalNoteBody
	id, ok := requestID(w, req)
	if !ok || !decode(w, req, &body) {
		return
	}
	r, err := h.deps.Approvals.Approve(req.Context(), approvalViewer(req), id, body.Note)
	respond(w, req, http.StatusOK, r, err)
}

func (h approvalHandlers) reject(w http.ResponseWriter, req *http.Request) {
	var body approvalNoteBody
	id, ok := requestID(w, req)
	if !ok || !decode(w, req, &body) {
		return
	}
	r, err := h.deps.Approvals.Reject(req.Context(), approvalViewer(req), id, body.Note)
	respond(w, req, http.StatusOK, r, err)
}

func (h approvalHandlers) cancel(w http.ResponseWriter, req *http.Request) {
	var body approvalNoteBody
	id, ok := requestID(w, req)
	if !ok || !decode(w, req, &body) {
		return
	}
	r, err := h.deps.Approvals.Cancel(req.Context(), approvalViewer(req), id, body.Note)
	respond(w, req, http.StatusOK, r, err)
}

// workflowsView is the editor's opening data: every action, and what steps may name.
type workflowsView struct {
	Data        []approvals.ActionWorkflow `json:"data"`
	Roles       []string                   `json:"roles"`
	Permissions []identity.Permission      `json:"permissions"`
}

func (h approvalHandlers) workflows(w http.ResponseWriter, req *http.Request) {
	list, err := h.deps.Approvals.Workflows(req.Context())
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	roles, err := h.deps.Identity.RoleNames(req.Context())
	respond(w, req, http.StatusOK, workflowsView{Data: list, Roles: roles, Permissions: identity.Permissions}, err)
}

func (h approvalHandlers) workflow(w http.ResponseWriter, req *http.Request) {
	wf, err := h.deps.Approvals.WorkflowOf(req.Context(), chi.URLParam(req, "action"))
	respond(w, req, http.StatusOK, wf, err)
}

func (h approvalHandlers) saveWorkflow(w http.ResponseWriter, req *http.Request) {
	var body approvals.WorkflowInput
	if !decode(w, req, &body) {
		return
	}
	wf, err := h.deps.Approvals.SaveWorkflow(req.Context(), claimsFrom(req.Context()).Subject, chi.URLParam(req, "action"), body)
	respond(w, req, http.StatusOK, wf, err)
}
