package httpx

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/approvals"
	"github.com/kalen-limited/hauler-rider/backend/internal/approvals/actions"
	"github.com/kalen-limited/hauler-rider/backend/internal/identity"
	"github.com/kalen-limited/hauler-rider/backend/internal/trips"
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
	_, ok := scopedTrip(w, req, d)
	return ok
}

// scopedTrip is tripInScope that also returns the trip.
func scopedTrip(w http.ResponseWriter, req *http.Request, d Deps) (*trips.TripDetail, bool) {
	t, err := d.Trips.AdminGet(req.Context(), chi.URLParam(req, "id"))
	if err != nil {
		writeDomainError(w, req, err)
		return nil, false
	}
	area := ""
	if t.Area != nil {
		area = t.Area.ID
	}
	if !allowedScope(claimsFrom(req.Context()), t.Country, area) {
		writeError(w, req, ErrScopeForbidden, nil)
		return nil, false
	}
	return t, true
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
	if !decode(w, req, &body) {
		return
	}
	t, ok := scopedTrip(w, req, h.deps)
	if !ok || sentForApproval(w, req, h.deps, approvals.Submission{Action: actions.TripRefund, Reason: body.Reason, Country: t.Country,
		Amount: &body.Amount, Target: "trip:" + t.ID, Payload: actions.RefundPayload{TripID: t.ID, Amount: body.Amount, BorneBy: body.BorneBy}}) {
		return
	}
	r, err := h.deps.Refunds.Create(req.Context(), claimsFrom(req.Context()).Subject, chi.URLParam(req, "id"), body.Amount, body.BorneBy, body.Reason,
		req.Header.Get("Idempotency-Key"))
	respond(w, req, http.StatusCreated, r, err)
}
