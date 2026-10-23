package httpx

import (
	"encoding/json"
	"github.com/kalen-limited/hauler-rider/backend/internal/identity"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
)

// Request bounds for pay-ins and gateway callbacks.
const (
	idempotencyHeader = "Idempotency-Key"
	webhookBodyLimit  = 64 << 10
	adminPayinsLimit  = 200
	adminHoldsLimit   = 500
)

// mountPayments registers debt pay-ins (driver app), the gateway webhook
// (public, untrusted) and the finance views of holds and pay-ins.
func mountPayments(r chi.Router, d Deps) {
	if d.Payments == nil || d.Holds == nil {
		return
	}
	h := paymentHandlers{deps: d}
	r.Group(func(g chi.Router) {
		g.Use(requireAuth(d), requireApp("driver"))
		g.Post("/driver/payins", h.createPayin)
		g.Get("/driver/payins/{id}", h.getPayin)
	})
	r.Post("/payments/webhooks/{gateway}", h.webhook)
	r.Group(func(g chi.Router) {
		g.Use(requireAuth(d), requireStaff(), scopeGuard)
		g.With(requirePermission(identity.PermFinanceView)).Get("/admin/finance/holds", h.listHolds)
		g.With(requirePermission(identity.PermFinanceView)).Get("/admin/finance/payins", h.listPayins)
		g.With(requirePermission(identity.PermHoldsManage)).Post("/admin/drivers/{id}/hold", h.placeHold)
		g.With(requirePermission(identity.PermHoldsManage)).Post("/admin/drivers/{id}/hold/release", h.releaseHold)
	})
}

type paymentHandlers struct{ deps Deps }

// payinRequest is the body of POST /driver/payins.
type payinRequest struct {
	Amount decimal.Decimal `json:"amount"`
	Phone  string          `json:"phone"` // defaults to the account phone
}

func (h paymentHandlers) createPayin(w http.ResponseWriter, req *http.Request) {
	var body payinRequest
	if !decode(w, req, &body) {
		return
	}
	driverID := claimsFrom(req.Context()).Subject
	phone := body.Phone
	if phone == "" {
		p, err := h.deps.Identity.Phone(req.Context(), driverID)
		if err != nil {
			writeDomainError(w, req, err)
			return
		}
		phone = p
	}
	p, replayed, err := h.deps.Payments.Create(req.Context(), driverID, req.Header.Get(idempotencyHeader), body.Amount, phone)
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

func (h paymentHandlers) getPayin(w http.ResponseWriter, req *http.Request) {
	p, err := h.deps.Payments.Get(req.Context(), claimsFrom(req.Context()).Subject, chi.URLParam(req, "id"))
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// webhook takes only our order reference from the callback and queues a
// re-check of that pay-in with the gateway. It answers 200 at once so the
// gateway does not retry; the payload never decides a status.
func (h paymentHandlers) webhook(w http.ResponseWriter, req *http.Request) {
	if !h.deps.Payments.KnowsGateway(chi.URLParam(req, "gateway")) {
		writeError(w, req, apperr.NotFound, nil)
		return
	}
	raw, _ := io.ReadAll(io.LimitReader(req.Body, webhookBodyLimit))
	h.deps.Payments.Webhook(req.Context(), orderRef(raw))
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// orderRef pulls our order id out of a callback body (order_id, or transid for Selcom).
func orderRef(raw []byte) string {
	var body struct {
		OrderID string `json:"order_id"`
		TransID string `json:"transid"`
	}
	_ = json.Unmarshal(raw, &body)
	if body.OrderID != "" {
		return body.OrderID
	}
	return body.TransID
}

func (h paymentHandlers) listHolds(w http.ResponseWriter, req *http.Request) {
	list, err := h.deps.Holds.ListActive(req.Context(), scopeFrom(req, h.deps).Country, adminHoldsLimit)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": list})
}

func (h paymentHandlers) listPayins(w http.ResponseWriter, req *http.Request) {
	limit := queryInt(req, "limit")
	if limit <= 0 || limit > adminPayinsLimit {
		limit = 50
	}
	list, err := h.deps.Payments.List(req.Context(), req.URL.Query().Get("status"), req.URL.Query().Get("driver_id"), adminOpsHandlers(h).currency(scopeFrom(req, h.deps)), req.URL.Query().Get("purpose"), limit)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": list})
}

func (h paymentHandlers) placeHold(w http.ResponseWriter, req *http.Request) {
	var body reasonBody
	if !decode(w, req, &body) {
		return
	}
	hold, err := h.deps.Holds.Place(req.Context(), claimsFrom(req.Context()).Subject, chi.URLParam(req, "id"), body.Reason)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, hold)
}

func (h paymentHandlers) releaseHold(w http.ResponseWriter, req *http.Request) {
	var body reasonBody
	if !decode(w, req, &body) {
		return
	}
	left, err := h.deps.Holds.Release(req.Context(), claimsFrom(req.Context()).Subject, chi.URLParam(req, "id"), body.Reason)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"holds": left})
}
