package httpx

import (
	"context"
	"github.com/kalen-limited/hauler-rider/backend/internal/approvals"
	"github.com/kalen-limited/hauler-rider/backend/internal/approvals/actions"
	"github.com/kalen-limited/hauler-rider/backend/internal/identity"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/scope"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"golang.org/x/sync/errgroup"

	"github.com/kalen-limited/hauler-rider/backend/internal/money"
	"github.com/kalen-limited/hauler-rider/backend/internal/payouts"
	"github.com/kalen-limited/hauler-rider/backend/internal/presence"
	"github.com/kalen-limited/hauler-rider/backend/internal/pricing"
	"github.com/kalen-limited/hauler-rider/backend/internal/trips"
)

// mountAdminOps registers the operations dashboard: overview, live map, trips,
// riders, pricing, finance and staff. Each group checks the staff role it needs.
func mountAdminOps(r chi.Router, d Deps) {
	h := adminOpsHandlers{deps: d}
	r.Group(func(g chi.Router) {
		g.Use(requireAuth(d), requireStaff(), scopeGuard)
		g.Get("/admin/overview", h.overview)
		g.Get("/admin/live", h.live)
		g.Get("/admin/trips", h.trips)
		g.Get("/admin/trips/{id}", h.trip)
		g.Get("/admin/riders", h.riders)
		g.With(requirePermission(identity.PermTripsCancel)).Post("/admin/trips/{id}/cancel", h.cancelTrip)
		g.With(requirePermission(identity.PermRidersBlock)).Post("/admin/riders/{id}/block", h.setUserStatus("DISABLED"))
		g.With(requirePermission(identity.PermRidersBlock)).Post("/admin/riders/{id}/unblock", h.setUserStatus("ACTIVE"))
		g.Get("/admin/pricing", h.pricing)
		g.With(requirePermission(identity.PermPricingEdit)).Put("/admin/pricing/tariffs/{service}/{class}", h.updateTariff)
		g.With(requirePermission(identity.PermPricingEdit)).Delete("/admin/pricing/tariffs/{service}/{class}", h.deleteTariff)
		g.With(requirePermission(identity.PermFeesEdit)).Post("/admin/pricing/policy", h.newPolicy)
		g.With(requirePermission(identity.PermPricingEdit)).Post("/admin/pricing/fixed-routes", h.saveRoute)
		g.With(requirePermission(identity.PermPricingEdit)).Delete("/admin/pricing/fixed-routes/{id}", h.deleteRoute)
		g.With(requirePermission(identity.PermFinanceView)).Get("/admin/finance", h.finance)
		g.With(requirePermission(identity.PermFinanceView)).Get("/admin/finance/journal", h.journal)
		g.With(requirePermission(identity.PermFinanceView)).Post("/admin/finance/verify", h.verifyLedger)
		g.With(requirePermission(identity.PermStaffManage)).Get("/admin/staff", h.staff)
		g.With(requirePermission(identity.PermStaffManage)).Put("/admin/staff", h.setStaff)
	})
}

type adminOpsHandlers struct{ deps Deps }

// Overview is the dashboard's first screen.
type Overview struct {
	Trips          *trips.Stats          `json:"trips"`
	DriversOnline  int                   `json:"drivers_online"`
	DriversOnTrip  int                   `json:"drivers_on_trip"`
	DriverStatuses map[string]int        `json:"driver_statuses"`
	OpenSOS        int                   `json:"open_sos"`
	FraudOpen      int                   `json:"fraud_open"`      // fraud flags waiting for review in the country
	PrivacyOverdue int                   `json:"privacy_overdue"` // privacy requests past their due date
	Finance        *money.FinanceSummary `json:"finance,omitempty"`
	SurgeAreas     int                   `json:"surge_areas"`
}

// midnight is the start of today in the brand's timezone.
func (h adminOpsHandlers) midnight(country string) time.Time {
	tz := h.deps.Brand.Timezone
	if c := h.deps.Brand.CountryByCode(country); c != nil {
		tz = c.Timezone
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}
	n := time.Now().In(loc)
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, loc)
}

// liveLimit bounds the drivers on the live map.
const liveLimit = 2000

