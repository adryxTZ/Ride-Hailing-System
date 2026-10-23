package httpx

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/kalen-limited/hauler-rider/backend/internal/branding"
	"github.com/kalen-limited/hauler-rider/backend/internal/identity"
)

// mountBranding registers the Branding page (identity, theme versions, logos
// and icons) for staff with branding.manage, and the public asset links the
// apps, receipts and emails load logos from.
func mountBranding(r chi.Router, d Deps) {
	if d.Branding == nil {
		return
	}
	h := brandingHandlers{deps: d}
	r.Get("/brand/assets/*", h.asset)
	r.Group(func(g chi.Router) {
		g.Use(requireAuth(d), requireStaff(), requirePermission(identity.PermBrandingManage))
		g.Get("/admin/branding", h.overview)
		g.Put("/admin/branding/identity", h.setIdentity)
		g.Post("/admin/branding/preview", h.preview)
		g.Post("/admin/branding/import", h.importColorHunt)
		g.Post("/admin/branding/assets/upload-url", h.uploadURL)
		g.Post("/admin/branding/themes", h.createDraft)
		g.Get("/admin/branding/themes/{id}", h.theme)
		g.Put("/admin/branding/themes/{id}", h.updateDraft)
		g.Delete("/admin/branding/themes/{id}", h.deleteDraft)
		g.Post("/admin/branding/themes/{id}/publish", h.publish)
	})
}

type brandingHandlers struct{ deps Deps }

// brandingOverview is everything the Branding page opens with.
type brandingOverview struct {
	Identity branding.Identity       `json:"identity"`
	Versions []branding.ThemeVersion `json:"versions"`
	Presets  any                     `json:"presets"`
	Fonts    []string                `json:"fonts"`
	Slots    []string                `json:"asset_slots"`
	ShapeMax map[string]int          `json:"shape_max"`
}

func (h brandingHandlers) overview(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	id, err := h.deps.Branding.Identity(ctx)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	versions, err := h.deps.Branding.Versions(ctx)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, brandingOverview{Identity: id, Versions: versions, Presets: h.deps.Branding.Presets(),
		Fonts: branding.Fonts, Slots: branding.AssetSlots, ShapeMax: branding.ShapeMax})
}

func (h brandingHandlers) setIdentity(w http.ResponseWriter, req *http.Request) {
	var body branding.IdentityInput
	if !decode(w, req, &body) {
		return
	}
	id, err := h.deps.Branding.SetIdentity(req.Context(), claimsFrom(req.Context()).Subject, body)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, id)
}

func (h brandingHandlers) preview(w http.ResponseWriter, req *http.Request) {
	var body branding.Roles
	if !decode(w, req, &body) {
		return
	}
	writeJSON(w, http.StatusOK, branding.PreviewRoles(body))
}

type importBody struct {
	URL string `json:"url"`
}

func (h brandingHandlers) importColorHunt(w http.ResponseWriter, req *http.Request) {
	var body importBody
	if !decode(w, req, &body) {
		return
	}
	colours, roles, err := branding.ImportColorHunt(body.URL)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"colours": colours, "roles": roles})
}

type uploadBody struct {
	ContentType string `json:"content_type"`
}

func (h brandingHandlers) uploadURL(w http.ResponseWriter, req *http.Request) {
	var body uploadBody
	if !decode(w, req, &body) {
		return
	}
	t, err := h.deps.Branding.UploadURL(req.Context(), body.ContentType)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (h brandingHandlers) createDraft(w http.ResponseWriter, req *http.Request) {
	var body branding.DraftInput
	if !decode(w, req, &body) {
		return
	}
	t, err := h.deps.Branding.CreateDraft(req.Context(), claimsFrom(req.Context()).Subject, body)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusCreated, t)
}

func (h brandingHandlers) theme(w http.ResponseWriter, req *http.Request) {
	t, err := h.deps.Branding.Get(req.Context(), chi.URLParam(req, "id"))
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (h brandingHandlers) updateDraft(w http.ResponseWriter, req *http.Request) {
	var body branding.DraftInput
	if !decode(w, req, &body) {
		return
	}
	t, err := h.deps.Branding.UpdateDraft(req.Context(), claimsFrom(req.Context()).Subject, chi.URLParam(req, "id"), body)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (h brandingHandlers) deleteDraft(w http.ResponseWriter, req *http.Request) {
	if err := h.deps.Branding.DeleteDraft(req.Context(), claimsFrom(req.Context()).Subject, chi.URLParam(req, "id")); err != nil {
		writeDomainError(w, req, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h brandingHandlers) publish(w http.ResponseWriter, req *http.Request) {
	var body reasonBody
	if !decode(w, req, &body) {
		return
	}
	t, err := h.deps.Branding.Publish(req.Context(), claimsFrom(req.Context()).Subject, chi.URLParam(req, "id"), body.Reason)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

// assetCacheS: asset keys are unique per upload, so a link never changes content.
const assetCacheS = "public, max-age=3600"

// asset redirects to a short-lived link for a published logo or icon (public).
func (h brandingHandlers) asset(w http.ResponseWriter, req *http.Request) {
	link, err := h.deps.Branding.AssetLink(req.Context(), chi.URLParam(req, "*"))
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	w.Header().Set("Cache-Control", assetCacheS)
	http.Redirect(w, req, link, http.StatusFound)
}
