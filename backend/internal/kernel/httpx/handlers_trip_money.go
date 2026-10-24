package httpx

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/identity"
)

// mountTripMoney registers the staff fare actions on a trip: set the price
// before payment (trips.price) and refund after completion (trips.refund).
func mountTripMoney(r chi.Router, d Deps) {
	h := tripMoneyHandlers{deps: d}
	r.Group(func(g chi.Router) {
		g.Use(requireAuth(d), requireStaff())
		g.With(requirePermission(identity.PermTripsPrice)).Post("/admin/trips/{id}/price", h.setPrice)
		if d.Refunds != nil {
			g.Get("/admin/trips/{id}/refunds", h.refunds)
			g.With(requirePermission(identity.PermTripsRefund)).Post("/admin/trips/{id}/refunds", h.refund)
		}
	})
}

type tripMoneyHandlers struct{ deps Deps }

// tripInScope refuses a trip outside the staff member's countries and regions.
func tripInScope(w http.ResponseWriter, req *http.Request, d Deps) bool {
	t, err := d.Trips.AdminGet(req.Context(), chi.URLParam(req, "id"))
	if err != nil {
		writeDomainError(w, req, err)
		return false
	}
	area := ""
	if t.Area != nil {
		area = t.Area.ID
	}
	if !allowedScope(claimsFrom(req.Context()), t.Country, area) {
		writeError(w, req, ErrScopeForbidden, nil)
		return false
	}
	return true
}

type priceBody struct {
	Amount decimal.Decimal `json:"amount"`
	Reason string          `json:"reason"`
}

func (h tripMoneyHandlers) setPrice(w http.ResponseWriter, req *http.Request) {
	var body priceBody
	if !decode(w, req, &body) || !tripInScope(w, req, h.deps) {
		return
	}
	t, err := h.deps.Trips.AdminSetPrice(req.Context(), claimsFrom(req.Context()).Subject, chi.URLParam(req, "id"), body.Amount, body.Reason)
	respond(w, req, http.StatusOK, t, err)
}

func (h tripMoneyHandlers) refunds(w http.ResponseWriter, req *http.Request) {
	if !tripInScope(w, req, h.deps) {
		return
	}
	r, err := h.deps.Refunds.ForTrip(req.Context(), chi.URLParam(req, "id"))
	respond(w, req, http.StatusOK, r, err)
}

type refundBody struct {
	Amount  decimal.Decimal `json:"amount"`
	BorneBy string          `json:"borne_by"`
	Reason  string          `json:"reason"`
}

func (h tripMoneyHandlers) refund(w http.ResponseWriter, req *http.Request) {
	var body refundBody
	if !decode(w, req, &body) || !tripInScope(w, req, h.deps) {
		return
	}
	r, err := h.deps.Refunds.Create(req.Context(), claimsFrom(req.Context()).Subject, chi.URLParam(req, "id"), body.Amount, body.BorneBy, body.Reason,
		req.Header.Get("Idempotency-Key"))
	respond(w, req, http.StatusCreated, r, err)
}
