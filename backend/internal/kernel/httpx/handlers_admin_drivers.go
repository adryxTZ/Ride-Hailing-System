package httpx

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/kalen-limited/hauler-rider/backend/internal/identity"
	"github.com/kalen-limited/hauler-rider/backend/internal/trips"
)

// mountAdminDrivers registers the staff review of drivers, vehicles and documents.
func mountAdminDrivers(r chi.Router, d Deps) {
	if d.Fleet == nil {
		return
	}
	h := adminDriverHandlers{deps: d}
	r.Group(func(g chi.Router) {
		g.Use(requireAuth(d), requireStaff(), scopeGuard)
		g.Get("/admin/drivers", h.list)
		g.Get("/admin/drivers/{id}", h.detail)
		review := requirePermission(identity.PermDriversReview)
		g.With(review).Post("/admin/drivers/{id}/approve", h.approve)
		g.With(review).Post("/admin/drivers/{id}/reject", h.decide("reject"))
		g.With(review).Post("/admin/drivers/{id}/suspend", h.decide("suspend"))
		g.With(review).Post("/admin/drivers/{id}/reinstate", h.decide("reinstate"))
		g.With(review).Post("/admin/drivers/{id}/documents/{docId}/review", h.reviewDocument)
		g.Get("/admin/audit", h.audit)
	})
}

type adminDriverHandlers struct{ deps Deps }

// reasonBody carries the reason every non-approval decision needs.
type reasonBody struct {
	Reason string `json:"reason"`
}

func queryInt(req *http.Request, key string) int {
	n, _ := strconv.Atoi(req.URL.Query().Get(key))
	return n
}

func (h adminDriverHandlers) list(w http.ResponseWriter, req *http.Request) {
	q := req.URL.Query()
	page, err := h.deps.Fleet.ListDrivers(req.Context(), q.Get("status"), q.Get("q"), scopeFrom(req, h.deps), queryInt(req, "limit"), queryInt(req, "offset"))
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (h adminDriverHandlers) detail(w http.ResponseWriter, req *http.Request) {
	if h.deps.People != nil {
		if err := h.deps.People.Check(req.Context(), trips.AsDriver, chi.URLParam(req, "id"), allowed(req)); err != nil {
			writeDomainError(w, req, err)
			return
		}
	}
	d, err := h.deps.Fleet.Detail(req.Context(), chi.URLParam(req, "id"))
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (h adminDriverHandlers) approve(w http.ResponseWriter, req *http.Request) {
	d, err := h.deps.Fleet.Approve(req.Context(), claimsFrom(req.Context()).Subject, chi.URLParam(req, "id"))
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (h adminDriverHandlers) decide(action string) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		var body reasonBody
		if !decode(w, req, &body) {
			return
		}
		staff, id := claimsFrom(req.Context()).Subject, chi.URLParam(req, "id")
		f := h.deps.Fleet
		decisions := map[string]func() (any, error){
			"reject":    func() (any, error) { return f.Reject(req.Context(), staff, id, body.Reason) },
			"suspend":   func() (any, error) { return f.Suspend(req.Context(), staff, id, body.Reason) },
			"reinstate": func() (any, error) { return f.Reinstate(req.Context(), staff, id, body.Reason) },
		}
		d, err := decisions[action]()
		if err != nil {
			writeDomainError(w, req, err)
			return
		}
		writeJSON(w, http.StatusOK, d)
	}
}

func (h adminDriverHandlers) reviewDocument(w http.ResponseWriter, req *http.Request) {
	var body struct {
		Approve bool   `json:"approve"`
		Reason  string `json:"reason"`
	}
	if !decode(w, req, &body) {
		return
	}
	d, err := h.deps.Fleet.ReviewDocument(req.Context(), claimsFrom(req.Context()).Subject,
		chi.URLParam(req, "id"), chi.URLParam(req, "docId"), body.Approve, body.Reason)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (h adminDriverHandlers) audit(w http.ResponseWriter, req *http.Request) {
	q := req.URL.Query()
	list, err := h.deps.Fleet.Audit(req.Context(), q.Get("target"), q.Get("action"), q.Get("actor"), queryInt(req, "limit"))
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": list})
}
