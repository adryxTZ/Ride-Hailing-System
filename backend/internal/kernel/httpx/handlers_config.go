package httpx

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// mountConfig registers small reference lists the apps render as-is.
func mountConfig(r chi.Router, d Deps) {
	r.Get("/config/cancel-reasons", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=3600")
		writeJSON(w, http.StatusOK, d.CancelReasons)
	})
}
