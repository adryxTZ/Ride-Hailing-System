package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/kalen-limited/hauler-rider/backend/internal/dispatch"
	"github.com/kalen-limited/hauler-rider/backend/internal/money"
	"github.com/kalen-limited/hauler-rider/backend/internal/presence"
	"github.com/kalen-limited/hauler-rider/backend/internal/providers/geocoding"
	"github.com/kalen-limited/hauler-rider/backend/internal/realtime"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"github.com/kalen-limited/hauler-rider/backend/internal/approvals/actions"
	"github.com/kalen-limited/hauler-rider/backend/internal/branding"
	"github.com/kalen-limited/hauler-rider/backend/internal/catalog"
	"github.com/kalen-limited/hauler-rider/backend/internal/identity"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/auth"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/httpx"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/i18n"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/phone"
	"github.com/kalen-limited/hauler-rider/backend/internal/pricing"
	"github.com/kalen-limited/hauler-rider/backend/internal/profile"
	"github.com/kalen-limited/hauler-rider/backend/internal/providers/routing"
	"github.com/kalen-limited/hauler-rider/backend/internal/providers/sms"
	"github.com/kalen-limited/hauler-rider/backend/internal/trips"
)

// runServe starts the roles requested on the command line. All roles share one
// process at launch; splitting them is a deployment choice, not a code change.
func runServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	rolesFlag := fs.String("roles", "api,realtime,worker,scheduler", "comma-separated roles to run")
	if err := fs.Parse(args); err != nil {
		return err
	}
	roles := parseRoles(*rolesFlag)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	bs, err := load(ctx, true)
	if err != nil {
		return err
	}
	defer bs.pool.Close()

	brandID, err := bs.brandID(ctx)
	if err != nil {
		return err
	}
	keys, err := auth.LoadOrGenerate(bs.cfg.JWTKeysDir)
	if err != nil {
		return fmt.Errorf("signing keys: %w", err)
	}
	messages, err := i18n.Load(bs.cfg.I18nDir, "en")
	if err != nil {
		return fmt.Errorf("i18n: %w", err)
	}
	th, _ := bs.presets.Resolve(bs.brand.ThemePreset)

	// Job queue: the client both enqueues (api role) and works (worker role).
	workers := river.NewWorkers()
	river.AddWorker(workers, &identity.SendOTPWorker{Sender: smsSender(bs), BrandName: bs.brand.DisplayName})
	mw, periodic := registerMoneyWorkers(workers)
	cw, compliancePeriodic := registerComplianceWorkers(workers)
	periodic = append(periodic, compliancePeriodic...)
	fw, fraudPeriodic := registerFraudWorkers(workers)
	periodic = append(periodic, fraudPeriodic...)
	pw, privacyPeriodic := registerPrivacyWorkers(workers)
	periodic = append(periodic, privacyPeriodic...)
	aw, approvalPeriodic := registerApprovalWorkers(workers)
	periodic = append(periodic, approvalPeriodic...)
	jobs, err := river.NewClient(riverpgxv5.New(bs.pool), &river.Config{
		Queues:       map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: 16}},
		Workers:      workers,
		PeriodicJobs: periodic,
	})
	if err != nil {
		return fmt.Errorf("river: %w", err)
	}
	if roles["worker"] {
		if err := jobs.Start(ctx); err != nil {
			return fmt.Errorf("start workers: %w", err)
		}
		defer func() { _ = jobs.Stop(context.Background()) }()
	}

	cat := catalog.New(bs.pool, brandID)
	ident := identity.New(bs.pool, keys, jobs, brandID, bs.brand.Code, bs.brand.Country, bs.brand.DefaultLocale)
	ident.DevEcho = bs.cfg.OTPDevEcho
	if ident.DevEcho {
		slog.Warn("OTP dev echo is ON — codes are returned in API responses; never enable in production")
	}
	if bs.cfg.OTPTestCode != "" {
		ident.TestCode = bs.cfg.OTPTestCode
		ident.TestNumbers = map[string]bool{}
		for _, n := range bs.cfg.OTPTestNumbers() {
			if p, err := phone.Normalize(n, bs.brand.Country); err == nil {
				ident.TestNumbers[p] = true
			}
		}
		if len(ident.TestNumbers) == 0 {
			slog.Warn("OTP test code applies to EVERY number; development only")
		} else {
			slog.Info("OTP test code enabled for allow-listed numbers", "count", len(ident.TestNumbers))
		}
	}

	router := routingProvider(bs)
	tripSvc := trips.New(bs.pool, brandID, cat)
	priceSvc := pricing.New(bs.pool, brandID, bs.brand.Country, cat, router)

	pricingProfiles, profileAreas := newProfiles(bs, brandID, cat, priceSvc, tripSvc)
	ledger := money.NewLedger(bs.pool, brandID)
	tripSvc.OnCompletedTx(postCashTripTx(ledger))
	tripSvc.OnChanged(postCompletedTrips(ledger))
	wireCancellationMoney(tripSvc, ledger)
	presenceSvc := presenceService(bs)
	hub, dispatcher := startRealtimeAndDispatch(ctx, bs, roles, tripSvc, presenceSvc, router)
	messagingSvc := startMessaging(bs, brandID, tripSvc, hub, dispatcher)
	surgeSvc := startSurge(bs, tripSvc, presenceSvc, dispatcher, priceSvc)
	files := fileStore(bs)
	fleetSvc := startFleet(bs, brandID, cat, files, presenceSvc)
	safetySvc := startSafety(bs, brandID, tripSvc, fleetSvc, presenceSvc)
	areaSvc := startAreas(bs, brandID, priceSvc, fleetSvc)
	profileAreas.svc = areaSvc
	geofenceSvc := newGeofences(bs, brandID, areaSvc, pricingProfiles)
	receiptSvc := startReceipts(bs, brandID, keys, tripSvc, fleetSvc)
	complianceSvc, err := startCompliance(bs, brandID, cw, cat, tripSvc, fleetSvc)
	if err != nil {
		return err
	}
	holdSvc, paySvc, payoutSvc, err := startMoney(bs, brandID, mw, jobs, ledger, priceSvc, presenceSvc, hub, tripSvc, ident)
	if err != nil {
		return err
	}
	documentRules(complianceSvc, fleetSvc, holdSvc, messagingSvc, smsSender(bs))
	fraudSvc := startFraud(bs, brandID, fw, jobs, tripSvc, presenceSvc)
	privacySvc, err := startPrivacy(ctx, bs, brandID, pw, jobs, files, ledger, presenceSvc)
	if err != nil {
		return err
	}
	profileSvc := profile.New(bs.pool)
	brandingSvc := branding.New(bs.pool, brandID, files, bs.presets, th)
	peopleSvc := newPeople(bs, peopleModules{ident: ident, trips: tripSvc, dispatch: dispatcher, msg: messagingSvc, safety: safetySvc,
		profile: profileSvc, presence: presenceSvc, ledger: ledger, holds: holdSvc, payouts: payoutSvc, payins: paySvc, fleet: fleetSvc})
	refundSvc := newRefunds(bs, brandID, ledger, tripSvc)
	approvalSvc := startApprovals(bs, brandID, aw, ident, actions.Modules{Profiles: pricingProfiles, Refunds: refundSvc, Payouts: payoutSvc,
		Privacy: privacySvc, Compliance: complianceSvc, Branding: brandingSvc})

	handler := httpx.New(httpx.Deps{
		Version:       Version,
		Brand:         bs.brand,
		Theme:         th,
		Presets:       bs.presets,
		Ready:         httpx.ReadyChecks(ctx, bs.cfg),
		MinApp:        bs.cfg.MinAppVersion,
		Features:      bs.cfg.Features(),
		Keys:          keys,
		Identity:      ident,
		Catalog:       cat,
		Pricing:       priceSvc,
		Routing:       router,
		Ledger:        ledger,
		Messaging:     messagingSvc,
		Fleet:         fleetSvc,
		Safety:        safetySvc,
		Surge:         surgeSvc,
		Areas:         areaSvc,
		Holds:         holdSvc,
		Payments:      paySvc,
		Receipts:      receiptSvc,
		Payouts:       payoutSvc,
		Compliance:    complianceSvc,
		PublicAPIURL:  os.Getenv("NEXA_PUBLIC_API_URL"),
		PublicWebURL:  os.Getenv("NEXA_PUBLIC_WEB_URL"),
		FileServer:    files,
		Places:        placesProvider(bs),
		Profile:       profileSvc,
		People:        peopleSvc,
		Branding:      brandingSvc,
		Profiles:      pricingProfiles,
		Geofences:     geofenceSvc,
		Simulator:     simulator{pricing: priceSvc},
		Fraud:         fraudSvc,
		Privacy:       privacySvc,
		Refunds:       refundSvc,
		Approvals:     approvalSvc,
		Trips:         tripSvc,
		Presence:      presenceSvc,
		Hub:           hub,
		Dispatch:      dispatcher,
		CancelReasons: mustRead(filepath.Join(bs.cfg.SeedDir, "cancel_reasons.json")),
		Messages:      messages,
		CORS:          bs.cfg.CORSOrigins(),
	})
	if !roles["api"] {
		// worker-only process: block until signalled
		slog.Info("worker process running", "roles", roles)
		<-ctx.Done()
		return nil
	}
	return serveHTTP(ctx, bs.cfg.HTTPAddr, handler, roles)
}

