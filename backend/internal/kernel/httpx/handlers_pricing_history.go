package httpx

import (
	"context"
	"encoding/csv"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/kalen-limited/hauler-rider/backend/internal/identity"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
	"github.com/kalen-limited/hauler-rider/backend/internal/money"
	"github.com/kalen-limited/hauler-rider/backend/internal/pricing"
	"github.com/kalen-limited/hauler-rider/backend/internal/profiles"
	"github.com/kalen-limited/hauler-rider/backend/internal/providers/routing"
)

// PricingSimulator prices a trip without saving it and splits each class's
// fare (wired in cmd, where pricing and money meet).
type PricingSimulator interface {
	Simulate(ctx context.Context, req pricing.Request, opts pricing.PreviewOptions) (*Simulation, error)
}

// Simulation is the admin simulator's answer: the options a rider would see,
// the profile resolution behind them and the split of each class's fare.
type Simulation struct {
	Country    string                     `json:"country"`
	Area       pricing.AreaRef            `json:"area"`
	DistanceM  int                        `json:"distance_m"`
	DurationS  int                        `json:"duration_s"`
	Polyline   string                     `json:"polyline"`
	Options    []pricing.Option           `json:"options"`
	Resolution profiles.QuotePricing      `json:"resolution"`
	Splits     map[string]money.Breakdown `json:"splits"`
}

// mountPricingHistory registers "as of", the assignment timeline, the
// simulator and the pricing-records CSV (S3). Staff only, inside their scope.
func mountPricingHistory(r chi.Router, d Deps) {
	if d.Profiles == nil || d.Pricing == nil {
		return
	}
	h := pricingHistoryHandlers{deps: d}
	r.Group(func(g chi.Router) {
		g.Use(requireAuth(d), requireStaff())
		g.With(scopeGuard).Get("/admin/pricing/as-of", h.asOf)
		g.With(scopeGuard).Get("/admin/pricing/history", h.history)
		g.With(scopeGuard, requirePermission(identity.PermFinanceView)).Get("/admin/pricing/records.csv", h.recordsCSV)
		if d.Simulator != nil {
			g.Post("/admin/pricing/simulate", h.simulate)
		}
	})
}

type pricingHistoryHandlers struct{ deps Deps }

// parseTime accepts an RFC 3339 time or a date (midnight UTC); nil when empty or invalid.
func parseTime(v string) *time.Time {
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return &t
	}
	return parseDate(v)
}

// queryPoint reads "<prefix>lat" and "<prefix>lng"; ok is false when either is missing or invalid.
func queryPoint(req *http.Request, prefix string) (routing.Point, bool) {
	q := req.URL.Query()
	lat, err1 := strconv.ParseFloat(q.Get(prefix+"lat"), 64)
	lng, err2 := strconv.ParseFloat(q.Get(prefix+"lng"), 64)
	return routing.Point{Lat: lat, Lng: lng}, err1 == nil && err2 == nil
}

func (h pricingHistoryHandlers) asOf(w http.ResponseWriter, req *http.Request) {
	pickup, ok := queryPoint(req, "")
	at := parseTime(req.URL.Query().Get("at"))
	if !ok || at == nil {
		writeError(w, req, apperr.Validation, map[string]any{"fields": []string{"lat", "lng", "at"}})
		return
	}
	var dropoff *routing.Point
	if p, ok := queryPoint(req, "dropoff_"); ok {
		dropoff = &p
	}
	res, err := h.deps.Pricing.AsOf(req.Context(), scopeFrom(req, h.deps).Country, pickup, dropoff, *at)
	respond(w, req, http.StatusOK, res, err)
}

func (h pricingHistoryHandlers) history(w http.ResponseWriter, req *http.Request) {
	q := req.URL.Query()
	from, to := parseTime(q.Get("from")), parseTime(q.Get("to"))
	if from == nil || to == nil {
		writeError(w, req, apperr.Validation, map[string]any{"fields": []string{"from", "to"}})
		return
	}
	if !allowedScope(claimsFrom(req.Context()), scopeFrom(req, h.deps).Country, "") { // the timeline spans the whole country
		writeError(w, req, ErrScopeForbidden, nil)
		return
	}
	hq := profiles.HistoryQuery{Country: scopeFrom(req, h.deps).Country, ScopeKind: strings.ToUpper(q.Get("scope_kind")), ScopeID: q.Get("scope_id"), From: *from, To: *to}
	res, err := h.deps.Profiles.History(req.Context(), hq)
	respond(w, req, http.StatusOK, res, err)
}

