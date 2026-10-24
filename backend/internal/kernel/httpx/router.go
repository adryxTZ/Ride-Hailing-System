package httpx

import (
	"context"
	"encoding/json"
	"github.com/kalen-limited/hauler-rider/backend/internal/accounts"
	"github.com/kalen-limited/hauler-rider/backend/internal/approvals"
	"github.com/kalen-limited/hauler-rider/backend/internal/branding"
	"github.com/kalen-limited/hauler-rider/backend/internal/compliance"
	"github.com/kalen-limited/hauler-rider/backend/internal/dispatch"
	"github.com/kalen-limited/hauler-rider/backend/internal/fleet"
	"github.com/kalen-limited/hauler-rider/backend/internal/fraud"
	"github.com/kalen-limited/hauler-rider/backend/internal/messaging"
	"github.com/kalen-limited/hauler-rider/backend/internal/money"
	"github.com/kalen-limited/hauler-rider/backend/internal/payments"
	"github.com/kalen-limited/hauler-rider/backend/internal/payouts"
	"github.com/kalen-limited/hauler-rider/backend/internal/people"
	"github.com/kalen-limited/hauler-rider/backend/internal/presence"
	"github.com/kalen-limited/hauler-rider/backend/internal/privacy"
	"github.com/kalen-limited/hauler-rider/backend/internal/profiles"
	"github.com/kalen-limited/hauler-rider/backend/internal/providers/geocoding"
	"github.com/kalen-limited/hauler-rider/backend/internal/providers/routing"
	"github.com/kalen-limited/hauler-rider/backend/internal/providers/storage"
	"github.com/kalen-limited/hauler-rider/backend/internal/realtime"
	"github.com/kalen-limited/hauler-rider/backend/internal/receipts"
	"github.com/kalen-limited/hauler-rider/backend/internal/refunds"
	"github.com/kalen-limited/hauler-rider/backend/internal/safety"
	"github.com/kalen-limited/hauler-rider/backend/internal/surge"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/kalen-limited/hauler-rider/backend/internal/areas"
	"github.com/kalen-limited/hauler-rider/backend/internal/catalog"
	"github.com/kalen-limited/hauler-rider/backend/internal/geofences"
	"github.com/kalen-limited/hauler-rider/backend/internal/holds"
	"github.com/kalen-limited/hauler-rider/backend/internal/identity"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/auth"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/brand"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/i18n"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/theme"
	"github.com/kalen-limited/hauler-rider/backend/internal/legal"
	"github.com/kalen-limited/hauler-rider/backend/internal/pricing"
	"github.com/kalen-limited/hauler-rider/backend/internal/profile"
	"github.com/kalen-limited/hauler-rider/backend/internal/trips"
)

