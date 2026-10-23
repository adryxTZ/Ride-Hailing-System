package httpx

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/kalen-limited/hauler-rider/backend/internal/identity"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/auth"
)

// ErrScopeForbidden is returned when staff ask for a country or region outside their access.
var ErrScopeForbidden = apperr.New("SCOPE_FORBIDDEN", http.StatusForbidden, "")

// requirePermission admits staff whose token carries the permission (or "*").
func requirePermission(perm string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if hasPermission(claimsFrom(req.Context()), perm) {
				next.ServeHTTP(w, req)
				return
			}
			writeError(w, req, apperr.Forbidden, map[string]any{"required_permission": perm})
		})
	}
}

func hasPermission(c *auth.Claims, perm string) bool {
	return c != nil && (slices.Contains(c.Roles, identity.PermissionPrefix+identity.PermAll) || slices.Contains(c.Roles, identity.PermissionPrefix+perm))
}

// staffAccess is the countries and regions a token may look at; empty = everywhere.
func staffAccess(c *auth.Claims) []identity.Access {
	var out []identity.Access
	if c == nil {
		return out
	}
	for _, r := range c.Roles {
		if v, ok := strings.CutPrefix(r, identity.ScopePrefix); ok {
			country, area, _ := strings.Cut(v, "/")
			out = append(out, identity.Access{Country: country, AreaID: area})
		}
	}
	return out
}

// allowedScope reports whether a token may look at a country and (optional) area.
func allowedScope(c *auth.Claims, country, areaID string) bool {
	access := staffAccess(c)
	if len(access) == 0 {
		return true
	}
	for _, a := range access {
		if a.Country == country && (a.AreaID == "" || a.AreaID == areaID) {
			return true
		}
	}
	return false
}

// scopeGuard keeps limited staff inside their countries and regions: a missing
// scope defaults to their first allowed one; an explicit one outside is refused.
func scopeGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		access := staffAccess(claimsFrom(req.Context()))
		if len(access) == 0 {
			next.ServeHTTP(w, req)
			return
		}
		q := req.URL.Query()
		country, area := strings.ToUpper(q.Get("country")), q.Get("area_id")
		if country == "" {
			country = access[0].Country
		}
		if area == "" { // a region-limited person always looks at their region
			for _, a := range access {
				if a.Country == country && a.AreaID != "" && !allowedScope(claimsFrom(req.Context()), country, "") {
					area = a.AreaID
					break
				}
			}
		}
		if !allowedScope(claimsFrom(req.Context()), country, area) {
			writeError(w, req, ErrScopeForbidden, nil)
			return
		}
		q.Set("country", country)
		if area != "" {
			q.Set("area_id", area)
		}
		req.URL.RawQuery = q.Encode()
		next.ServeHTTP(w, req)
	})
}

// gateTTL is how long a person's session gate is cached; ending a session or
// deactivating an account takes effect within this time.
const gateTTL = 5 * time.Second

type gateEntry struct {
	gate identity.SessionGate
	at   time.Time
}

// gateCache keeps one gate per person for gateTTL.
type gateCache struct {
	mu sync.Mutex
	m  map[string]gateEntry
}

var gates = &gateCache{m: map[string]gateEntry{}}

func (g *gateCache) get(ctx context.Context, d Deps, userID string) (identity.SessionGate, error) {
	g.mu.Lock()
	e, ok := g.m[userID]
	g.mu.Unlock()
	if ok && time.Since(e.at) < gateTTL {
		return e.gate, nil
	}
	gate, err := d.Identity.GateFor(ctx, userID)
	if err != nil {
		return gate, err
	}
	g.mu.Lock()
	g.m[userID] = gateEntry{gate: gate, at: time.Now()}
	g.mu.Unlock()
	return gate, nil
}

// sessionAlive refuses dashboard tokens of deactivated accounts and ended sessions.
func sessionAlive(ctx context.Context, d Deps, c *auth.Claims) bool {
	if c == nil || c.App != "admin" || d.Identity == nil {
		return true
	}
	g, err := gates.get(ctx, d, c.Subject)
	if err != nil {
		return true // fail open on a read error: the token is still signed and short-lived
	}
	return g.Active && !g.EndedSince[c.Session]
}

// forget drops a person's cached gate so a change applies on their next request.
func (g *gateCache) forget(userID string) {
	g.mu.Lock()
	delete(g.m, userID)
	g.mu.Unlock()
}
