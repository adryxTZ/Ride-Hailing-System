package httpx

import (
	"context"
	"net/http"
	"strings"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/auth"
)

type ctxKey int

const claimsKey ctxKey = 1

// requireAuth verifies the bearer token and stores its claims in the context.
// Routes that need a specific role add a second check on top of this.
func requireAuth(d Deps) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			raw := strings.TrimPrefix(req.Header.Get("Authorization"), "Bearer ")
			if raw == "" || raw == req.Header.Get("Authorization") {
				writeError(w, req, apperr.Unauthenticated, nil)
				return
			}
			claims, err := d.Keys.Verify(raw)
			if err != nil || claims.Brand != d.Brand.Code || !sessionAlive(req.Context(), d, claims) {
				writeError(w, req, apperr.Unauthenticated, nil)
				return
			}
			next.ServeHTTP(w, req.WithContext(context.WithValue(req.Context(), claimsKey, claims)))
		})
	}
}

// claimsFrom returns the verified claims placed by requireAuth.
func claimsFrom(ctx context.Context) *auth.Claims {
	c, _ := ctx.Value(claimsKey).(*auth.Claims)
	return c
}

// optionalAuth attaches claims when a valid bearer token is present and
// otherwise lets the request through anonymously.
func optionalAuth(d Deps) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			raw := strings.TrimPrefix(req.Header.Get("Authorization"), "Bearer ")
			if raw != "" && raw != req.Header.Get("Authorization") {
				if claims, err := d.Keys.Verify(raw); err == nil && claims.Brand == d.Brand.Code {
					req = req.WithContext(context.WithValue(req.Context(), claimsKey, claims))
				}
			}
			next.ServeHTTP(w, req)
		})
	}
}
