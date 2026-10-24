package httpx

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/kalen-limited/hauler-rider/backend/internal/approvals"
	"github.com/kalen-limited/hauler-rider/backend/internal/approvals/actions"
	"github.com/kalen-limited/hauler-rider/backend/internal/identity"
	"github.com/kalen-limited/hauler-rider/backend/internal/privacy"
)

// consentCacheS: notices change rarely; clients revalidate with the ETag.
const consentCacheS = "public, max-age=300"

// mountPrivacy registers consent, a person's privacy requests and the staff queue.
func mountPrivacy(r chi.Router, d Deps) {
	if d.Privacy == nil {
		return
	}
	h := privacyHandlers{deps: d}
	r.Get("/legal/consent", h.document)
	r.Group(func(g chi.Router) {
		g.Use(requireAuth(d))
		g.Post("/me/consents", h.accept)
		g.Get("/me/privacy-requests", h.mine)
		g.Post("/me/privacy-requests", h.create)
		g.Post("/me/privacy-requests/{id}/cancel", h.cancel)
		g.Get("/me/privacy-requests/{id}/download", h.download)
	})
	r.Group(func(g chi.Router) {
		g.Use(requireAuth(d), requireStaff(), requirePermission(identity.PermPrivacyHandle))
		g.With(scopeGuard).Get("/admin/privacy/requests", h.list)
		g.Get("/admin/privacy/requests/{id}", h.get)
		g.Post("/admin/privacy/requests/{id}/approve", h.approve)
		g.Post("/admin/privacy/requests/{id}/reject", h.reject)
		g.Get("/admin/privacy/retention", h.retention)
	})
}

type privacyHandlers struct{ deps Deps }

// consentGate refuses an action until the person accepted their app's current notice.
func consentGate(w http.ResponseWriter, req *http.Request, d Deps) bool {
	if d.Privacy == nil {
		return true
	}
	c := claimsFrom(req.Context())
	required, err := d.Privacy.Required(req.Context(), c.Subject, privacy.AppOf(c.App))
	if err != nil {
		writeDomainError(w, req, err)
		return false
	}
	if required {
		writeError(w, req, privacy.ErrConsentRequired, nil)
		return false
	}
	return true
}

// consentRequired is the sign-in flag the apps use to show the consent screen.
func consentRequired(req *http.Request, d Deps, pair *identity.TokenPair, app string) {
	if d.Privacy == nil || pair == nil {
		return
	}
	claims, err := d.Keys.Verify(pair.AccessToken)
	if err != nil {
		return
	}
	pair.ConsentRequired, _ = d.Privacy.Required(req.Context(), claims.Subject, privacy.AppOf(app))
}

func (h privacyHandlers) document(w http.ResponseWriter, req *http.Request) {
	lang := "en"
	if cat := messagesFrom(req.Context()); cat != nil {
		lang = cat.Pick(req.Header.Get("Accept-Language"))
	}
	doc, ok, err := h.deps.Privacy.Document(req.Context(), strings.ToUpper(req.URL.Query().Get("app")), lang)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	if !ok {
		writeError(w, req, privacy.ErrNotFound, nil)
		return
	}
	// The legal pages' links are stored relative to the API (it may sit at any address).
	doc.URL = apiLink(req, h.deps, doc.URL)
	for i := range doc.Links {
		doc.Links[i].URL = apiLink(req, h.deps, doc.Links[i].URL)
	}
	w.Header().Set("Vary", "Accept-Language")
	writeJSONCached(w, req, doc, consentCacheS)
}

type acceptBody struct {
	App     string `json:"app"`
	Version int    `json:"version"`
}

func (h privacyHandlers) accept(w http.ResponseWriter, req *http.Request) {
	var body acceptBody
	if !decode(w, req, &body) {
		return
	}
	c := claimsFrom(req.Context())
	app := strings.ToUpper(body.App)
	if app != privacy.AppOf(c.App) {
		writeError(w, req, identity.ErrAppMismatch, nil)
		return
	}
	cur, err := h.deps.Privacy.Accept(req.Context(), c.Subject, privacy.AcceptInput{App: app, Version: body.Version,
		InstallID: req.Header.Get(installHeader), IP: clientIP(req), UserAgent: req.UserAgent()})
	if err == privacy.ErrConsentStale {
		writeError(w, req, privacy.ErrConsentStale, map[string]any{"current_version": cur})
		return
	}
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h privacyHandlers) mine(w http.ResponseWriter, req *http.Request) {
	list, err := h.deps.Privacy.Mine(req.Context(), claimsFrom(req.Context()).Subject)
	respond(w, req, http.StatusOK, map[string]any{"data": list}, err)
}

