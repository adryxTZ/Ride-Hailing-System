package httpx

import (
	"net/http"
	"slices"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/kalen-limited/hauler-rider/backend/internal/identity"
	"github.com/kalen-limited/hauler-rider/backend/internal/people"
	"github.com/kalen-limited/hauler-rider/backend/internal/trips"
)

// mountPeople registers the staff rider and driver pages: the figures, ending
// a person's sessions, and staff notes. Staff limited to countries only see
// people registered or riding there.
func mountPeople(r chi.Router, d Deps) {
	if d.People == nil {
		return
	}
	h := peopleHandlers{deps: d}
	r.Group(func(g chi.Router) {
		g.Use(requireAuth(d), requireStaff(), personID)
		g.Get("/admin/riders/{id}", h.rider)
		g.Get("/admin/drivers/{id}/activity", h.driver)
		g.Get("/admin/riders/{id}/charts", h.charts(trips.AsRider))
		g.Get("/admin/drivers/{id}/charts", h.charts(trips.AsDriver))
		g.With(requirePermission(identity.PermRidersBlock)).Post("/admin/riders/{id}/sessions/end", h.endSessions(h.rider0))
		g.With(requirePermission(identity.PermDriversReview)).Post("/admin/drivers/{id}/sessions/end", h.endSessions(h.driver0))
		g.Post("/admin/people/{id}/notes", h.addNote)
	})
}

type peopleHandlers struct{ deps Deps }

// personID answers 404 for an {id} that is not a UUID, before any query runs.
func personID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if _, err := uuid.Parse(chi.URLParam(req, "id")); err != nil {
			writeError(w, req, identity.ErrPersonNotFound, nil)
			return
		}
		next.ServeHTTP(w, req)
	})
}

// allowed is the staff member's country check (nil = every country).
func allowed(req *http.Request) people.Allowed {
	access := staffAccess(claimsFrom(req.Context()))
	if len(access) == 0 {
		return nil
	}
	return func(country string) bool {
		return slices.ContainsFunc(access, func(a identity.Access) bool { return a.Country == country })
	}
}

func (h peopleHandlers) rider(w http.ResponseWriter, req *http.Request) {
	r, err := h.deps.People.Rider(req.Context(), chi.URLParam(req, "id"), allowed(req))
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, r)
}

func (h peopleHandlers) driver(w http.ResponseWriter, req *http.Request) {
	d, err := h.deps.People.Driver(req.Context(), chi.URLParam(req, "id"), allowed(req))
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

// charts serves a person page's Charts tab (?days=, default 90, at most 365).
func (h peopleHandlers) charts(role string) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		c, err := h.deps.People.Charts(req.Context(), role, chi.URLParam(req, "id"), queryInt(req, "days"), allowed(req))
		if err != nil {
			writeDomainError(w, req, err)
			return
		}
		writeJSON(w, http.StatusOK, c)
	}
}

// rider0 / driver0 check the person is in the staff member's scope before a write.
func (h peopleHandlers) rider0(req *http.Request, id string) error {
	return h.deps.People.Check(req.Context(), trips.AsRider, id, allowed(req))
}

func (h peopleHandlers) driver0(req *http.Request, id string) error {
	return h.deps.People.Check(req.Context(), trips.AsDriver, id, allowed(req))
}

// endSessions signs a rider or driver out of one session or all of them, audited.
func (h peopleHandlers) endSessions(check func(*http.Request, string) error) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		var body endSessionsBody
		if !decode(w, req, &body) {
			return
		}
		id := chi.URLParam(req, "id")
		if err := check(req, id); err != nil {
			writeDomainError(w, req, err)
			return
		}
		if err := h.deps.Identity.EndSessions(req.Context(), claimsFrom(req.Context()).Subject, id, body.SessionID, body.Reason); err != nil {
			writeDomainError(w, req, err)
			return
		}
		gates.forget(id)
		w.WriteHeader(http.StatusNoContent)
	}
}

type noteBody struct {
	Body string `json:"body"`
}

func (h peopleHandlers) addNote(w http.ResponseWriter, req *http.Request) {
	var body noteBody
	if !decode(w, req, &body) {
		return
	}
	n, err := h.deps.Identity.AddNote(req.Context(), claimsFrom(req.Context()).Subject, chi.URLParam(req, "id"), body.Body)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusCreated, n)
}
