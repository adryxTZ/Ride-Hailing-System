package httpx

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/kalen-limited/hauler-rider/backend/internal/identity"
	"github.com/kalen-limited/hauler-rider/backend/internal/legal"
)

// mountLegal registers the public legal pages (HTML or JSON) and the Legal
// page's routes for staff with legal.manage (drafts, publish, details, preview).
func mountLegal(r chi.Router, d Deps) {
	if d.Legal == nil {
		return
	}
	h := legalHandlers{deps: d}
	r.Get("/legal/{app}/{kind}", h.page)
	r.Group(func(g chi.Router) {
		g.Use(requireAuth(d), requireStaff(), requirePermission(identity.PermLegalManage))
		g.Get("/admin/legal", h.overview)
		g.Put("/admin/legal/details", h.setDetails)
		g.Post("/admin/legal/preview", h.preview)
		g.Get("/admin/legal/{app}/{kind}", h.versions)
		g.Put("/admin/legal/{app}/{kind}/draft", h.saveDraft)
		g.Delete("/admin/legal/{app}/{kind}/draft", h.deleteDraft)
		g.Post("/admin/legal/{app}/{kind}/publish", h.publish)
	})
}

type legalHandlers struct{ deps Deps }

// docParams reads {app} and {kind}; a document the app does not have is a 404.
func docParams(w http.ResponseWriter, req *http.Request) (string, string, bool) {
	app, okApp := legal.ParseApp(chi.URLParam(req, "app"))
	kind, okKind := legal.ParseKind(chi.URLParam(req, "kind"))
	if !okApp || !okKind || !legal.Offered(app, kind) {
		writeError(w, req, legal.ErrNotFound, nil)
		return "", "", false
	}
	return app, kind, true
}

// page serves a live document: sanitised HTML, or JSON with ?format=json.
func (h legalHandlers) page(w http.ResponseWriter, req *http.Request) {
	app, kind, ok := docParams(w, req)
	if !ok {
		return
	}
	lang := req.URL.Query().Get("lang")
	if cat := messagesFrom(req.Context()); lang == "" && cat != nil {
		lang = cat.Pick(req.Header.Get("Accept-Language"))
	}
	p, err := h.deps.Legal.Page(req.Context(), app, kind, lang)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	w.Header().Set("Vary", "Accept-Language")
	if req.URL.Query().Get("format") == "json" {
		writeJSONCached(w, req, p, legalCacheS)
		return
	}
	writeLegalPage(w, req, h.deps, p)
}

// legalOverview is what the Legal page opens with.
type legalOverview struct {
	Documents     []legal.Overview         `json:"documents"`
	Details       legal.Details            `json:"details"`
	Placeholders  []legal.PlaceholderValue `json:"placeholders"`
	Locales       []string                 `json:"locales"`
	DefaultLocale string                   `json:"default_locale"`
}

func (h legalHandlers) overview(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	docs, err := h.deps.Legal.Overview(ctx)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	det, err := h.deps.Legal.Details(ctx)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	vals, err := h.deps.Legal.PlaceholderValues(ctx, h.deps.Brand.DefaultLocale)
	respond(w, req, http.StatusOK, legalOverview{Documents: docs, Details: det, Placeholders: vals,
		Locales: h.deps.Brand.Locales, DefaultLocale: h.deps.Brand.DefaultLocale}, err)
}

// legalVersions is one document's history for the editor.
type legalVersions struct {
	App      string           `json:"app"`
	Kind     string           `json:"kind"`
	Slug     string           `json:"slug"`
	Consent  bool             `json:"consent"`
	PageURL  string           `json:"page_url"`
	Versions []legal.Document `json:"versions"`
}

func (h legalHandlers) versions(w http.ResponseWriter, req *http.Request) {
	app, kind, ok := docParams(w, req)
	if !ok {
		return
	}
	list, err := h.deps.Legal.Versions(req.Context(), app, kind)
	info, _ := legal.Info(kind)
	page := apiLink(req, h.deps, legal.PagePath(app, kind, h.deps.Brand.DefaultLocale))
	respond(w, req, http.StatusOK, legalVersions{App: app, Kind: kind, Slug: info.Slug, Consent: info.Consent, PageURL: page, Versions: list}, err)
}

func (h legalHandlers) saveDraft(w http.ResponseWriter, req *http.Request) {
	app, kind, ok := docParams(w, req)
	if !ok {
		return
	}
	var body legal.DraftInput
	if !decode(w, req, &body) {
		return
	}
	d, err := h.deps.Legal.SaveDraft(req.Context(), claimsFrom(req.Context()).Subject, app, kind, body)
	respond(w, req, http.StatusOK, d, err)
}

func (h legalHandlers) deleteDraft(w http.ResponseWriter, req *http.Request) {
	app, kind, ok := docParams(w, req)
	if !ok {
		return
	}
	if err := h.deps.Legal.DeleteDraft(req.Context(), claimsFrom(req.Context()).Subject, app, kind); err != nil {
		writeDomainError(w, req, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h legalHandlers) publish(w http.ResponseWriter, req *http.Request) {
	app, kind, ok := docParams(w, req)
	if !ok {
		return
	}
	var body reasonBody
	if !decode(w, req, &body) {
		return
	}
	d, err := h.deps.Legal.Publish(req.Context(), claimsFrom(req.Context()).Subject, app, kind, body.Reason)
	respond(w, req, http.StatusOK, d, err)
}

func (h legalHandlers) setDetails(w http.ResponseWriter, req *http.Request) {
	var body legal.DetailsInput
	if !decode(w, req, &body) {
		return
	}
	d, err := h.deps.Legal.SetDetails(req.Context(), claimsFrom(req.Context()).Subject, body)
	respond(w, req, http.StatusOK, d, err)
}

// previewBody is an unsaved text to render in one language.
type previewBody struct {
	App   string     `json:"app"`
	Kind  string     `json:"kind"`
	Lang  string     `json:"lang"`
	Title legal.Text `json:"title"`
	Body  legal.Text `json:"body"`
}

func (h legalHandlers) preview(w http.ResponseWriter, req *http.Request) {
	var body previewBody
	if !decode(w, req, &body) {
		return
	}
	app, _ := legal.ParseApp(body.App)
	kind, _ := legal.ParseKind(body.Kind)
	p, err := h.deps.Legal.Preview(req.Context(), app, kind, strings.ToLower(body.Lang), legal.DraftInput{Title: body.Title, Body: body.Body})
	respond(w, req, http.StatusOK, p, err)
}