type kindBody struct {
	Kind string `json:"kind"`
}

func (h privacyHandlers) create(w http.ResponseWriter, req *http.Request) {
	var body kindBody
	if !decode(w, req, &body) {
		return
	}
	c := claimsFrom(req.Context())
	r, err := h.deps.Privacy.Create(req.Context(), c.Subject, privacy.AppOf(c.App), strings.ToUpper(body.Kind))
	respond(w, req, http.StatusCreated, r, err)
}

func (h privacyHandlers) cancel(w http.ResponseWriter, req *http.Request) {
	r, err := h.deps.Privacy.Cancel(req.Context(), claimsFrom(req.Context()).Subject, chi.URLParam(req, "id"))
	respond(w, req, http.StatusOK, r, err)
}

func (h privacyHandlers) download(w http.ResponseWriter, req *http.Request) {
	url, expires, err := h.deps.Privacy.Download(req.Context(), claimsFrom(req.Context()).Subject, chi.URLParam(req, "id"))
	respond(w, req, http.StatusOK, map[string]any{"url": url, "expires_at": expires}, err)
}

func (h privacyHandlers) list(w http.ResponseWriter, req *http.Request) {
	q := req.URL.Query()
	if q.Get("country") == "" { // unlimited staff who picked no country see every country
		q.Set("country", "")
	}
	list, err := h.deps.Privacy.List(req.Context(), privacy.Filter{Status: strings.ToUpper(q.Get("status")), Kind: strings.ToUpper(q.Get("kind")),
		Limit: queryInt(req, "limit"), Country: strings.ToUpper(q.Get("country"))})
	respond(w, req, http.StatusOK, map[string]any{"data": list}, err)
}

// requestIn loads a request and refuses one outside the staff member's countries.
func (h privacyHandlers) requestIn(w http.ResponseWriter, req *http.Request) (*privacy.Request, bool) {
	r, err := h.deps.Privacy.Get(req.Context(), chi.URLParam(req, "id"))
	if err != nil {
		writeDomainError(w, req, err)
		return nil, false
	}
	country := ""
	if r.Country != nil {
		country = *r.Country
	}
	if !allowedScope(claimsFrom(req.Context()), country, "") {
		writeError(w, req, ErrScopeForbidden, nil)
		return nil, false
	}
	return r, true
}

func (h privacyHandlers) get(w http.ResponseWriter, req *http.Request) {
	if r, ok := h.requestIn(w, req); ok {
		writeJSON(w, http.StatusOK, r)
	}
}

func (h privacyHandlers) approve(w http.ResponseWriter, req *http.Request) {
	var body reasonBody
	if !decode(w, req, &body) {
		return
	}
	pr, ok := h.requestIn(w, req)
	if !ok || (pr.Kind == privacy.KindDelete && pr.Status == privacy.StatusRequested &&
		sentForApproval(w, req, h.deps, approvals.Submission{Action: actions.PrivacyDelete, Reason: body.Reason, Country: deref(pr.Country),
			Target: "privacy_request:" + pr.ID, Payload: actions.PrivacyPayload{RequestID: pr.ID}})) {
		return // exports are never gated; a closed request gets the service's answer
	}
	r, err := h.deps.Privacy.Approve(req.Context(), claimsFrom(req.Context()).Subject, chi.URLParam(req, "id"), body.Reason)
	respond(w, req, http.StatusOK, r, err)
}

func (h privacyHandlers) reject(w http.ResponseWriter, req *http.Request) {
	var body reasonBody
	if !decode(w, req, &body) {
		return
	}
	if _, ok := h.requestIn(w, req); !ok {
		return
	}
	r, err := h.deps.Privacy.Reject(req.Context(), claimsFrom(req.Context()).Subject, chi.URLParam(req, "id"), body.Reason)
	respond(w, req, http.StatusOK, r, err)
}

func (h privacyHandlers) retention(w http.ResponseWriter, req *http.Request) {
	rules, err := h.deps.Privacy.Retention(req.Context())
	respond(w, req, http.StatusOK, map[string]any{"data": rules}, err)
}
