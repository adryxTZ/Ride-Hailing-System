package httpx

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/kalen-limited/hauler-rider/backend/internal/payments"
)

// mountTripPayment registers what a rider does about a finished ride's fare:
// send another mobile-money prompt (to their number or another), or pay cash.
func mountTripPayment(r chi.Router, d Deps) {
	if d.Payments == nil {
		return
	}
	h := tripPaymentHandlers{deps: d}
	r.Group(func(g chi.Router) {
		g.Use(requireAuth(d), requireApp("rider"))
		g.Post("/trips/{id}/payment/prompt", h.prompt)
		g.Post("/trips/{id}/payment/cash", h.cash)
	})
}

type tripPaymentHandlers struct{ deps Deps }

// promptBody optionally names another number to prompt (default: the account phone).
type promptBody struct {
	Phone string `json:"phone"`
}

// prompt sends a new fare prompt; Idempotency-Key replays return the same prompt.
func (h tripPaymentHandlers) prompt(w http.ResponseWriter, req *http.Request) {
	var body promptBody
	if !decode(w, req, &body) {
		return
	}
	riderID, tripID, key := claimsFrom(req.Context()).Subject, chi.URLParam(req, "id"), req.Header.Get(idempotencyHeader)
	// A retried request answers with the prompt it already sent (the trip is PENDING by then).
	if replay, err := h.deps.Payments.ReplayFare(req.Context(), riderID, tripID, key); err != nil || replay != nil {
		h.respond(w, req, riderID, tripID, err)
		return
	}
	due, err := h.deps.Trips.FareFor(req.Context(), tripID, riderID)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	fare := payments.Fare{TripID: due.TripID, RiderID: due.RiderID, Phone: due.Phone, Amount: due.Amount, Currency: due.Currency}
	_, err = h.deps.Payments.PromptFare(req.Context(), fare, body.Phone, key, false)
	h.respond(w, req, riderID, tripID, err)
}

// respond answers a prompt request with the trip as it now stands.
func (h tripPaymentHandlers) respond(w http.ResponseWriter, req *http.Request, riderID, tripID string, err error) {
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	t, err := h.deps.Trips.Get(req.Context(), tripID, riderID)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusAccepted, t)
}

// cash records that the rider pays the driver in cash instead.
func (h tripPaymentHandlers) cash(w http.ResponseWriter, req *http.Request) {
	t, err := h.deps.Trips.ChooseCash(req.Context(), chi.URLParam(req, "id"), claimsFrom(req.Context()).Subject)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}
