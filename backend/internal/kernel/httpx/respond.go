package httpx

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5/middleware"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
)

// writeJSON encodes v with the given status. All handlers go through here so
// content type and encoding are set in exactly one place.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeJSONWithETag serves v with a content-hash ETag and honours
// If-None-Match, returning 304 when the client already has this version.
func writeJSONWithETag(w http.ResponseWriter, req *http.Request, v any, maxAgeSeconds int) {
	writeJSONCached(w, req, v, "public, max-age="+strconv.Itoa(maxAgeSeconds))
}

// writeJSONCached is writeJSONWithETag with an explicit Cache-Control (e.g.
// "private, max-age=86400, immutable" for a receipt only its owner may cache).
func writeJSONCached(w http.ResponseWriter, req *http.Request, v any, cacheControl string) {
	body, err := json.Marshal(v)
	if err != nil {
		writeError(w, req, apperr.Internal, nil)
		return
	}
	sum := sha256.Sum256(body)
	etag := `"` + hex.EncodeToString(sum[:8]) + `"`
	if req.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", cacheControl)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

// writeError renders the Platform error envelope. The message is localised by
// Accept-Language through the catalogue; the code is stable and is what
// clients key on.
func writeError(w http.ResponseWriter, req *http.Request, e *apperr.E, details map[string]any) {
	writeJSON(w, e.HTTP, errorResponse{Error: errorBody{
		Code:      e.Code,
		Message:   localise(req, e),
		Details:   details,
		RequestID: middleware.GetReqID(req.Context()),
	}})
}

// localise picks the message for the caller's language; falls back to the
// error's built-in text when no catalogue is configured.
func localise(req *http.Request, e *apperr.E) string {
	cat := messagesFrom(req.Context())
	if cat == nil {
		return e.Message
	}
	return cat.T(cat.Pick(req.Header.Get("Accept-Language")), e.Code)
}

// logUnexpected records errors that reached the transport layer unmapped.
func logUnexpected(req *http.Request, err error) {
	slog.Error("unhandled error", "err", err, "path", req.URL.Path, "request_id", middleware.GetReqID(req.Context()))
}

// notFound is the shared 404 error (kept as a function so callers never mutate the shared value).
func notFound() *apperr.E { return apperr.NotFound }