// serveHTTP runs the listener until the context is cancelled, then drains.
func serveHTTP(ctx context.Context, addr string, handler http.Handler, roles map[string]bool) error {
	srv := &http.Server{Addr: addr, Handler: handler, ReadHeaderTimeout: 10 * time.Second}
	slog.Info("listening", "version", Version, "roles", roleList(roles), "addr", addr)
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

// routeCacheTTL keeps road routes between the same ~11 m cells; roads change rarely.
const routeCacheTTL = 6 * time.Hour

// routingProvider picks the brand's routing provider. Google and Valhalla
// register here as they are implemented; until then the offline estimate serves.
func routingProvider(bs *bootstrap) routing.Provider {
	switch bs.brand.Providers["routing"] {
	case "osrm":
		base := os.Getenv("NEXA_OSRM_URL")
		if base == "" {
			base = "https://router.project-osrm.org" // public demo server: development only
		}
		slog.Info("routing via OSRM", "url", base)
		return routing.Cached{Inner: routing.NewOSRM(base), Fallback: routing.DefaultEstimate(), Redis: bs.redis, TTL: routeCacheTTL, Prefix: "route2:" + bs.brand.Code} // v2: with turn-by-turn steps
	default:
		slog.Warn("no production routing provider configured; using the offline estimate", "provider", bs.brand.Providers["routing"])
		return routing.DefaultEstimate()
	}
}

// smsSender picks the brand's SMS provider. Only the development logger exists
// yet; production providers register here as they are implemented.
func smsSender(bs *bootstrap) sms.Sender {
	switch bs.brand.Providers["sms"] {
	default:
		slog.Warn("no production SMS provider configured; using log sender", "provider", bs.brand.Providers["sms"])
		return sms.Log{}
	}
}

func parseRoles(s string) map[string]bool {
	out := map[string]bool{}
	for _, r := range strings.Split(s, ",") {
		if r = strings.TrimSpace(r); r != "" {
			out[r] = true
		}
	}
	return out
}

func roleList(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// mustRead loads a small seed file the server serves verbatim; a missing file is a startup error.
func mustRead(path string) []byte {
	raw, err := os.ReadFile(path)
	if err != nil {
		slog.Error("seed file missing", "path", path, "err", err)
		os.Exit(1)
	}
	return raw
}

// presenceService builds the Redis-backed driver presence, or nil when Redis is not configured.
func presenceService(bs *bootstrap) *presence.Service {
	if bs.redis == nil {
		slog.Warn("NEXA_REDIS_URL not set: driver presence and dispatch are disabled")
		return nil
	}
	return presence.New(bs.redis, bs.brand.Code)
}

// startRealtimeAndDispatch wires the websocket hub and the dispatcher (both need
// Redis) and connects them to trip changes: every committed change is pushed to
// the rider and driver, and dispatch frees drivers when trips end.
func startRealtimeAndDispatch(ctx context.Context, bs *bootstrap, roles map[string]bool, t *trips.Service, p *presence.Service, router routing.Provider) (*realtime.Hub, *dispatch.Service) {
	if bs.redis == nil || p == nil {
		return nil, nil
	}
	hub := realtime.NewHub(bs.redis, bs.brand.Code)
	d := dispatch.New(bs.pool, bs.redis, bs.brand.Code, t, p, hub, router)
	t.OnChanged(func(ctx context.Context, trip *trips.Trip) {
		d.TripChanged(ctx, trip)
		frame := realtime.NewFrame(realtime.TypeTripUpdated, trip)
		hub.Publish(ctx, trip.RiderID, frame)
		if trip.DriverID != nil {
			hub.Publish(ctx, *trip.DriverID, frame)
		}
	})
	if roles["realtime"] || roles["api"] {
		go hub.Run(ctx)
	}
	if roles["scheduler"] {
		go d.Run(ctx)
		slog.Info("dispatcher running", "tick", dispatch.TickInterval, "offer_ttl", dispatch.OfferTTL)
	}
	return hub, d
}

// placeCacheTTL keeps place answers a day; names and addresses change rarely.
const placeCacheTTL = 24 * time.Hour

// placesProvider picks place search. Nominatim (OpenStreetMap) until a Google
// Places key is configured; NEXA_NOMINATIM_URL points at a self-hosted server.
func placesProvider(bs *bootstrap) geocoding.Provider {
	base := os.Getenv("NEXA_NOMINATIM_URL")
	if base == "" {
		base = "https://nominatim.openstreetmap.org" // public server: development only
	}
	ua := "platform-dev/0.1 (" + bs.brand.Code + ")"
	inner := geocoding.NewNominatim(base, ua, bs.brand.Country, bs.brand.DefaultLocale)
	return geocoding.Cached{Inner: inner, Redis: bs.redis, TTL: placeCacheTTL, Prefix: "places:" + bs.brand.Code}
}

// postCompletedTrips records each cash-settled trip's fare split in the ledger
// (idempotent: a repeated notification posts nothing new). Trips paid through
// the gateway were posted inside their completing transaction by payments.
func postCompletedTrips(l *money.Ledger) func(context.Context, *trips.Trip) {
	return func(ctx context.Context, t *trips.Trip) {
		if t.Status != trips.Completed || t.DriverID == nil || len(t.Fees) == 0 || !paidInCash(t) {
			return
		}
		var b money.Breakdown
		if err := json.Unmarshal(t.Fees, &b); err != nil {
			slog.Warn("ledger: bad fare split", "trip", t.ID, "err", err)
			return
		}
		// Normally already posted in the completing transaction (a no-op here);
		// either way the driver's holds and app hear about the wallet change.
		if _, err := l.PostCashTripCompleted(ctx, t.ID, *t.DriverID, b); err != nil {
			slog.Warn("ledger: trip not posted", "trip", t.ID, "err", err)
		}
		l.WalletMoved(ctx, *t.DriverID)
	}
}

// paidInCash reports whether the driver confirmed cash for a completed trip.
func paidInCash(t *trips.Trip) bool {
	return t.PaymentStatus != nil && *t.PaymentStatus == trips.PayCashConfirmed
}

// postCashTripTx posts a cash-settled trip (the platform's share becomes the
// driver's debt, earlier dues are cleared) inside the transaction that completes
// it, so a crash between commit and a listener can never lose the debt.
func postCashTripTx(l *money.Ledger) trips.CompletedTxHook {
	return func(ctx context.Context, tx pgx.Tx, t *trips.Trip) error {
		if t.DriverID == nil || !paidInCash(t) {
			return nil
		}
		var b money.Breakdown
		if err := json.Unmarshal(t.Fees, &b); err != nil {
			return fmt.Errorf("trip %s has no fare split: %w", t.ID, err)
		}
		if _, err := l.PostCashTripCompletedTx(ctx, tx, t.ID, *t.DriverID, b); err != nil {
			return err
		}
		if dues := t.DuesIn(); dues.IsPositive() {
			_, err := l.PostDuesCollectedTx(ctx, tx, t.ID, t.RiderID, *t.DriverID, dues, t.Currency)
			return err
		}
		return nil
	}
}
