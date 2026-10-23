# Phase 0 progress — 2026-10-14

Repo initialised as a monorepo in this folder (git branch `main`, no remote yet). Internal codename `nexa`; display name/colours/icons are brand data.

## Done
- `backend/`: Go 1.27 module `github.com/kalen-limited/hauler-rider/backend`; binary `nexa` with `serve --roles`, `migrate`/`seed`/`verify-ledger`/`keys` stubs; kernel packages `config` (env, fail-fast), `brand` (loads `brands/<code>.json`), `theme` (presets + WCAG-checked derivation), `apperr` (error envelope), `httpx` (router / handlers / DTOs / respond / middleware / ready, one concern per file). Endpoints: `/v1/health`, `/v1/ready`, `/v1/app/config` (ETag), `/v1/theme/presets`, `/v1/services` (empty until catalog). Migrations `00001_brands.sql`, `00002_identity.sql` (goose, run via `go tool goose`). Makefile, Dockerfile (distroless), `.env.example`, `infra/compose.local.yml` (PostGIS + Redis). gofmt, vet, golangci-lint, tests and the brand-name guard pass.
- `backend/api/openapi.yaml`: contract skeleton (health, ready, app config, services, OTP request/verify, error envelope, Brand/Theme/Palette/Service schemas).
- `brands/songa.json`: first brand; `backend/seed/theme_presets.json`: 20 presets (19 Color Hunt + Songa classic).
- Apps workspace: pnpm 10 + Turborepo; `apps/packages/api-client` (generated from the spec with openapi-typescript + openapi-fetch + openapi-react-query), `packages/ui` (Theme/Palette types, bootstrap theme), stubs for realtime/forms/i18n/domain; `apps/rider` and `apps/driver` on Expo 54 + expo-router with brand-driven `app.config.ts`, ThemeProvider over `/app/config`, screens/components/hooks/services separated.
- CI: `.github/workflows/ci.yml` (Go format/vet/lint/brand-guard/test/build, OpenAPI lint, pnpm typecheck/lint with generated-client drift check, GHCR image on main).
- Engineering standards added to `CLAUDE.md` (comments, separation of concerns, small functions).

## Next (Phase 0 remainder → Phase 1)
- OTP auth + EdDSA JWT + JWKS (`internal/identity`), `nexa migrate` and `nexa seed` commands, River setup.
- Catalog module (services, vehicle classes, request schemas) with reference import from Songa.
- i18n catalogue (EN/SW) for error messages.
- Admin app moved into `frontend/admin` with the generated client.
- Push the repo to GitHub (`hauler-rider`), enable CI, EAS project setup per brand.

## Update (later on 2026-10-14) — identity + sign-in done, structure finalised
- Repo re-structured into self-contained `backend/`, `apps/`, `frontend/` (ADR 0007); contract at `backend/api/openapi.yaml`, also served at `GET /v1/openapi.yaml`; brand runtime data `backend/brands/`, app build identity `apps/brands/`. **Default brand is Nexa**; Songa kept as example brand.
- Backend: `nexa migrate` (goose + River) and `nexa seed`; identity module (OTP, EdDSA tokens + JWKS, refresh rotation with reuse detection, logout), River SMS job with dev log sender, EN/SW message catalogue, CORS middleware (`NEXA_CORS_ORIGINS`), test OTP: `NEXA_OTP_TEST_CODE=0000` for `NEXA_OTP_TEST_NUMBERS=255626512332,255715123702` (no SMS, no rate limit). Integration tests in `internal/identity/service_integration_test.go` run against `NEXA_TEST_DATABASE_URL` (CI has a Postgres service).
- Apps: Expo SDK 57; `@platform/ui` (ThemeProvider + primitives), `@platform/auth` (secure session, OTP service, refresh-on-401 client; localStorage fallback on web); phone + code screens, auth gate, sign-out. Verified on two phones over Wi-Fi and by the Playwright smoke test `apps/tools/e2e-web-signin.mjs` against the web build.
- Local dev: backend `make run` (brand nexa, OTP echo + test code on); apps `BRAND=nexa npx expo start --lan`; LAN address 192.168.100.20.

