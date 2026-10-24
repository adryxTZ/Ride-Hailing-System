package httpx

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/kalen-limited/hauler-rider/backend/internal/dispatch"
	"github.com/kalen-limited/hauler-rider/backend/internal/identity"
)

// Trip queue routes: the driver's next trip (banner while finishing) and the
// staff dispatch settings (per country, audited). The driver route is mounted
// with the other driver trip routes (mountDriverTrips).

// next returns the driver's queued next trip, or null.
func (h driverTripHandlers) next(w http.ResponseWriter, req *http.Request) {
	n, err := h.deps.Dispatch.NextTrip(req.Context(), claimsFrom(req.Context()).Subject)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"next_trip": n})
}

// mountDispatchSettings registers the staff dispatch settings page routes.
func mountDispatchSettings(r chi.Router, d Deps) {
	if d.Dispatch == nil || d.Dispatch.Settings() == nil {
		return
	}
	h := dispatchSettingsHandlers{settings: d.Dispatch.Settings()}
	r.Group(func(g chi.Router) {
		g.Use(requireAuth(d), requireStaff(), scopeGuard)
		manage := requirePermission(identity.PermDispatchManage)
		g.With(manage).Get("/admin/dispatch/settings", h.list)
		g.With(manage).Patch("/admin/dispatch/settings/{country}", h.set)
	})
}

type dispatchSettingsHandlers struct{ settings *dispatch.Settings }

// list returns every country's settings the staff member's scope covers.
func (h dispatchSettingsHandlers) list(w http.ResponseWriter, req *http.Request) {
	all, err := h.settings.List(req.Context())
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	claims := claimsFrom(req.Context())
	out := make([]dispatch.QueueSettings, 0, len(all))
	for _, q := range all {
		if allowedScope(claims, q.Country, "") {
			out = append(out, q)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
}

// set changes one country's settings; only staff whose scope covers the country.
func (h dispatchSettingsHandlers) set(w http.ResponseWriter, req *http.Request) {
	var body dispatch.QueueSettingsInput
	if !decode(w, req, &body) {
		return
	}
	country := strings.ToUpper(chi.URLParam(req, "country"))
	claims := claimsFrom(req.Context())
	if !allowedScope(claims, country, "") {
		writeError(w, req, ErrScopeForbidden, nil)
		return
	}
	q, err := h.settings.Set(req.Context(), claims.Subject, country, body)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, q)
}
