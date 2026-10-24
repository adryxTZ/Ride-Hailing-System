package httpx

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/approvals"
	"github.com/kalen-limited/hauler-rider/backend/internal/approvals/actions"
	"github.com/kalen-limited/hauler-rider/backend/internal/identity"
	"github.com/kalen-limited/hauler-rider/backend/internal/payouts"
)

// mountPayouts registers cash-outs: drivers ask for and follow their payouts;
// Finance lists them and approves or rejects each request (audited).
func mountPayouts(r chi.Router, d Deps) {
	if d.Payouts == nil {
		return
	}
	h := payoutHandlers{deps: d}
	r.Group(func(g chi.Router) {
		g.Use(requireAuth(d), requireApp("driver"))
		g.Post("/driver/payouts", h.create)
		g.Get("/driver/payouts", h.list)
		g.Get("/driver/payouts/{id}", h.get)
		g.Post("/driver/payouts/{id}/cancel", h.cancel)
	})
	r.Group(func(g chi.Router) {
		g.Use(requireAuth(d), requireStaff(), scopeGuard)
		g.With(requirePermission(identity.PermFinanceView)).Get("/admin/finance/payouts", h.adminList)
		g.With(requirePermission(identity.PermPayoutsApprove)).Post("/admin/finance/payouts/{id}/approve", h.decide(true))
		g.With(requirePermission(identity.PermPayoutsApprove)).Post("/admin/finance/payouts/{id}/reject", h.decide(false))
	})
}

type payoutHandlers struct{ deps Deps }

type payoutRequest struct {
	Amount decimal.Decimal `json:"amount"`
}

// create asks for a cash-out; Idempotency-Key replays return the original (200).
func (h payoutHandlers) create(w http.ResponseWriter, req *http.Request) {
	var body payoutRequest
	if !decode(w, req, &body) {
		return
	}
	p, replayed, err := h.deps.Payouts.Create(req.Context(), claimsFrom(req.Context()).Subject, req.Header.Get(idempotencyHeader), body.Amount)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	status := http.StatusCreated
	if replayed {
		status = http.StatusOK
	}
	writeJSON(w, status, p)
}

func (h payoutHandlers) list(w http.ResponseWriter, req *http.Request) {
	list, err := h.deps.Payouts.List(req.Context(), claimsFrom(req.Context()).Subject)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, listResponse[payouts.Payout]{Data: list})
}

func (h payoutHandlers) get(w http.ResponseWriter, req *http.Request) {
	p, err := h.deps.Payouts.Get(req.Context(), claimsFrom(req.Context()).Subject, chi.URLParam(req, "id"))
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (h payoutHandlers) cancel(w http.ResponseWriter, req *http.Request) {
	p, err := h.deps.Payouts.Cancel(req.Context(), claimsFrom(req.Context()).Subject, chi.URLParam(req, "id"))
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// adminList is the payouts of the selected country's currency, open ones first.
func (h payoutHandlers) adminList(w http.ResponseWriter, req *http.Request) {
	cur := adminOpsHandlers(h).currency(scopeFrom(req, h.deps))
	list, err := h.deps.Payouts.AdminList(req.Context(), req.URL.Query().Get("status"), cur, queryInt(req, "limit"))
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": list, "enabled": h.deps.Payouts.Enabled()})
}

// decide approves or rejects a requested payout with a reason.
func (h payoutHandlers) decide(approve bool) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		var body reasonBody
		if !decode(w, req, &body) {
			return
		}
		staff, id := claimsFrom(req.Context()).Subject, chi.URLParam(req, "id")
		if !h.mayDecide(req, id) {
			writeError(w, req, ErrScopeForbidden, nil)
			return
		}
		if approve && h.sentForApproval(w, req, id, body.Reason) {
			return
		}
		var p *payouts.Payout
		var err error
		if approve {
			p, err = h.deps.Payouts.Approve(req.Context(), staff, id, body.Reason)
		} else {
			p, err = h.deps.Payouts.Reject(req.Context(), staff, id, body.Reason)
		}
		if err != nil {
			writeDomainError(w, req, err)
			return
		}
		writeJSON(w, http.StatusOK, p)
	}
}

// sentForApproval offers a payout approval to its workflow (amount condition).
// The request belongs to the first country of the payout's currency that the
// caller may work in. An unknown payout is left to the service (404).
func (h payoutHandlers) sentForApproval(w http.ResponseWriter, req *http.Request, id, reason string) bool {
	if h.deps.Approvals == nil || !h.deps.Payouts.Enabled() || !h.deps.Approvals.Enabled(req.Context(), actions.PayoutApprove) {
		return false // without a gateway the service answers PAYOUTS_UNAVAILABLE as before
	}
	p, err := h.deps.Payouts.AdminGet(req.Context(), id)
	if err != nil {
		return false
	}
	country, c := "", claimsFrom(req.Context())
	for _, bc := range h.deps.Brand.Countries {
		if bc.Currency == p.Currency && allowedScope(c, bc.Code, "") {
			country = bc.Code
			break
		}
	}
	return sentForApproval(w, req, h.deps, approvals.Submission{Action: actions.PayoutApprove, Reason: reason, Country: country,
		Amount: &p.Amount, Target: "payout:" + p.ID, Payload: actions.PayoutPayload{PayoutID: p.ID}})
}

// mayDecide reports whether the caller's country scope covers the payout: it is
// in a currency of a country they may work in (whole country; finance is per
// country). Unlimited staff pass; an unknown payout is left to the service (404).
func (h payoutHandlers) mayDecide(req *http.Request, id string) bool {
	p, err := h.deps.Payouts.AdminGet(req.Context(), id)
	if err != nil {
		return true
	}
	c := claimsFrom(req.Context())
	for _, country := range h.deps.Brand.Countries {
		if country.Currency == p.Currency && allowedScope(c, country.Code, "") {
			return true
		}
	}
	return false
}
