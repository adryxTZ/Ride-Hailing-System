package httpx

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/kalen-limited/hauler-rider/backend/internal/fraud"
	"github.com/kalen-limited/hauler-rider/backend/internal/identity"
)

// mountFraud registers the fraud review queue: staff read it inside their
// countries; reviewing needs fraud.review.
func mountFraud(r chi.Router, d Deps) {
	if d.Fraud == nil {
		return
	}
	h := fraudHandlers{deps: d}
	r.Group(func(g chi.Router) {
		g.Use(requireAuth(d), requireStaff())
		g.With(scopeGuard).Get("/admin/fraud/flags", h.list)
		g.Get("/admin/fraud/flags/{id}", h.get)
		g.With(requirePermission(identity.PermFraudReview)).Post("/admin/fraud/flags/{id}/review", h.review)
	})
}

type fraudHandlers struct{ deps Deps }

func (h fraudHandlers) list(w http.ResponseWriter, req *http.Request) {
	q := req.URL.Query()
	f := fraud.Filter{Status: strings.ToUpper(q.Get("status")), Rule: strings.ToUpper(q.Get("rule")), SubjectKind: strings.ToUpper(q.Get("subject_kind")),
		SubjectID: q.Get("subject_id"), Country: scopeFrom(req, h.deps).Country, Limit: queryInt(req, "limit"), Before: parseTime(q.Get("cursor"))}
	list, next, err := h.deps.Fraud.List(req.Context(), f)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	cursor := ""
	if next != nil {
		cursor = next.Format(time.RFC3339Nano)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": list, "next_cursor": cursor})
}

// flagIn loads a flag and refuses one outside the staff member's countries.
func (h fraudHandlers) flagIn(w http.ResponseWriter, req *http.Request) (*fraud.Flag, bool) {
	f, err := h.deps.Fraud.Get(req.Context(), chi.URLParam(req, "id"))
	if err != nil {
		writeDomainError(w, req, err)
		return nil, false
	}
	country := ""
	if f.Country != nil {
		country = *f.Country
	}
	if !allowedScope(claimsFrom(req.Context()), country, "") {
		writeError(w, req, ErrScopeForbidden, nil)
		return nil, false
	}
	return f, true
}

func (h fraudHandlers) get(w http.ResponseWriter, req *http.Request) {
	if f, ok := h.flagIn(w, req); ok {
		writeJSON(w, http.StatusOK, f)
	}
}

type reviewBody struct {
	Decision string `json:"decision"`
	Note     string `json:"note"`
}

func (h fraudHandlers) review(w http.ResponseWriter, req *http.Request) {
	var body reviewBody
	if !decode(w, req, &body) {
		return
	}
	if _, ok := h.flagIn(w, req); !ok {
		return
	}
	f, err := h.deps.Fraud.Review(req.Context(), claimsFrom(req.Context()).Subject, chi.URLParam(req, "id"), strings.ToUpper(body.Decision), body.Note)
	respond(w, req, http.StatusOK, f, err)
}
