package httpx

import (
	"net/http"
	"slices"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
)

// staffRole is the account role every admin-dashboard user carries.
const staffRole = "STAFF"

// requireStaff admits tokens issued to the admin app for staff accounts.
func requireStaff() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			c := claimsFrom(req.Context())
			if c == nil || c.App != "admin" || !slices.Contains(c.Roles, staffRole) {
				writeError(w, req, apperr.Forbidden, map[string]any{"required": "staff"})
				return
			}
			next.ServeHTTP(w, req)
		})
	}
}
