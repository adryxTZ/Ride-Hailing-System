package httpx

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/kalen-limited/hauler-rider/backend/internal/fleet"
	"github.com/kalen-limited/hauler-rider/backend/internal/identity"
)

// mountDocuments registers driver documents: staff choose per country which
// documents are mandatory, optional or off and how expiry is treated, follow
// expiring and expired documents, and correct dates; approved drivers see their
// documents and upload renewals.
func mountDocuments(r chi.Router, d Deps) {
	if d.Fleet == nil {
		return
	}
	h := documentHandlers{deps: d}
	r.Group(func(g chi.Router) {
		g.Use(requireAuth(d), requireApp("driver"))
		g.Get("/driver/documents", h.mine)
		g.Post("/driver/documents/renewals", h.renew)
	})
	r.Group(func(g chi.Router) {
		g.Use(requireAuth(d), requireStaff(), scopeGuard)
		manage := requirePermission(identity.PermComplianceManage)
		g.With(manage).Get("/admin/compliance/document-types", h.types)
		g.With(manage).Put("/admin/compliance/document-types", h.saveType)
		g.With(manage).Get("/admin/compliance/documents", h.list)
		g.With(requirePermission(identity.PermDriversReview)).Patch("/admin/drivers/{id}/documents/{docId}/expiry", h.setExpiry)
	})
}

type documentHandlers struct{ deps Deps }

func (h documentHandlers) mine(w http.ResponseWriter, req *http.Request) {
	docs, err := h.deps.Fleet.Documents(req.Context(), claimsFrom(req.Context()).Subject)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, docs)
}

func (h documentHandlers) renew(w http.ResponseWriter, req *http.Request) {
	var body fleet.DocumentInput
	if !decode(w, req, &body) {
		return
	}
	docs, err := h.deps.Fleet.Renew(req.Context(), claimsFrom(req.Context()).Subject, body)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusCreated, docs)
}

func (h documentHandlers) types(w http.ResponseWriter, req *http.Request) {
	list, err := h.deps.Fleet.DocumentTypes(req.Context(), scopeFrom(req, h.deps).Country)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": list})
}

type documentTypeBody struct {
	fleet.DocumentType
	Reason string `json:"reason"`
}

// saveType changes one country's document type (only staff whose scope covers the country).
func (h documentHandlers) saveType(w http.ResponseWriter, req *http.Request) {
	var body documentTypeBody
	if !decode(w, req, &body) {
		return
	}
	if !allowedScope(claimsFrom(req.Context()), strings.ToUpper(body.Country), "") {
		writeError(w, req, ErrScopeForbidden, nil)
		return
	}
	list, err := h.deps.Fleet.SaveDocumentType(req.Context(), claimsFrom(req.Context()).Subject, body.DocumentType, body.Reason)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": list})
}

func (h documentHandlers) list(w http.ResponseWriter, req *http.Request) {
	list, err := h.deps.Fleet.AdminDocuments(req.Context(), scopeFrom(req, h.deps).Country, req.URL.Query().Get("state"))
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": list})
}

type expiryBody struct {
	ExpiresOn string `json:"expires_on"`
	Reason    string `json:"reason"`
}

func (h documentHandlers) setExpiry(w http.ResponseWriter, req *http.Request) {
	var body expiryBody
	if !decode(w, req, &body) {
		return
	}
	d, err := h.deps.Fleet.SetExpiry(req.Context(), claimsFrom(req.Context()).Subject, chi.URLParam(req, "id"), chi.URLParam(req, "docId"), body.ExpiresOn, body.Reason)
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}
