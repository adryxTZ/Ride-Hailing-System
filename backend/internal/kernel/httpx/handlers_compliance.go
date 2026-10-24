package httpx

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/kalen-limited/hauler-rider/backend/internal/approvals"
	"github.com/kalen-limited/hauler-rider/backend/internal/approvals/actions"
	"github.com/kalen-limited/hauler-rider/backend/internal/compliance"
	"github.com/kalen-limited/hauler-rider/backend/internal/identity"
)

// mountCompliance registers the regulator pages: each regulated country's
// switches and figures, the submissions outbox, and a manual retry (audited).
func mountCompliance(r chi.Router, d Deps) {
	if d.Compliance == nil {
		return
	}
	h := complianceHandlers{deps: d}
	r.Group(func(g chi.Router) {
		g.Use(requireAuth(d), requireStaff(), scopeGuard)
		read := requirePermission(identity.PermComplianceManage)
		g.With(read).Get("/admin/compliance/regulator", h.status)
		g.With(read).Patch("/admin/compliance/regulator/{country}", h.settings)
		g.With(read).Get("/admin/compliance/submissions", h.list)
		g.With(read).Post("/admin/compliance/submissions/{id}/retry", h.retry)
	})
}

type complianceHandlers struct{ deps Deps }

func (h complianceHandlers) status(w http.ResponseWriter, req *http.Request) {
	list, err := h.deps.Compliance.Statuses(req.Context(), strings.ToUpper(req.URL.Query().Get("country")))
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": list})
}

type settingsBody struct {
	Enabled    *bool  `json:"enabled"`
	FailClosed *bool  `json:"fail_closed"`
	Reason     string `json:"reason"`
}

// settings changes one country's switches; only staff whose scope covers the country.
func (h complianceHandlers) settings(w http.ResponseWriter, req *http.Request) {
	var body settingsBody
	if !decode(w, req, &body) {
		return
	}
	country := strings.ToUpper(chi.URLParam(req, "country"))
	if !allowedScope(claimsFrom(req.Context()), country, "") {
		writeError(w, req, ErrScopeForbidden, nil)
		return
	}
	if sentForApproval(w, req, h.deps, approvals.Submission{Action: actions.RegulatorSettings, Reason: body.Reason, Country: country,
		Target: "regulator:" + country, Payload: actions.RegulatorPayload{Country: country, Enabled: body.Enabled, FailClosed: body.FailClosed}}) {
		return
	}
	st, err := h.deps.Compliance.SetSettings(req.Context(), claimsFrom(req.Context()).Subject, country, body.Enabled, body.FailClosed, body.Reason)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (h complianceHandlers) list(w http.ResponseWriter, req *http.Request) {
	q := req.URL.Query()
	f := compliance.Filter{Country: strings.ToUpper(q.Get("country")), Kind: q.Get("kind"), Status: q.Get("status"),
		Q: strings.TrimSpace(q.Get("q")), Stuck: q.Get("stuck") == "true", Limit: queryInt(req, "limit")}
	list, err := h.deps.Compliance.List(req.Context(), f)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": list})
}

func (h complianceHandlers) retry(w http.ResponseWriter, req *http.Request) {
	var body reasonBody
	if !decode(w, req, &body) {
		return
	}
	sub, err := h.deps.Compliance.Retry(req.Context(), claimsFrom(req.Context()).Subject, chi.URLParam(req, "id"), body.Reason)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, sub)
}