func (h adminOpsHandlers) overview(w http.ResponseWriter, req *http.Request) {
	sc := scopeFrom(req, h.deps)
	ctx := req.Context()
	var o Overview
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() (err error) { o.Trips, err = h.deps.Trips.Stats(gctx, h.midnight(sc.Country), sc); return })
	g.Go(func() error {
		if h.deps.Presence == nil {
			return nil
		}
		ds, err := h.deps.Presence.Fresh(gctx, liveLimit)
		for _, d := range h.inScope(gctx, ds, sc) {
			o.DriversOnline++
			if d.State == presence.OnTrip {
				o.DriversOnTrip++
			}
		}
		return err
	})
	g.Go(func() error {
		if h.deps.Fleet == nil {
			return nil
		}
		page, err := h.deps.Fleet.ListDrivers(gctx, "", "", sc, 1, 0)
		if page != nil {
			o.DriverStatuses = page.Counts
		}
		return err
	})
	g.Go(func() (err error) {
		if h.deps.Privacy != nil {
			o.PrivacyOverdue, err = h.deps.Privacy.OverdueCount(gctx, sc.Country)
		}
		return err
	})
	g.Go(func() (err error) {
		if h.deps.Fraud != nil {
			o.FraudOpen, err = h.deps.Fraud.OpenTotal(gctx, sc.Country)
		}
		return err
	})
	g.Go(func() error {
		if h.deps.Safety == nil {
			return nil
		}
		a, err := h.deps.Safety.Alerts(gctx, true, sc)
		o.OpenSOS = len(a)
		return err
	})
	g.Go(func() (err error) {
		if h.deps.Ledger != nil {
			o.Finance, err = h.deps.Ledger.Summary(gctx, h.currency(sc))
		}
		return
	})
	g.Go(func() error {
		if h.deps.Surge != nil {
			areas, err := h.deps.Surge.Areas(gctx)
			o.SurgeAreas = len(areas)
			return err
		}
		return nil
	})
	if err := g.Wait(); err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, o)
}

// LiveDriver is a driver on the live map.
type LiveDriver struct {
	presence.Driver
	Name string `json:"name"`
}

// Live is the live map: drivers, waiting requests, surge areas.
type Live struct {
	Drivers  []LiveDriver           `json:"drivers"`
	Requests []trips.WaitingRequest `json:"requests"`
	Surge    map[string]string      `json:"surge"`
}