// Deps is everything the HTTP layer needs from the rest of the system.
// Each field is a value or a narrow function so handlers stay testable
// without a database.
type Deps struct {
	Version       string                      // build version reported by /health
	Brand         *brand.Brand                // the brand this process serves
	Theme         *theme.Theme                // published theme for /app/config
	Presets       *theme.Presets              // preset gallery for /theme/presets
	Ready         func(context.Context) error // nil means "always ready"
	MinApp        string                      // minimum supported app version
	Features      map[string]bool             // feature flags exposed to clients
	Keys          *auth.KeySet                // token verification + JWKS
	Identity      *identity.Service           // sign-in use cases
	Catalog       *catalog.Catalog            // cached service catalogue
	Pricing       *pricing.Service            // quotes
	Profile       *profile.Service            // own profile + saved places
	Trips         *trips.Service              // booking + trip state
	Presence      *presence.Service           // live driver supply (nil = presence disabled)
	Hub           *realtime.Hub               // websocket fan-out (nil without Redis)
	Dispatch      *dispatch.Service           // offers, accept / decline (nil without Redis)
	Routing       routing.Provider            // road routes + ETAs (cached)
	Places        geocoding.Provider          // place search + reverse names (cached)
	Ledger        *money.Ledger               // driver wallets
	Messaging     *messaging.Service          // chat, ratings, push devices (nil = off)
	Fleet         *fleet.Service              // driver onboarding + staff review (nil = off)
	FileServer    storage.Server              // serves the local store's signed URLs (nil with a cloud store)
	Safety        *safety.Service             // trusted contacts, share links, SOS (nil = off)
	Surge         *surge.Service              // busy-area multipliers (nil without Redis)
	Areas         *areas.Service              // countries and operating areas (nil = off)
	Holds         *holds.Service              // driver debt and manual holds (nil = off)
	Payments      *payments.Service           // mobile-money debt pay-ins and fares (nil = off)
	Receipts      *receipts.Service           // trip receipts (nil = off)
	Payouts       *payouts.Service            // driver cash-outs (nil = off)
	Compliance    *compliance.Service         // regulator submissions (nil = off)
	People        *people.Service             // staff rider and driver pages (nil = off)
	Branding      *branding.Service           // identity and published theme for app config, Branding page (nil = brand file only)
	Profiles      *profiles.Service           // pricing profiles, payees, trip pricing records (nil = off)
	Geofences     *geofences.Service          // districts, zones and places (nil = off)
	Simulator     PricingSimulator            // admin pricing simulator (nil = off)
	Fraud         *fraud.Service              // fraud flags and the review queue (nil = off)
	Privacy       *privacy.Service            // consent, privacy requests, retention (nil = off)
	Refunds       *refunds.Service            // staff refunds on completed trips (nil = off)
	Accounts      *accounts.Service           // Finance › Accounts: balances per party, payments (nil = off)
	Legal         *legal.Service              // legal texts: public pages and the Legal page (nil = off)
	Approvals     *approvals.Service          // approval workflows and the inbox (nil = every change applies at once)
	PublicAPIURL  string                      // public API base for links (receipt PDFs); "" = the request's host
	PublicWebURL  string                      // base of public links (share pages); empty = the request's own host
	CancelReasons json.RawMessage             // seed/cancel_reasons.json, served verbatim
	Messages      *i18n.Catalogue             // localised error messages
	CORS          []string                    // allowed browser origins; empty = none, "*" = any (dev)
}

// New returns the fully wired HTTP handler. Routes are grouped by audience;
// each group lives in its own handlers_<group>.go file.
func New(d Deps) http.Handler {
	r := chi.NewRouter()
	r.Use(
		middleware.RequestID,
		middleware.RealIP,
		middleware.Recoverer,
		middleware.Timeout(30*time.Second),
		requestLogger,
	)

	r.Use(withMessages(d.Messages))
	if len(d.CORS) > 0 {
		r.Use(cors(d.CORS))
	}

	r.Route("/v1", func(v1 chi.Router) {
		mountPublic(v1, d)
		mountCatalog(v1, d)
		mountQuotes(v1, d)
		mountAuth(v1, d)
		mountMe(v1, d)
		mountTrips(v1, d)
		mountDriver(v1, d)
		mountNearby(v1, d)
		mountDriverTrips(v1, d)
		mountRealtime(v1, d)
		mountRoutes(v1, d)
		mountPlaces(v1, d)
		mountConfig(v1, d)
		mountMessaging(v1, d)
		mountFiles(v1, d)
		mountOnboarding(v1, d)
		mountAdminDrivers(v1, d)
		mountSafety(v1, d)
		mountAdminOps(v1, d)
		mountPayments(v1, d)
		mountTripPayment(v1, d)
		mountReceipts(v1, d)
		mountPayouts(v1, d)
		mountCompliance(v1, d)
		mountDocuments(v1, d)
		mountAreas(v1, d)
		mountAdminStaff(v1, d)
		mountPeople(v1, d)
		mountBranding(v1, d)
		mountProfiles(v1, d)
		mountPricingHistory(v1, d)
		mountFraud(v1, d)
		mountPrivacy(v1, d)
		mountTripMoney(v1, d)
		mountGeofences(v1, d)
		mountAccounts(v1, d)
		mountLegal(v1, d)
		mountApprovals(v1, d)
	})

	mountSharePage(r, d) // public live-trip page at /share/{token}

	r.NotFound(func(w http.ResponseWriter, req *http.Request) {
		writeError(w, req, apperr.NotFound, nil)
	})
	return r
}