// recordsHeader is the CSV's first row.
var recordsHeader = []string{"trip_id", "finalized_at", "country", "scope_kind", "scope_name", "profile_id", "profile_name", "profile_version",
	"default_version_id", "source", "charges", "breakdown", "record_hash"}

// recordsCSV streams finalised pricing records; it covers a whole country, so
// it needs country-level access.
func (h pricingHistoryHandlers) recordsCSV(w http.ResponseWriter, req *http.Request) {
	q := req.URL.Query()
	country := scopeFrom(req, h.deps).Country
	if !allowedScope(claimsFrom(req.Context()), country, "") {
		writeError(w, req, ErrScopeForbidden, nil)
		return
	}
	from, to := parseTime(q.Get("from")), parseTime(q.Get("to"))
	if from == nil || to == nil {
		writeError(w, req, apperr.Validation, map[string]any{"fields": []string{"from", "to"}})
		return
	}
	cw := csv.NewWriter(w)
	started := false
	err := h.deps.Profiles.ExportRecords(req.Context(), country, *from, *to, func(r profiles.RecordRow) error {
		if !started {
			startCSV(w, "pricing-records-"+country+".csv")
			started = true
			if err := cw.Write(recordsHeader); err != nil {
				return err
			}
		}
		return cw.Write([]string{r.TripID, r.FinalizedAt.UTC().Format(time.RFC3339), r.Country, r.ScopeKind, csvSafe(r.ScopeName), r.ProfileID, csvSafe(r.ProfileName),
			strconv.Itoa(r.ProfileVersion), r.DefaultVersion, r.Source, strconv.Itoa(r.Charges), string(r.Final), r.RecordHash})
	})
	if err != nil && !started {
		writeDomainError(w, req, err)
		return
	}
	if err != nil { // mid-stream: drop the connection so the file is never taken as complete
		slog.Error("records export failed", "err", err)
		panic(http.ErrAbortHandler)
	}
	if !started { // no rows: still a valid file with its header
		startCSV(w, "pricing-records-"+country+".csv")
		_ = cw.Write(recordsHeader)
	}
	cw.Flush()
}

// csvSafe stops a spreadsheet from running staff-typed text as a formula.
func csvSafe(v string) string {
	if v != "" && strings.ContainsRune("=+-@\t\r", rune(v[0])) {
		return "'" + v
	}
	return v
}

func startCSV(w http.ResponseWriter, name string) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
}

type simulateBody struct {
	Service          string          `json:"service"`
	VehicleClass     string          `json:"vehicle_class"`
	Pickup           routing.Point   `json:"pickup"`
	Dropoff          *routing.Point  `json:"dropoff"`
	Waypoints        []routing.Point `json:"waypoints"`
	At               *time.Time      `json:"at"`
	PreviewProfileID string          `json:"preview_profile_id"`
}

func (h pricingHistoryHandlers) simulate(w http.ResponseWriter, req *http.Request) {
	var body simulateBody
	if !decode(w, req, &body) {
		return
	}
	opts := pricing.PreviewOptions{PreviewProfileID: body.PreviewProfileID}
	if body.At != nil {
		opts.At = *body.At
	}
	sim, err := h.deps.Simulator.Simulate(req.Context(), pricing.Request{ServiceKey: body.Service, Pickup: body.Pickup, Dropoff: body.Dropoff, Waypoints: body.Waypoints}, opts)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	if !allowedScope(claimsFrom(req.Context()), sim.Country, sim.Area.ID) {
		writeError(w, req, ErrScopeForbidden, nil)
		return
	}
	if body.VehicleClass != "" {
		sim.Options = onlyClass(sim.Options, body.VehicleClass)
	}
	writeJSON(w, http.StatusOK, sim)
}

func onlyClass(opts []pricing.Option, class string) []pricing.Option {
	out := []pricing.Option{}
	for _, o := range opts {
		if o.VehicleClass == class {
			out = append(out, o)
		}
	}
	return out
}
