package httpx

import (
	"errors"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
	"github.com/kalen-limited/hauler-rider/backend/internal/providers/storage"
)

// mountFiles serves the local store's signed URLs (development and single-box
// installs). The signature is the authorisation: no bearer token.
func mountFiles(r chi.Router, d Deps) {
	if d.FileServer == nil {
		return
	}
	h := fileHandlers{srv: d.FileServer}
	r.Put("/files/*", h.put)
	r.Get("/files/*", h.get)
}

type fileHandlers struct{ srv storage.Server }

var errBadSignature = apperr.New("BAD_SIGNATURE", http.StatusForbidden, "")

func (h fileHandlers) verified(w http.ResponseWriter, req *http.Request, method string) (string, bool) {
	key := chi.URLParam(req, "*")
	q := req.URL.Query()
	exp, _ := strconv.ParseInt(q.Get("exp"), 10, 64)
	if q.Get("m") != method || !h.srv.Verify(key, method, exp, q.Get("sig")) {
		writeError(w, req, errBadSignature, nil)
		return "", false
	}
	return key, true
}

func (h fileHandlers) put(w http.ResponseWriter, req *http.Request) {
	key, ok := h.verified(w, req, http.MethodPut)
	if !ok {
		return
	}
	if _, allowed := storage.AllowedTypes[strings.Split(req.Header.Get("Content-Type"), ";")[0]]; !allowed {
		writeError(w, req, apperr.Validation, map[string]any{"fields": []map[string]string{{"path": "content_type", "code": "UNSUPPORTED"}}})
		return
	}
	if err := h.srv.Write(key, req.Body, storage.MaxUploadBytes); err != nil {
		writeError(w, req, apperr.New("UPLOAD_FAILED", http.StatusBadRequest, ""), nil)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h fileHandlers) get(w http.ResponseWriter, req *http.Request) {
	key, ok := h.verified(w, req, http.MethodGet)
	if !ok {
		return
	}
	f, ctype, err := h.srv.Open(key)
	if errors.Is(err, os.ErrNotExist) {
		writeError(w, req, apperr.NotFound, nil)
		return
	}
	if err != nil {
		writeError(w, req, apperr.Internal, nil)
		return
	}
	defer func() { _ = f.Close() }()
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Cache-Control", "private, max-age=300")
	_, _ = io.Copy(w, f)
}
