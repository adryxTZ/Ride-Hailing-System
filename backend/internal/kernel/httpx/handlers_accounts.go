package httpx

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/accounts"
	"github.com/kalen-limited/hauler-rider/backend/internal/identity"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
)

// mountAccounts registers Finance › Accounts: every party paid from fares with
// its balance and entries (finance.view), and recording payments against what
// the platform owes (finance.settle). Amounts are in the selected country's currency.
func mountAccounts(r chi.Router, d Deps) {
	if d.Accounts == nil {
		return
	}
	h := accountHandlers{deps: d, ops: adminOpsHandlers{deps: d}}
	r.Group(func(g chi.Router) {
		g.Use(requireAuth(d), requireStaff(), scopeGuard)
		view := g.With(requirePermission(identity.PermFinanceView))
		view.Get("/admin/finance/accounts", h.list)
		view.Get("/admin/finance/accounts/{key}/entries", h.entries)
		view.Get("/admin/finance/accounts/{key}/export", h.export)
		view.Get("/admin/finance/rider-refunds/owed", h.owed)
		g.With(requirePermission(identity.PermFinanceSettle)).Post("/admin/finance/accounts/{key}/payments", h.pay)
	})
}

type accountHandlers struct {
	deps Deps
	ops  adminOpsHandlers // currency and timezone of the scoped country
}

// periodDays is the date format of the from/to query parameters.
const periodDays = "2006-01-02"

// errBadPeriod refuses a from/to that is not a date range.
var errBadPeriod = apperr.Validation.WithDetails(map[string]any{"field": "from"})

// period reads from/to (dates in the country's timezone; to is inclusive).
// The default is this month up to today.
func (h accountHandlers) period(req *http.Request) (accounts.Period, bool) {
	today := h.ops.midnight(scopeFrom(req, h.deps).Country)
	loc := today.Location()
	from := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, loc)
	to := today
	q := req.URL.Query()
	var err error
	if v := q.Get("from"); v != "" {
		if from, err = time.ParseInLocation(periodDays, v, loc); err != nil {
			return accounts.Period{}, false
		}
	}
	if v := q.Get("to"); v != "" {
		if to, err = time.ParseInLocation(periodDays, v, loc); err != nil {
			return accounts.Period{}, false
		}
	}
	if to.Before(from) {
		return accounts.Period{}, false
	}
	return accounts.Period{From: from, To: to.AddDate(0, 0, 1)}, true
}

// currency is the scoped country's currency.
func (h accountHandlers) currency(req *http.Request) string {
	return h.ops.currency(scopeFrom(req, h.deps))
}

func (h accountHandlers) list(w http.ResponseWriter, req *http.Request) {
	p, ok := h.period(req)
	if !ok {
		writeDomainError(w, req, errBadPeriod)
		return
	}
	list, err := h.deps.Accounts.List(req.Context(), h.currency(req), p)
	respond(w, req, http.StatusOK, list, err)
}

func (h accountHandlers) entries(w http.ResponseWriter, req *http.Request) {
	p, ok := h.period(req)
	if !ok {
		writeDomainError(w, req, errBadPeriod)
		return
	}
	page, err := h.deps.Accounts.Entries(req.Context(), chi.URLParam(req, "key"), h.currency(req), p,
		queryInt(req, "limit"), queryInt(req, "offset"))
	respond(w, req, http.StatusOK, page, err)
}

// export streams the period's entries as CSV (formula cells escaped).
func (h accountHandlers) export(w http.ResponseWriter, req *http.Request) {
	p, ok := h.period(req)
	if !ok {
		writeDomainError(w, req, errBadPeriod)
		return
	}
	key := chi.URLParam(req, "key")
	list, err := h.deps.Accounts.Export(req.Context(), key, h.currency(req), p)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	name := strings.NewReplacer(":", "-").Replace(key) + "_" + p.From.Format(periodDays) + "_" + p.To.AddDate(0, 0, -1).Format(periodDays) + ".csv"
	startCSV(w, strings.ToLower(name))
	if err := accounts.WriteCSV(w, h.currency(req), list); err != nil {
		logUnexpected(req, err) // the client went away mid-file
	}
}

func (h accountHandlers) owed(w http.ResponseWriter, req *http.Request) {
	owed, err := h.deps.Accounts.Owed(req.Context(), h.currency(req))
	respond(w, req, http.StatusOK, owed, err)
}

type paymentBody struct {
	Amount    decimal.Decimal `json:"amount"`
	PaidOn    string          `json:"paid_on"`
	Method    string          `json:"method"`
	Reference string          `json:"reference"`
	Reason    string          `json:"reason"`
	RefundIDs []string        `json:"refund_ids"`
	RiderID   string          `json:"rider_id"`
}

// pay records a payment; finance is per country, so only staff who may work in
// the whole country record one. A replayed Idempotency-Key answers 200.
func (h accountHandlers) pay(w http.ResponseWriter, req *http.Request) {
	var body paymentBody
	if !decode(w, req, &body) {
		return
	}
	sc := scopeFrom(req, h.deps)
	if !allowedScope(claimsFrom(req.Context()), sc.Country, "") {
		writeError(w, req, ErrScopeForbidden, nil)
		return
	}
	p, err := h.deps.Accounts.RecordPayment(req.Context(), accounts.PaymentInput{
		Key: chi.URLParam(req, "key"), Currency: h.ops.currency(sc), Country: sc.Country, Amount: body.Amount, PaidOn: body.PaidOn,
		Method: body.Method, Reference: body.Reference, Reason: body.Reason, RefundIDs: body.RefundIDs,
		RiderID: body.RiderID, IdemKey: req.Header.Get(idempotencyHeader), StaffID: claimsFrom(req.Context()).Subject,
	})
	status := http.StatusCreated
	if err == nil && p.Replayed {
		status = http.StatusOK
	}
	respond(w, req, status, p, err)
}