## Catalogue phase (2026-10-14, evening)
- Backend: migration `00003_catalog.sql` (services, vehicle_classes, cargo_types, config_versions), seeds `backend/seed/catalog/{cargo_hauling,taxi}.json` (hauling ACTIVE with 5 classes from Songa's 7 offerings + 4 cargo types; taxi HIDDEN), `internal/kernel/cache` (versioned in-process cache, 5 s version check, serves stale on failure), `internal/catalog` (store/model/Catalog read API, `nexa seed catalog`), `GET /v1/services` (ACTIVE only, ETag = catalogue version → 304) and `GET /v1/services/{key}`.
- Apps: rider home is now the service picker (tiles from the catalogue, localised by brand locale); `/service/[key]` shows vehicle classes and cargo types. Smoke test extended through the picker. Next: schema-driven request form + quotes.
- Phone sign-in debugging: phone taps reach the backend (202) but the device showed no navigation; added a dev status line under "Send code" and switched to Expo Router object navigation. Awaiting the status line text from the phone.

## Pricing phase (2026-10-14, late)
- `internal/pricing/engine`: pure, decimal-based. **Engine registry** (`engine.Register/Get/Describe`): `bracket_meter` (exact port of Songa V1: brackets → surge → floor → waiting/loaders/approach/booking fee; golden tests), `per_km_min` (taxi meter), `flat_rate` (fixed price per trip). Each engine publishes a settings JSON schema so the admin can render its form; `GET /v1/pricing/engines` lists them. Loaders are a flat fee per loader (Songa parity).
- Tariffs: per vehicle class with `engine` + `params` (migrations 00004/00005), brackets for the bracket engine, fee policy (`fee_policies`, versioned), versioned cache scope `pricing`.
- **Fixed routes** (00006/00007): origin/destination areas (point + radius), per service and class, bidirectional with **direction-specific prices** (`price`, `reverse_price` — Dar→Mbeya 3,000,000 vs Mbeya→Dar 2,000,000), includes_loaders, surge_applies, priority, validity. Precedence: fixed route > metered. Zones (area-based tariffs) are next in this layer.
- `POST /v1/quotes`: validates attributes against the service's request schema (jsonschema), filters classes by cargo allow-list and weight capacity, routes via `RoutingProvider` (offline estimate in dev; Google/Valhalla later), prices every class, persists the quote (3 min TTL). Seeds: `seed/pricing/{cargo_hauling,taxi,fixed_routes,fee_policy}.json` (placeholder numbers).
- Apps: next = schema-driven request form + quote screen in the rider app.

## General screens + design language (2026-10-14, evening)
- Backend: `profile` module (`GET/PATCH /v1/me`, `GET/POST/DELETE /v1/me/places` with HOME/WORK/OTHER), `GET /v1/config/cancel-reasons` (seeded EN/SW, rider + driver lists), migration 00009. Contract rewritten clean after an edit corrupted it (lesson: never `replace("")`).
- Catalogue: 10 services (taxi, delivery, food delivery, fast food, groceries, cargo transport, waste water, construction materials with icon-grid of water/sand/cement/gravel/stone/blocks/steel/timber + water bowser class, recovery, mechanic); icons are MCI glyph names stored in data; 30 tariffs across bracket_meter / per_km_min / flat_rate.
- Theme: brand preset `nexa-violet` (primary #5003C0, accent #7A3EF0, surface #F6F2FD) — still fully configurable from the admin.
- Rider app: tab shell Home · Activity · Account with floating **PillTabBar**; Account, Profile (inline edit), Settings (language toggle, log out), Saved places (home/work/other, current location), Payment (gradient balance card, Personal/Work toggle, cash live), Safety, Work profile, Activity (empty state), Cancel-reason modal; `@platform/ui` grew ListRow, Section, Header, RadioRow, Avatar, Banner, Icon, ScalePressable, PillTabBar; Reanimated with staggered entrances and spring press. Sign-out only in Settings. Design standards added to CLAUDE.md. Reference boards in `DOC/designs/` (general screens, concept, taxi story = taxi booking flow, next).
- Smoke tests: `apps/tools/e2e-web-signin.mjs` (sign-in → picker → service → account → payment → settings → logout), `e2e-web-grid.mjs`, `e2e-web-tabbar.mjs`.

## Booking core + taxi journey (2026-10-14, night)
- Backend `trips` module (migration 00010): explicit state machine per lifecycle profile (`machine.go`, tested for taxi/hauling/on-site), one-active-trip-per-rider enforced by a partial unique index, `trip_events` as outbox/replay log, `POST /v1/trips` (from a quote option; copies fare/route/stops), `GET /v1/trips[/active|/{id}|/{id}/events]`, `POST /v1/trips/{id}/cancel` (reason from config). Verified: quote → book → double-booking blocked → active → cancel → events → history.
- Rider app booking flow (`screens/booking/`): RouteScreen (pickup = current location, destination from saved/recent places or map pin), OptionsScreen (quote per class, Recommended/Faster/Cheaper, payment profile, Confirm order), RequestedScreen (progress, cancel via reasons sheet, route + payment cards). Booking draft in `services/bookingStore.ts`; `BookingMap` wraps react-native-maps (web placeholder). Smoke test `tools/e2e-web-taxi.mjs` passes end to end.
- Styling per Kalen: tiles faint `highlight` tint (#D9FDD3, new configurable palette role), 10px radius, centred, no shadow; fares displayed as whole shillings.
- Next: driver presence (Redis), WS hub, dispatch saga, driver app online/offer/trip screens → "driver on the way" state in the rider app.

## Presence + driver online (2026-10-14, afternoon)
- Backend `presence` module (Redis only, `internal/presence`): `presence:{brand}:{driver}` hash, `cell:{brand}:{h3}` sorted set scored by last ping, `cells:active` set; H3 res 9 via `uber/h3-go/v4`; staleness 120 s is the single definition of "online"; `Nearby` = k-ring + ZRANGEBYSCORE + capability filter, nearest first; `GC` sweeps expired members. Redis-backed test `presence_test.go` (skips when Redis is down).
- Endpoints: `GET/PUT /v1/driver/status`, `POST /v1/driver/location` (driver-app token only; `requireApp`, 503 `PRESENCE_UNAVAILABLE` when Redis is unset). Verified with curl: online → ping → bad point 400 → rider token 403 (sw message) → offline → ping 409 `DRIVER_NOT_ONLINE`.
- Driver app: `services/presence.ts`, `hooks/usePresence.ts` (status query, set-status mutation, `useLocationStream` = expo-location watch every 10 s / 25 m → POST location), `OnlinePulse` breathing ring, HomeScreen online toggle with status card. Capabilities are hard-coded to taxi/standard until vehicles exist.
- Icons: Kalen wants realistic 3D product-render icons with violet accents like `DOC/designs/taxi story/1.jpeg`. `apps/tools/gen-icons.py` (OpenAI Images API, transparent PNG, one prompt style, brand accent as a parameter) + `apps/tools/sync-icons.sh` (generates `rider/src/assets/serviceIcons.ts`); `ServiceIcon` component shows the PNG when present, else the glyph in brand primary (the violet square container is gone). **The OpenAI account had no credits**, so no icons are generated yet; key lives in `apps/tools/.env` (gitignored).
- Dev networking: both apps now derive the API host from the address Expo Go loaded them from (`src/config.ts`), so one dev server serves a USB phone (127.0.0.1 via adb reverse) and a Wi‑Fi phone (exp://192.168.100.20:808x) at the same time.
- `pnpm-workspace.yaml` pins `react`/`react-dom` 19.2.3 via `overrides`: a peer auto-install had pulled react-dom 19.3.0 under expo-router and blanked the web builds. Driver app now declares react-dom + react-native-web.
- Smoke test `apps/tools/e2e-web-driver.mjs` passes: sign in → go online (cell shown) → ping → go offline.

## Taxi rider journey cloned from the taxi story + video (2026-10-14, afternoon)
- Source: `DOC/designs/taxi story/1–6.jpeg` and the screen recording (frames extracted with an AVFoundation script; contact sheets in the session scratchpad). Brand colour stays the theme primary; layout, motion and copy follow the recording.
- Rider screens (all map-first):
  `/book` TaxiHomeScreen (map + nearby cars, "Great to see you." / "Go wherever, whenever.", 2×2 ride tiles incl. Schedule, Where to? + Later, recents) →
  `/book/route` RouteScreen (bordered pickup/drop-off card, clear, map button, + / swap, place list with km, skeleton while loading) →
  `/book/pick` PickOnMapScreen (fixed pin, name the spot) →
  `/book/options` OptionsScreen (route line, "Pickup · N min" and "Dropoff · time" pins, floating route bar, saved-pickup / "Ride cancelled" snackbar, sheet snapping min/half/full with filter chips and map scrim, Personal chip ↔ personal/work switch, Confirm order + schedule; skeleton rows while pricing; soft loading and the sheet slides away on confirm) →
  `/book/requested` RequestedScreen ("Looking for a driver" → fly-in zoom to pickup with pulse, "Ride requested", searching bar, cancel, route, payment, More: share / cancel) →
  "Are you sure?" sheet → "What went wrong?" (restyled, Done pinned) → back to options with "Ride cancelled" (draft kept, re-priced).
- Splash: brand-colour screen with the brand name, circular reveal (ring whose border shrinks; no mask library).
- Backend: `GET /v1/drivers/nearby` (Redis presence only; positions + classes, never identity) feeds car pins and per-class pickup ETAs (`models/eta.ts`, 18 km/h × 1.3 road factor until dispatch ETAs). Taxi classes got descriptions in the seed.
- Driver app: map-first home (radar rings, car marker, online switch, sheet with offline/online panels, stats strip with live online timer) + tabs Home/Earnings/History/Account, Settings holds log out.
- Smoke tests: `apps/tools/e2e-web-taxi.mjs` (whole journey incl. cancel → rebook) and `e2e-web-driver.mjs` pass.
- Not yet built (need dispatch/realtime first): driver-assigned card (design 6), chat, edit stops, Safety, payment screen parity, scheduling.

## Maps engine change (2026-10-14, late afternoon)
- The black maps were **Google Maps "Authorization failure"**: Expo Go uses its own built-in Google key (`host.exp.exponent`), which Google rejects; our key cannot be injected into Expo Go. (An earlier note blamed ScrollViews; that was wrong.) UrlTile overlays do not help: the SDK draws nothing after the auth failure.
- New package `apps/packages/maps` (`@platform/maps`): `MapCanvas` renders MapLibre GL JS 4.7.1 with OpenFreeMap Positron vector tiles (free, no key) inside `react-native-webview` 13.16.1 on phones and an iframe on web. Screens pass plain data (markers: pickup/dropoff/car/self, route, fit points, zoom/fly-in, padding, theme colours); camera moves are eased (fitBounds/flyTo/easeTo), the OSM credit stays above sheets. Rider `TaxiMap` and driver `DriverMap` now use it; Google-only pieces removed.
- Verified on the rider Pixel: taxi home shows streets and the rider's position.
- Production path: a development build (EAS / `expo run:android`) with native MapLibre (`@maplibre/maplibre-react-native`) or Google with our own restricted key; `MapCanvas` props stay the same so screens do not change.
- Motion: all springs removed (Kalen: "remove the bouncing animation"); eased timings only.

## Dispatch + realtime + driver/rider trip flow (2026-10-14, evening)
- Migration 00011: `trip_offers` (wave, distance, ETA, outcome OFFERED/ACCEPTED/DECLINED/TIMED_OUT/CANCELLED, source AUTO/MANUAL) with partial unique indexes: one open offer per trip and per driver.
- `internal/realtime`: `/v1/ws` hub (coder/websocket). First-frame auth (token never in the URL), `ready`, ping/pong + server pings, Redis pub/sub fan-out `rt:{brand}:u:{user}` so any process can publish. Frames: `trip.updated`, `offer.new`, `offer.closed`, `driver.location`.
- `internal/dispatch` (v0, doc in `doc.go`): 1 s tick under a per-brand Redis leader lock. Expire offers (TIMED_OUT → trip back to REQUESTED), end trips past the window (NO_DRIVER_FOUND), then batched assignment: H3 ring search 2→4→8→16, idle + eligible + not-yet-asked drivers within 6 km, greedy min-ETA one-to-one across all waiting trips (`matcher.go`, unit-tested), sequential offer with 20 s TTL. Accept is atomic (offer row lock + trip `WHERE status='OFFERED'`); decline/timeout never re-asks the same driver.
- `trips`: rider/driver parties on every trip, `driver_location` from presence on rider reads, dispatch ops (MarkOffered/Release/Assign/ExpireUnmatched), driver transitions incl. `CANCELLED_BY_DRIVER` → re-dispatch with a fresh window, `OnChanged` → realtime fan-out; GPS pings relay `driver.location` to the rider from Redis only.
- Endpoints: `GET /driver/offer`, `POST /driver/offers/{tripId}/accept|decline`, `GET /driver/trips/active`, `POST /driver/trips/{id}/transitions`, `GET /ws`.
- Apps: `@platform/realtime` (reconnect with backoff, heartbeat, silence detection), rider `useTripRealtime` (cache updates; polling now a 10 s fallback), `TripScreen` for every phase (design 6 driver card, Safety pill, arrived / riding / pay / done / no-driver), driver `OfferCard` (countdown, Decline/Accept), `TripPanel` (navigate, call, step button from `allowed_transitions`), map framing car ↔ stop, 4 s GPS while on a trip.
- Fixed on the way: session `storage.remove()` recursed forever on phones (sign-out); websocket token refresh now shares the client's single-flight refresh (`freshAccessToken`) — two parallel refreshes reuse a rotated token and the server revokes the session.
- Tests: `apps/tools/e2e-dispatch.mjs` (API + sockets: offer, decline/no re-offer, accept, double-accept rejected, live location, full lifecycle, driver back to IDLE) and `apps/tools/e2e-web-ride.mjs` (both web apps, a full ride) pass.
- Next: chat with driver, rating after the trip, vehicles (plate/colour/model) for the driver card, routed ETAs, driver earnings from completed trips.

## Road routing, resume, capacity (2026-10-14, night)
- Road routing via OSRM (`internal/providers/routing/osrm.go`), Redis-cached per ~11 m cells for 6 h with fallback to the estimate (`cached.go`); brand `routing: "osrm"`, `NEXA_OSRM_URL` (public demo server is development only). Quotes, offers (road pickup ETA for the chosen driver) and `GET /v1/routes` use it. Apps draw road polylines and road ETAs (rider trip + options, driver trip panel).
- Ride in progress takes over: rider `useActiveTripRedirect`, driver `useWorkRedirect`; booking 409 TRIP_ALREADY_ACTIVE opens the existing ride. Test `apps/tools/e2e-web-resume.mjs`.
- Driver heartbeat is time-based (was movement-only: parked drivers vanished); status re-read every 30 s. "Use a different number" works after reloads.
- Capacity: see `DOC/05-capacity-plan.md`. Fixed a crash (unlocked pricing schema map), presence moved to per-service Redis GEO indexes + `seen` zset GC every 30 s, 3 s per-area nearby cache. Load tool `backend/tools/load`.

## Place search + stops (2026-10-14, late night)
- `internal/providers/geocoding`: Provider port (Search, Reverse), Nominatim implementation (1 req/s spacing, country + language from the brand, ~40 km bias box) behind a Redis cache (24 h; search per query and ~1 km area, reverse per ~11 m). `GET /v1/places/search?q=&lat=&lng=` (3+ chars), `GET /v1/places/reverse`. Public Nominatim is development-only: production = Google Places (key) or self-hosted Nominatim/Photon (`NEXA_NOMINATIM_URL`).
- Stops: quotes take `waypoints` (≤3, migration 00012 `quotes.waypoints`), routing through them (`routing.Via`, OSRM multi-point), no fixed-route match when stops exist; trips store PICKUP → WAYPOINT… → DROPOFF; `/routes?via=`; offers carry `stops_count`.
- Rider: debounced search with saved/recent first then results, "Use current location", GPS pickup named by reverse lookup, "+" adds up to 3 stops (removable), focus moves to the next empty field, pick-on-map names the spot and supports stops; stop pins and lists on options/trip screens. Driver: stop list in the trip panel, navigation through stops, stop pins.
- Test `apps/tools/e2e-web-route.mjs`; verified on the rider Pixel (pickup "Makongo", "sinza" suggestions, "+" adds Stop 1).

## Fares visible, history, services, money phase start (2026-10-14, late night)
- Rider: fare + payment method on every trip card, "Pay TZS X" step, receipt (fare lines, booking fee, total, includes VAT / LATRA levy) on finish and in trip details. Activity (design "history": latest-trip card with map + Rebook, rows) and `/trip/[id]` details. Driver: fare block with "You earn", big "Collect TZS X in cash · You keep … · platform fee …", "Cash collected · TZS X"; History tab and Earnings (today/week, fares, platform fee).
- `GET /v1/driver/trips`; trip lists load stops in one extra query (no N+1).
- Services: Food = restaurants + fast food (fast_food HIDDEN); Delivery = taxi flow with Boda, Guta, Kirikuu, Town Ace (≤ 400 kg; placeholder tariffs; request details optional for now); Cargo transport = trucks/closed van (> 400 kg) with required `trip_scope` IN_REGION / OUT_OF_REGION / OUT_OF_COUNTRY; Breakdown sorted second. Seeder now writes class status. Driver picks service + class in Account → Vehicle (remembered on device) until the vehicles module exists.
- Money (`internal/money`): `Split` (commission, booking fee, VAT on platform revenue extracted, LATRA levy and who bears it, driver earnings; exact-sum invariant; tests) using `fee_policies` (version pinned per trip); `trips.fees` stored at booking and recomputed at completion. Ledger (migration 00014): accounts, idempotent transactions (kind+reference), append-only hash-chained entries (DB trigger), `PostTripCompleted` for cash trips (driver owes platform share → DRIVER_WALLET debit; PLATFORM_REVENUE, VAT_PAYABLE, LEVY_PAYABLE credits), `GET /v1/driver/wallet`, `nexa verify-ledger`. Driver Earnings shows "You owe the platform".
- Current policy: commission 10 %, VAT 18 % on platform revenue, LATRA levy 0 %, booking fee 0 — owner to confirm values (levy rate from LATRA, booking fee).
- Next in the money phase: debt limits / holds, settlement by mobile money (Selcom), payouts, digital payments, admin finance views.

## Turn-by-turn, trips for someone else, recovery sizing (2026-10-14, night)
- Routes carry turn-by-turn `steps` (OSRM `steps=true`: type, modifier, street, distance, roundabout exit, location; intermediate arrivals become `waypoint`). Route cache key moved to `route2:`; quotes store the route without steps.
- Driver app: `NavBanner` over the map while driving to the pickup or the destination (arrow, distance, "Turn left onto …", "Then …"), advancing when within 25 m of a manoeuvre; fresh route every ~110 m of movement (`models/navigation.ts`, `hooks/useNavigation.ts`). The trip card shows and calls the person at the stop (passenger / sender at pickup, receiver at drop-off, "Booked by …") instead of always the account holder.
- Contacts (migration 00015 `trips.contacts`): lifecycle rules `pickup_contact: when_not_here` (taxi, delivery, cargo, recovery) and `dropoff_contact: required` (delivery, cargo; server rejects with `CONTACT_REQUIRED`). Phone ≥ 9 digits and a name.
- Rider: new Details step (`/book/details`) between route and options, only when needed — pickup away from the phone's position (> 150 m, not the GPS pick) asks "Who is riding? / Who hands it over? / Who is with the vehicle?" Me | Someone else + name and phone; Delivery and Cargo always ask "Who receives it?"; the service's request fields render from its schema (icon-grid, segmented, stepper, toggle, number, text). Cargo and Recovery now book through the same map flow.
- Recovery: required "What needs recovery?" (motorcycle, saloon, small car, SUV, pickup, minibus, truck, lorry/bus); each size lists the recovery classes allowed to carry it and the quote only offers those (`filterByOptionRules`, e.g. lorry → heavy recovery; saloon → tow truck or flatbed).
- Tests: `apps/tools/e2e-web-details.mjs` (taxi for someone else, delivery receiver required, lorry → heavy recovery) plus route, ride and dispatch (taxi and delivery) all pass.
