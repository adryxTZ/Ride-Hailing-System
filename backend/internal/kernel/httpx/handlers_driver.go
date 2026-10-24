package httpx

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/kalen-limited/hauler-rider/backend/internal/areas"
	"github.com/kalen-limited/hauler-rider/backend/internal/holds"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
	"github.com/kalen-limited/hauler-rider/backend/internal/presence"
	"github.com/kalen-limited/hauler-rider/backend/internal/providers/routing"
	"github.com/kalen-limited/hauler-rider/backend/internal/realtime"
)

// mountDriver registers the driver-app supply endpoints: go online/offline and
// stream position. Both need a token issued to the driver app.
func mountDriver(r chi.Router, d Deps) {
	h := driverHandlers{deps: d}
	r.Group(func(g chi.Router) {
		g.Use(requireAuth(d), requireApp("driver"), requirePresence(d))
		g.Get("/driver/status", h.status)
		g.Put("/driver/status", h.setStatus)
		g.Post("/driver/location", h.location)
	})
}

type driverHandlers struct{ deps Deps }

// driverStatusRequest is the body of PUT /driver/status.
type driverStatusRequest struct {
	Online         bool          `json:"online"`
	Location       routing.Point `json:"location"`
	Heading        float64       `json:"heading"`
	Services       []string      `json:"services"`        // service keys the driver offers now
	VehicleClasses []string      `json:"vehicle_classes"` // class keys of the vehicle in use
	VehicleID      string        `json:"vehicle_id"`
	Mocked         bool          `json:"mocked"` // Android reports a mocked location (fraud check)
	Areas          []string      `json:"-"`      // from the driver's registration, never from the app
}

type driverStatusResponse struct {
	Online           bool             `json:"online"`
	Presence         *presence.Driver `json:"presence,omitempty"`
	Holds            []holds.Hold     `json:"holds"` // why the driver cannot go online (empty = free)
	Areas            []areas.AreaRef  `json:"areas"` // where the driver is registered to work
	CurrentArea      *areas.AreaRef   `json:"current_area"`
	InRegisteredArea bool             `json:"in_registered_area"`
}

