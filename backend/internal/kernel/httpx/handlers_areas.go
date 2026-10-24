package httpx

import (
	"github.com/kalen-limited/hauler-rider/backend/internal/identity"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/kalen-limited/hauler-rider/backend/internal/areas"
)

// mountAreas registers the operating-area endpoints: countries and areas for
// staff (read: any staff; write: Operations), and a driver's areas (Operations
// and Driver Review).
func mountAreas(r chi.Router, d Deps) {
	if d.Areas == nil {
		return
	}
	h := areaHandlers{deps: d}
	r.Group(func(g chi.Router) {
		g.Use(requireAuth(d), requireStaff(), scopeGuard)
		g.Get("/admin/countries", h.countries)
		g.Get("/admin/areas", h.list)
		g.Get("/admin/areas/{id}", h.get)
		g.With(requirePermission(identity.PermAreasEdit)).Post("/admin/areas", h.create)
		g.With(requirePermission(identity.PermAreasEdit)).Patch("/admin/areas/{id}", h.update)
		g.With(requirePermission(identity.PermAreasEdit)).Put("/admin/areas/{id}/cells", h.saveCells)
		g.With(requirePermission(identity.PermDriversReview)).Put("/admin/drivers/{id}/areas", h.setDriverAreas)
	})
	r.With(requireAuth(d), requireApp("driver")).Get("/driver/areas", h.driverAreas)
}

// driverAreaChoices is what the driver app's region picker shows.
type driverAreaChoices struct {
	Country  string          `json:"country"`
	Areas    []areas.AreaRef `json:"areas"`    // ACTIVE areas of the country, by name
	Selected []areas.AreaRef `json:"selected"` // where the driver is registered now
	Primary  string          `json:"primary"`
}

// driverAreas lists the regions a driver can work in (their country's ACTIVE
// areas) and the ones they are registered in. Changing them after approval is
// a staff action; at sign-up the profile form sends area_id.
func (h areaHandlers) driverAreas(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	country := strings.ToUpper(req.URL.Query().Get("country"))
	selected, primary, err := h.deps.Areas.DriverAreas(ctx, claimsFrom(ctx).Subject)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	if len(selected) > 0 {
		country = selected[0].Country // registered drivers pick within their country
	}
	if country == "" {
		country = h.deps.Brand.Country
	}
	list, err := h.deps.Areas.List(ctx, country)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	out := driverAreaChoices{Country: country, Areas: []areas.AreaRef{}, Selected: selected, Primary: primary}
	if out.Selected == nil {
		out.Selected = []areas.AreaRef{}
	}
	for _, a := range list {
		if a.Status == areas.StatusActive {
			out.Areas = append(out.Areas, areas.AreaRef{ID: a.ID, Code: a.Code, Name: a.Name, Country: a.Country})
		}
	}
	writeJSON(w, http.StatusOK, out)
}

type areaHandlers struct{ deps Deps }

func (h areaHandlers) countries(w http.ResponseWriter, req *http.Request) {
	list, err := h.deps.Areas.Countries(req.Context())
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": list})
}

func (h areaHandlers) list(w http.ResponseWriter, req *http.Request) {
	country := req.URL.Query().Get("country")
	if country == "" {
		country = h.deps.Brand.Country
	}
	list, err := h.deps.Areas.List(req.Context(), country)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": list})
}

func (h areaHandlers) get(w http.ResponseWriter, req *http.Request) {
	a, err := h.deps.Areas.Get(req.Context(), chi.URLParam(req, "id"))
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	if !allowedScope(claimsFrom(req.Context()), a.Country, a.ID) {
		writeError(w, req, ErrScopeForbidden, nil)
		return
	}
	writeJSON(w, http.StatusOK, a)
}

func (h areaHandlers) create(w http.ResponseWriter, req *http.Request) {
	var body areas.CreateInput
	if !decode(w, req, &body) {
		return
	}
	if !allowedScope(claimsFrom(req.Context()), strings.ToUpper(body.Country), "") {
		writeError(w, req, ErrScopeForbidden, nil)
		return
	}
	a, err := h.deps.Areas.Create(req.Context(), claimsFrom(req.Context()).Subject, body)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusCreated, a)
}

func (h areaHandlers) update(w http.ResponseWriter, req *http.Request) {
	var body areas.UpdateInput
	if !decode(w, req, &body) || !h.mayTouch(w, req) {
		return
	}
	a, err := h.deps.Areas.Update(req.Context(), claimsFrom(req.Context()).Subject, chi.URLParam(req, "id"), body)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, a)
}

// cellsBody is the body of PUT /admin/areas/{id}/cells.
type cellsBody struct {
	Cells  []string `json:"cells"`
	Reason string   `json:"reason"`
}

func (h areaHandlers) saveCells(w http.ResponseWriter, req *http.Request) {
	var body cellsBody
	if !decode(w, req, &body) || !h.mayTouch(w, req) {
		return
	}
	a, err := h.deps.Areas.SaveCells(req.Context(), claimsFrom(req.Context()).Subject, chi.URLParam(req, "id"), body.Cells, body.Reason)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, a)
}

// driverAreasBody is the body of PUT /admin/drivers/{id}/areas.
type driverAreasBody struct {
	AreaIDs       []string `json:"area_ids"`
	PrimaryAreaID string   `json:"primary_area_id"`
	Reason        string   `json:"reason"`
}

func (h areaHandlers) setDriverAreas(w http.ResponseWriter, req *http.Request) {
	var body driverAreasBody
	if !decode(w, req, &body) {
		return
	}
	refs, err := h.deps.Areas.SetDriverAreas(req.Context(), claimsFrom(req.Context()).Subject, chi.URLParam(req, "id"), body.AreaIDs, body.PrimaryAreaID, body.Reason)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"areas": refs})
}

// mayTouch refuses changes to an area outside the caller's countries and regions.
func (h areaHandlers) mayTouch(w http.ResponseWriter, req *http.Request) bool {
	a, err := h.deps.Areas.Get(req.Context(), chi.URLParam(req, "id"))
	if err != nil {
		writeDomainError(w, req, err)
		return false
	}
	if !allowedScope(claimsFrom(req.Context()), a.Country, a.ID) {
		writeError(w, req, ErrScopeForbidden, nil)
		return false
	}
	return true
}
