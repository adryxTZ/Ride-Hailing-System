package httpx

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/holds"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
	"github.com/kalen-limited/hauler-rider/backend/internal/money"
	"github.com/kalen-limited/hauler-rider/backend/internal/payments"
	"github.com/kalen-limited/hauler-rider/backend/internal/payouts"
	"github.com/kalen-limited/hauler-rider/backend/internal/presence"
	"github.com/kalen-limited/hauler-rider/backend/internal/trips"
)

// mountDriverTrips registers what a driver does with offers and assigned trips.
func mountDriverTrips(r chi.Router, d Deps) {
	h := driverTripHandlers{deps: d}
	r.Group(func(g chi.Router) {
		g.Use(requireAuth(d), requireApp("driver"), requireDispatch(d))
		g.Get("/driver/offer", h.currentOffer)
		g.Post("/driver/offers/{tripId}/accept", h.accept)
		g.Post("/driver/offers/{tripId}/decline", h.decline)
		g.Get("/driver/trips/active", h.active)
		g.Get("/driver/trips", h.list)
		g.Get("/driver/wallet", h.wallet)
		g.Post("/driver/trips/{id}/transitions", h.transition)
	})
}

type driverTripHandlers struct{ deps Deps }

func (h driverTripHandlers) currentOffer(w http.ResponseWriter, req *http.Request) {
	o, err := h.deps.Dispatch.Current(req.Context(), claimsFrom(req.Context()).Subject)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"offer": o})
}

func (h driverTripHandlers) accept(w http.ResponseWriter, req *http.Request) {
	t, err := h.deps.Dispatch.Accept(req.Context(), chi.URLParam(req, "tripId"), claimsFrom(req.Context()).Subject)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (h driverTripHandlers) decline(w http.ResponseWriter, req *http.Request) {
	if err := h.deps.Dispatch.Decline(req.Context(), chi.URLParam(req, "tripId"), claimsFrom(req.Context()).Subject); err != nil {
		writeDomainError(w, req, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h driverTripHandlers) active(w http.ResponseWriter, req *http.Request) {
	t, err := h.deps.Trips.ActiveForDriver(req.Context(), claimsFrom(req.Context()).Subject)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"trip": t})
}

// list returns the driver's recent trips, newest first (history and earnings).
func (h driverTripHandlers) list(w http.ResponseWriter, req *http.Request) {
	list, err := h.deps.Trips.ListForDriver(req.Context(), claimsFrom(req.Context()).Subject)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, listResponse[trips.Trip]{Data: list})
}

const walletEntries = 50

// wallet returns the driver's balance (negative = owes the platform) and history.
// walletView is the driver's wallet with their debt position and open pay-in.
type walletView struct {
	money.Wallet
	Debt         decimal.Decimal  `json:"debt"`
	Limits       holds.Limits     `json:"limits"`
	Holds        []holds.Hold     `json:"holds"`
	SoftHoldAt   *time.Time       `json:"soft_hold_at"`
	PayToUnhold  *decimal.Decimal `json:"pay_to_unhold"`
	PendingPayin *payments.Payin  `json:"pending_payin"`
	// Cash-out: what the driver may cash out, the minimum, any open payout.
	Payouts *payouts.Wallet `json:"payouts,omitempty"`
}

func (h driverTripHandlers) wallet(w http.ResponseWriter, req *http.Request) {
	if h.deps.Ledger == nil {
		writeJSON(w, http.StatusOK, map[string]any{"currency": "", "balance": "0", "entries": []any{}})
		return
	}
	driverID := claimsFrom(req.Context()).Subject
	wal, err := h.deps.Ledger.DriverWallet(req.Context(), driverID, walletEntries)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	view := walletView{Wallet: wal, Debt: decimal.Zero, Holds: []holds.Hold{}}
	if wal.Balance.IsNegative() {
		view.Debt = wal.Balance.Neg()
	}
	if h.deps.Holds != nil {
		st, err := h.deps.Holds.Status(req.Context(), driverID)
		if err != nil {
			writeDomainError(w, req, err)
			return
		}
		view.Limits, view.Holds, view.SoftHoldAt, view.PayToUnhold = st.Limits, st.Holds, st.SoftHoldAt, st.PayToUnhold
	}
	if h.deps.Payments != nil {
		if view.PendingPayin, err = h.deps.Payments.Pending(req.Context(), driverID); err != nil {
			writeDomainError(w, req, err)
			return
		}
	}
	if h.deps.Payouts != nil {
		pw, err := h.deps.Payouts.WalletOf(req.Context(), driverID)
		if err != nil {
			writeDomainError(w, req, err)
			return
		}
		view.Payouts = &pw
	}
	writeJSON(w, http.StatusOK, view)
}

type transitionBody struct {
	To trips.Status `json:"to"`
	// Reason with CANCELLED_BY_DRIVER: "rider_no_show" charges the no-show fee instead of finding a new driver.
	Reason string `json:"reason"`
}

// transition moves the driver's trip one step (en route, arrived, start, complete, cancel).
func (h driverTripHandlers) transition(w http.ResponseWriter, req *http.Request) {
	var body transitionBody
	if !decode(w, req, &body) {
		return
	}
	if body.To == "" {
		writeError(w, req, apperr.Validation, map[string]any{"fields": []map[string]string{{"path": "to", "code": "REQUIRED"}}})
		return
	}
	driverID := claimsFrom(req.Context()).Subject
	t, err := h.deps.Trips.TransitionByDriver(req.Context(), chi.URLParam(req, "id"), driverID, body.To, body.Reason)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	if body.To == trips.CancelledByDriver || trips.IsTerminal(t.Status) {
		_ = h.deps.Presence.SetState(req.Context(), driverID, presence.Idle, "", "")
	}
	writeJSON(w, http.StatusOK, t)
}

// requireDispatch answers 503 when the process runs without Redis (no dispatch).
func requireDispatch(d Deps) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if d.Dispatch == nil || d.Presence == nil {
				writeError(w, req, apperr.New("PRESENCE_UNAVAILABLE", http.StatusServiceUnavailable, "Driver presence is not available."), nil)
				return
			}
			next.ServeHTTP(w, req)
		})
	}
}
