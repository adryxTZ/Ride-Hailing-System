package httpx

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/kalen-limited/hauler-rider/backend/internal/catalog"
)

// mountCatalog registers the public catalogue routes.
func mountCatalog(r chi.Router, d Deps) {
	h := catalogHandlers{deps: d}
	r.Get("/services", h.services)
	r.Get("/services/{key}", h.service)
}

type catalogHandlers struct{ deps Deps }

// services returns every active service with its vehicle classes and
// request schema. ETag = catalogue version, so unchanged catalogues cost a 304.
func (h catalogHandlers) services(w http.ResponseWriter, req *http.Request) {
	cat, version, err := h.deps.Catalog.Catalogue(req.Context())
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	etag := `"catalog-` + strconv.FormatInt(version, 10) + `"`
	if req.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "public, max-age=60")
	writeJSON(w, http.StatusOK, listResponse[catalog.Service]{Data: activeOnly(cat.Services)})
}

// activeOnly hides HIDDEN services from riders (they stay reachable by key
// for staff previews and for apps that already know the key).
func activeOnly(in []catalog.Service) []catalog.Service {
	out := make([]catalog.Service, 0, len(in))
	for _, s := range in {
		if s.Status == "ACTIVE" {
			out = append(out, s)
		}
	}
	return out
}

// service returns one service by key.
func (h catalogHandlers) service(w http.ResponseWriter, req *http.Request) {
	svc, err := h.deps.Catalog.Find(req.Context(), chi.URLParam(req, "key"))
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	if svc == nil {
		writeError(w, req, notFound(), nil)
		return
	}
	writeJSON(w, http.StatusOK, svc)
}
