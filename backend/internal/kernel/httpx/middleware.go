package httpx

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/i18n"
)

const messagesKey ctxKey = 2

// withMessages attaches the i18n catalogue so error writers can localise.
func withMessages(cat *i18n.Catalogue) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), messagesKey, cat)))
		})
	}
}

func messagesFrom(ctx context.Context) *i18n.Catalogue {
	c, _ := ctx.Value(messagesKey).(*i18n.Catalogue)
	return c
}

// requestLogger emits one structured log line per request with the fields
// dashboards and tracing correlate on (request_id, status, latency).
func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		slog.Info("http",
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.Status(),
			"ms", time.Since(start).Milliseconds(),
			"ip", r.RemoteAddr,
			"request_id", middleware.GetReqID(r.Context()),
		)
	})
}
