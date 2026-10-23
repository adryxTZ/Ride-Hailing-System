package httpx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/auth"
)

// scopeRequest runs scopeGuard for a token with the given roles and query,
// returning the status and the query the handler finally saw.
func scopeRequest(t *testing.T, roles []string, query string) (int, string) {
	t.Helper()
	var seen string
	h := scopeGuard(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		seen = req.URL.RawQuery
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/admin/overview?"+query, nil)
	req = req.WithContext(context.WithValue(req.Context(), claimsKey, &auth.Claims{Roles: roles}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code, seen
}

func TestScopeGuard(t *testing.T) {
	kenya := []string{"staff:Operations", "scope:KE"}
	nairobi := []string{"staff:Operations", "scope:KE/area-nbo"}
	cases := []struct {
		name   string
		roles  []string
		query  string
		status int
		want   string
	}{
		{"unlimited staff pass untouched", []string{"staff:Super Admin"}, "country=TZ", http.StatusOK, "country=TZ"},
		{"missing scope defaults to the allowed country", kenya, "", http.StatusOK, "country=KE"},
		{"another country is refused", kenya, "country=TZ", http.StatusForbidden, ""},
		{"a region inside the country is allowed", kenya, "country=KE&area_id=area-msa", http.StatusOK, "area_id=area-msa&country=KE"},
		{"region-limited staff are pinned to their region", nairobi, "country=KE", http.StatusOK, "area_id=area-nbo&country=KE"},
		{"region-limited staff cannot see another region", nairobi, "country=KE&area_id=area-msa", http.StatusForbidden, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status, seen := scopeRequest(t, c.roles, c.query)
			if status != c.status || seen != c.want {
				t.Fatalf("got %d %q, want %d %q", status, seen, c.status, c.want)
			}
		})
	}
}