func (h driverHandlers) status(w http.ResponseWriter, req *http.Request) {
	driverID := claimsFrom(req.Context()).Subject
	p, err := h.deps.Presence.Get(req.Context(), driverID)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	hs, err := h.activeHolds(req.Context(), driverID)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	resp := driverStatusResponse{Online: p != nil, Presence: p, Holds: hs, Areas: []areas.AreaRef{}, InRegisteredArea: true}
	if h.deps.Areas != nil {
		var at *routing.Point
		if p != nil {
			at = &p.Location
		}
		if err := h.fillAreas(req.Context(), driverID, at, &resp); err != nil {
			writeDomainError(w, req, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

// activeHolds lists the driver's holds, or none when the holds module is off.
func (h driverHandlers) activeHolds(ctx context.Context, driverID string) ([]holds.Hold, error) {
	if h.deps.Holds == nil {
		return []holds.Hold{}, nil
	}
	return h.deps.Holds.Active(ctx, driverID)
}

func (h driverHandlers) setStatus(w http.ResponseWriter, req *http.Request) {
	var body driverStatusRequest
	if !decode(w, req, &body) {
		return
	}
	driverID := claimsFrom(req.Context()).Subject
	if !body.Online {
		if err := h.deps.Presence.Offline(req.Context(), driverID); err != nil {
			writeDomainError(w, req, err)
			return
		}
		writeJSON(w, http.StatusOK, driverStatusResponse{Online: false})
		return
	}
	if !validPoint(body.Location) {
		writeError(w, req, apperr.Validation, map[string]any{"fields": []string{"location"}})
		return
	}
	if !consentGate(w, req, h.deps) {
		return
	}
	// With the fleet module the approved vehicle decides what the driver serves;
	// the app's own claim is only used where fleet is not wired (tests).
	if h.deps.Fleet != nil {
		el, err := h.deps.Fleet.Eligibility(req.Context(), driverID)
		if err != nil {
			writeDomainError(w, req, err)
			return
		}
		body.Services, body.VehicleClasses, body.VehicleID = el.Services, el.Classes, el.VehicleID
	}
	if len(body.Services) == 0 || len(body.VehicleClasses) == 0 {
		writeError(w, req, apperr.Validation, map[string]any{"fields": []string{"services", "vehicle_classes"}})
		return
	}
	// A driver registered in areas may only go online inside one of them.
	if h.deps.Areas != nil && h.deps.Fleet != nil {
		var resp driverStatusResponse
		if err := h.fillAreas(req.Context(), driverID, &body.Location, &resp); err != nil {
			writeDomainError(w, req, err)
			return
		}
		if len(resp.Areas) > 0 && !resp.InRegisteredArea {
			writeError(w, req, areas.ErrOutsideDriverAreas, map[string]any{"areas": resp.Areas})
			return
		}
		for _, a := range resp.Areas {
			body.Areas = append(body.Areas, a.ID)
		}
	}
	// A held driver (debt or staff) stays offline; the response says what lifts it.
	if h.deps.Holds != nil {
		st, err := h.deps.Holds.Status(req.Context(), driverID)
		if err != nil {
			writeDomainError(w, req, err)
			return
		}
		if len(st.Holds) > 0 {
			writeError(w, req, holds.ErrOnHold, map[string]any{"holds": st.Holds, "pay_to_unhold": st.PayToUnhold})
			return
		}
	}
	p, err := h.deps.Presence.Online(req.Context(), driverID,
		presence.Ping{Location: body.Location, Heading: body.Heading, Mocked: body.Mocked},
		presence.Capabilities{Services: body.Services, Classes: body.VehicleClasses, VehicleID: body.VehicleID, Areas: body.Areas})
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, driverStatusResponse{Online: true, Presence: p, Holds: []holds.Hold{}})
}

// locationRequest is one GPS sample; the HTTP path is the fallback for the websocket stream.
type locationRequest struct {
	Location routing.Point `json:"location"`
	Heading  float64       `json:"heading"`
	SpeedMps float64       `json:"speed_mps"`
	Mocked   bool          `json:"mocked"`    // Android reports a mocked location (fraud check)
	ClientAt *time.Time    `json:"client_at"` // kept for diagnostics only; freshness uses server time
}

func (h driverHandlers) location(w http.ResponseWriter, req *http.Request) {
	var body locationRequest
	if !decode(w, req, &body) {
		return
	}
	if !validPoint(body.Location) {
		writeError(w, req, apperr.Validation, map[string]any{"fields": []string{"location"}})
		return
	}
	p, err := h.deps.Presence.Update(req.Context(), claimsFrom(req.Context()).Subject,
		presence.Ping{Location: body.Location, Heading: body.Heading, SpeedMps: body.SpeedMps, Mocked: body.Mocked})
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	if p != nil && p.TripID != "" && p.RiderID != "" && p.State == presence.OnTrip {
		h.deps.Hub.Publish(req.Context(), p.RiderID, realtime.NewFrame(realtime.TypeDriverLocation, map[string]any{
			"trip_id": p.TripID, "location": p.Location, "heading": p.Heading, "at": p.At,
		}))
	}
	writeJSON(w, http.StatusOK, driverStatusResponse{Online: true, Presence: p, Holds: []holds.Hold{}})
}

func validPoint(p routing.Point) bool {
	return p.Lat >= -90 && p.Lat <= 90 && p.Lng >= -180 && p.Lng <= 180 && !(p.Lat == 0 && p.Lng == 0)
}

// requireApp rejects tokens issued to another app (a rider token cannot go online as a driver).
func requireApp(app string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if c := claimsFrom(req.Context()); c == nil || c.App != app {
				writeError(w, req, apperr.Forbidden, map[string]any{"required_app": app})
				return
			}
			next.ServeHTTP(w, req)
		})
	}
}

// requirePresence answers 503 when the process runs without Redis.
func requirePresence(d Deps) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if d.Presence == nil {
				writeError(w, req, apperr.New("PRESENCE_UNAVAILABLE", http.StatusServiceUnavailable, "Driver presence is not available."), nil)
				return
			}
			next.ServeHTTP(w, req)
		})
	}
}

// fillAreas adds the driver's registered areas and whether `at` lies in one of them.
func (h driverHandlers) fillAreas(ctx context.Context, driverID string, at *routing.Point, resp *driverStatusResponse) error {
	refs, _, err := h.deps.Areas.DriverAreas(ctx, driverID)
	if err != nil {
		return err
	}
	resp.Areas, resp.InRegisteredArea = refs, true
	if at == nil || len(refs) == 0 {
		return nil
	}
	ix, err := h.deps.Areas.Index(ctx)
	if err != nil {
		return err
	}
	ref, ok := ix.AreaOf(*at)
	if ok {
		resp.CurrentArea = &ref
	}
	resp.InRegisteredArea = false
	for _, r := range refs {
		if ok && r.ID == ref.ID {
			resp.InRegisteredArea = true
		}
	}
	return nil
}