func (h adminOpsHandlers) live(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	out := Live{Drivers: []LiveDriver{}, Surge: map[string]string{}}
	if h.deps.Presence != nil {
		ds, err := h.deps.Presence.Fresh(ctx, liveLimit)
		if err != nil {
			writeDomainError(w, req, err)
			return
		}
		ds = h.inScope(ctx, ds, scopeFrom(req, h.deps))
		ids := make([]string, len(ds))
		for i, d := range ds {
			ids[i] = d.ID
		}
		names, _ := h.deps.Identity.Names(ctx, ids)
		for _, d := range ds {
			out.Drivers = append(out.Drivers, LiveDriver{Driver: d, Name: names[d.ID]})
		}
	}
	sc := scopeFrom(req, h.deps)
	reqs, err := h.deps.Trips.Waiting(ctx, sc)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	out.Requests = reqs
	if h.deps.Surge != nil {
		if areas, err := h.deps.Surge.Areas(ctx); err == nil {
			out.Surge = areas
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func parseDate(v string) *time.Time {
	t, err := time.Parse(time.DateOnly, v)
	if err != nil {
		return nil
	}
	return &t
}

func (h adminOpsHandlers) trips(w http.ResponseWriter, req *http.Request) {
	q := req.URL.Query()
	f := trips.AdminFilter{Scope: scopeFrom(req, h.deps), Group: q.Get("status"), Q: q.Get("q"), Service: q.Get("service"),
		RiderID: q.Get("rider_id"), DriverID: q.Get("driver_id"), From: parseDate(q.Get("from")), To: parseDate(q.Get("to")), Limit: queryInt(req, "limit"), Offset: queryInt(req, "offset")}
	if (f.RiderID != "" || f.DriverID != "") && q.Get("country") == "" {
		f.Scope.Country = "" // one person's trips span every country they rode in (limited staff get theirs from scopeGuard)
	}
	page, err := h.deps.Trips.AdminList(req.Context(), f)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (h adminOpsHandlers) trip(w http.ResponseWriter, req *http.Request) {
	t, err := h.deps.Trips.AdminGet(req.Context(), chi.URLParam(req, "id"))
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	area := ""
	if t.Area != nil {
		area = t.Area.ID
	}
	if !allowedScope(claimsFrom(req.Context()), t.Country, area) {
		writeError(w, req, ErrScopeForbidden, nil)
		return
	}
	withDriverLocation(req, h.deps, &t.Trip)
	writeJSON(w, http.StatusOK, t)
}

func (h adminOpsHandlers) cancelTrip(w http.ResponseWriter, req *http.Request) {
	var body reasonBody
	if !decode(w, req, &body) || !tripInScope(w, req, h.deps) {
		return
	}
	t, err := h.deps.Trips.AdminCancel(req.Context(), claimsFrom(req.Context()).Subject, chi.URLParam(req, "id"), body.Reason)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (h adminOpsHandlers) riders(w http.ResponseWriter, req *http.Request) {
	sc := scopeFrom(req, h.deps)
	page, err := h.deps.Identity.ListUsers(req.Context(), "RIDER", req.URL.Query().Get("q"), sc.Country, sc.AreaID, queryInt(req, "limit"), queryInt(req, "offset"))
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (h adminOpsHandlers) setUserStatus(status string) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		var body reasonBody
		if !decode(w, req, &body) {
			return
		}
		if err := h.deps.Identity.SetStatus(req.Context(), claimsFrom(req.Context()).Subject, chi.URLParam(req, "id"), status, body.Reason); err != nil {
			writeDomainError(w, req, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func (h adminOpsHandlers) pricing(w http.ResponseWriter, req *http.Request) {
	sc := scopeFrom(req, h.deps)
	p, err := h.deps.Pricing.AdminPricing(req.Context(), sc.Country, sc.AreaID)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (h adminOpsHandlers) updateTariff(w http.ResponseWriter, req *http.Request) {
	var body pricing.TariffInput
	if !decode(w, req, &body) {
		return
	}
	err := h.deps.Pricing.UpdateTariff(req.Context(), claimsFrom(req.Context()).Subject, scopeFrom(req, h.deps).Country, scopeFrom(req, h.deps).AreaID, chi.URLParam(req, "service"), chi.URLParam(req, "class"), body)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	h.pricing(w, req)
}

func (h adminOpsHandlers) newPolicy(w http.ResponseWriter, req *http.Request) {
	var body struct {
		pricing.PolicyView
		Reason string `json:"reason"`
	}
	if !decode(w, req, &body) {
		return
	}
	if body.Country == "" {
		body.Country = scopeFrom(req, h.deps).Country
	}
	if !allowedScope(claimsFrom(req.Context()), strings.ToUpper(body.Country), body.AreaID) {
		writeError(w, req, ErrScopeForbidden, nil)
		return
	}
	if err := h.deps.Pricing.NewPolicy(req.Context(), claimsFrom(req.Context()).Subject, body.PolicyView, body.Reason); err != nil {
		writeDomainError(w, req, err)
		return
	}
	h.pricing(w, req)
}

func (h adminOpsHandlers) saveRoute(w http.ResponseWriter, req *http.Request) {
	var body struct {
		pricing.FixedRoute
		Reason string `json:"reason"`
	}
	if !decode(w, req, &body) {
		return
	}
	if err := h.deps.Pricing.SaveFixedRoute(req.Context(), claimsFrom(req.Context()).Subject, body.FixedRoute, body.Reason); err != nil {
		writeDomainError(w, req, err)
		return
	}
	h.pricing(w, req)
}

func (h adminOpsHandlers) deleteRoute(w http.ResponseWriter, req *http.Request) {
	if err := h.deps.Pricing.DeleteFixedRoute(req.Context(), claimsFrom(req.Context()).Subject, chi.URLParam(req, "id"), req.URL.Query().Get("reason")); err != nil {
		writeDomainError(w, req, err)
		return
	}
	h.pricing(w, req)
}

// financeWalletsLimit bounds the driver balance list.
const financeWalletsLimit = 200

// FinanceView is the finance page: totals and driver balances.
type FinanceView struct {
	Summary *money.FinanceSummary `json:"summary"`
	Drivers []money.DriverBalance `json:"drivers"`
	Payouts *payouts.Totals       `json:"payouts,omitempty"`
}

func (h adminOpsHandlers) finance(w http.ResponseWriter, req *http.Request) {
	cur := h.currency(scopeFrom(req, h.deps))
	sum, err := h.deps.Ledger.Summary(req.Context(), cur)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	drivers, err := h.deps.Ledger.DriverBalances(req.Context(), cur, financeWalletsLimit)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	view := FinanceView{Summary: sum, Drivers: drivers}
	if h.deps.Payouts != nil {
		sc := scopeFrom(req, h.deps)
		t, err := h.deps.Payouts.Totals(req.Context(), cur, h.midnight(sc.Country))
		if err != nil {
			writeDomainError(w, req, err)
			return
		}
		view.Payouts = &t
	}
	writeJSON(w, http.StatusOK, view)
}

func (h adminOpsHandlers) journal(w http.ResponseWriter, req *http.Request) {
	limit := queryInt(req, "limit")
	if limit <= 0 || limit > financeWalletsLimit {
		limit = 100
	}
	list, err := h.deps.Ledger.Journal(req.Context(), h.currency(scopeFrom(req, h.deps)), limit)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": list})
}

func (h adminOpsHandlers) verifyLedger(w http.ResponseWriter, req *http.Request) {
	broken, checked, err := h.deps.Ledger.Verify(req.Context())
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": broken == 0, "checked": checked, "broken_at": strconv.FormatInt(broken, 10)})
}

func (h adminOpsHandlers) staff(w http.ResponseWriter, req *http.Request) {
	list, err := h.deps.Identity.ListStaff(req.Context())
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	roles, err := h.deps.Identity.ListRoles(req.Context())
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	names := make([]string, len(roles))
	for i, r := range roles {
		names[i] = r.Name
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": list, "roles": names})
}

func (h adminOpsHandlers) setStaff(w http.ResponseWriter, req *http.Request) {
	var body struct {
		Phone    string   `json:"phone"`
		Name     string   `json:"full_name"`
		Roles    []string `json:"roles"`
		Email    string   `json:"email"`
		Password string   `json:"password"`
		Reason   string   `json:"reason"`
	}
	if !decode(w, req, &body) {
		return
	}
	if h.staffRolesGated(w, req, body.Phone, body.Name, body.Roles, body.Reason) {
		return // a held role change stores no email or password: set them after approval
	}
	u, err := h.deps.Identity.SetStaffRoles(req.Context(), claimsFrom(req.Context()).Subject, body.Phone, body.Name, body.Roles)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	if body.Email != "" || body.Password != "" {
		if err := h.deps.Identity.SetStaffCredentials(req.Context(), u.ID, body.Email, body.Password); err != nil {
			writeDomainError(w, req, err)
			return
		}
	}
	h.staff(w, req)
}

// staffRolesGated offers a role change to its workflow; an edit that keeps the
// person's roles (e.g. only a new password) is not a role change and applies at once.
func (h adminOpsHandlers) staffRolesGated(w http.ResponseWriter, req *http.Request, phone, name string, roles []string, reason string) bool {
	if h.deps.Approvals == nil || !h.deps.Approvals.Enabled(req.Context(), actions.StaffRoles) {
		return false
	}
	cur, err := h.deps.Identity.StaffRolesByPhone(req.Context(), phone)
	if err != nil {
		writeDomainError(w, req, err)
		return true
	}
	if actions.SameRoles(cur.Roles, roles) {
		return false
	}
	return sentForApproval(w, req, h.deps, approvals.Submission{Action: actions.StaffRoles, Reason: reason, Target: "user:" + cur.UserID,
		Payload: actions.StaffRolesPayload{Phone: phone, FullName: name, Roles: roles}})
}

// deleteTariff removes an area's own prices so it inherits the country's again.
func (h adminOpsHandlers) deleteTariff(w http.ResponseWriter, req *http.Request) {
	sc := scopeFrom(req, h.deps)
	if err := h.deps.Pricing.DeleteTariffOverride(req.Context(), claimsFrom(req.Context()).Subject, sc.Country, sc.AreaID, chi.URLParam(req, "service"), chi.URLParam(req, "class"), req.URL.Query().Get("reason")); err != nil {
		writeDomainError(w, req, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// scopeFrom reads the staff scope (?country=&area_id=); the brand's home country by default.
// currency is the money of the scoped country (each country is reported in its own currency).
func (h adminOpsHandlers) currency(sc scope.Scope) string {
	if c := h.deps.Brand.CountryByCode(sc.Country); c != nil {
		return c.Currency
	}
	return h.deps.Brand.Currency
}

// inScope keeps the online drivers registered in the scoped area, or in any
// area of the scoped country (area membership is how presence knows a country).
func (h adminOpsHandlers) inScope(ctx context.Context, ds []presence.Driver, sc scope.Scope) []presence.Driver {
	if h.deps.Areas == nil {
		return ds
	}
	want := map[string]bool{}
	if sc.AreaID != "" {
		want[sc.AreaID] = true
	} else if list, err := h.deps.Areas.List(ctx, sc.Country); err == nil {
		for _, a := range list {
			want[a.ID] = true
		}
	}
	out := ds[:0:0]
	for _, d := range ds {
		if slices.ContainsFunc(d.Areas, func(id string) bool { return want[id] }) {
			out = append(out, d)
		}
	}
	return out
}

func scopeFrom(req *http.Request, d Deps) scope.Scope {
	q := req.URL.Query()
	sc := scope.Scope{Country: strings.ToUpper(q.Get("country")), AreaID: q.Get("area_id")}
	if sc.Country == "" {
		sc.Country = d.Brand.Country
	}
	return sc
}
