# 00 · Songa system scan (the reference system)

> Read-only scan of the Songa workspace made on 2026-10-14 as the reference for the Platform redesign. Songa is the system being replaced; nothing here is a target design. See `01-platform-redesign.md` for the target and `02-dispatch-engine.md` for booking and matching. Section 8 is the list of flaws the redesign removes (expanded in the redesign doc, D12).

## Context

Kalen asked for a read-only scan of the whole Songa workspace (backend, APIs, admin dashboard, mobile apps, website, docs) and a write-up of how it fits together and how the APIs work. No code change is requested; the deliverable is this understanding, so the team can decide what to do next from a shared picture.

## Workspace at a glance (verified 2026-10-14)

| Dir | What | Git | Last commit |
|---|---|---|---|
| `songa_backend_revamp_v2/` | Django REST + Channels backend (code in `songa_backend_revamp/`) | main | 2026-10-12 docs: Selcom + LATRA vendor PDFs |
| `songa_admin/` | Next.js admin dashboard | main (uncommitted drivers/riders split) | 2026-10-13 sign-in brand panel |
| `songa-rider-app/` | Flutter rider app (separate developer, do not edit) | main | 2026-10-12 booking fee on payment card |
| `songa_driver_app/` | Flutter driver app (separate developer, do not edit) | main | 2026-10-01 trip receipts |
| `website/` | React 19 + Vite marketing site, local git only | main | 2026-10-12 initial |
| `LATRA/`, `selcom/`, `KEYS/`, `build/`, `Songa-resources/` | vendor PDFs, keys, brand assets | — | — |
| top-level `*.md`/`.txt` | Aug-2026 API reference + Swahili catalog, nearby-drivers contract, droplet access, FCM bug note | — | — |


## 1. Backend architecture

Three runtime services plus workers, all behind nginx (prod: behind traefik):

```
nginx :80 ──/api/*, /──▶ Django (uvicorn ASGI :8000, DRF + SimpleJWT)
          ──/ws/*────▶ Java WS gateway (Spring WebFlux / Reactor Netty :8080)

Django ──Kafka outbound_ws_events──▶ gateway ──▶ client sockets
gateway ──Kafka inbound_ws_events──▶ Django `run_kafka_consumer` (location, trip events, chat)
Celery worker + beat (Redis broker) ── all slow work: dispatch, trip transitions, billing, LATRA, payouts
Redis ── live driver presence (h3:{cell}:drivers, driver:{id}:state), trip GPS stream trip:{id}:path, caches, OTP limits
PostGIS (managed DO Postgres via pgbouncer, primary/replica router) ── durable data
```

- **Websockets are NOT Django.** `channels` is in requirements but not installed; `asgi.py` rejects any WS with 4404. The old `trips/consumers/*.py` are dead code. The real socket server is `songa_ws_gateway/` (Java 21): paths `/ws/trips/**` and `/ws/driver/**`, auth `?token=<SimpleJWT>` validated HS256 with Django's `SECRET_KEY`. Each connection joins `user_{id}` + `driver_{driver_id}`, plus `drivers_location_updates`, `trip_admin` or `trip_{uuid}` by path. The gateway writes driver GPS straight into Redis (H3 set moves, state merge, path stream) before forwarding to Kafka.
- **Django apps**: `users` (User, Driver, OTP, fraud profile), `vehicles` (CarType, CargoType→allowed_car_types, Make, Model, Vehicle), `config` (Region, District, MatchingConfig, SurgeConfig, FraudConfig, RatingAspectConfig, VehicleImageConfig, PricingModelConfig), `trips` (Trip, TripAction, TripRating, TripLocationHistory partitioned), `pricing` (+`pricing.v3`: brackets, zones, zone profiles, fixed routes, V3 cargo catalogue), `payments` (Account, hash-chained LedgerEntry, Transaction, DriverWallet, PaymentRecord, Invoice, Settlement, Disbursement, GatewayTransaction, PaymentMethod, ServiceChargeSetting, DisbursementSetting, PaymentGatewayConfig), `messaging` (ChatMessage, DeviceToken, QuickReplyTemplate), `admin_api` (AdminAuditLog + all staff endpoints, Django-Group RBAC), `latra` (CeTS client, JWKS, LatraDailyReport), `eventlog` (SystemEvent partitioned activity log), `web` (landing).
- **Auth model**: SimpleJWT only (access 60 min, refresh 30 d, rotation + blacklist). Mobile = phone OTP, no password (`generate-otp` → NextSMS → `verify-otp` → JWT pair; 60 s resend limit, 5 fails = 10 min lockout, in Redis). Staff = email+password at `/token/`, JWT carries `roles` (Group names). **No role column on User**: driver = has a `Driver` row, rider = no Driver row, staff = Groups. `DEFAULT_PERMISSION_CLASSES` is commented out, so any view without explicit `permission_classes` is public (quote, nearby-drivers, OTP views are intentionally open).

## 2. API surface (by prefix)

No API versioning. `/api/v3/` is one endpoint (V3 pricing catalogue); "V1/V2/V3" everywhere else means pricing engines, not API versions. Views are mostly APIViews; DRF routers only in payments and admin_api.

| Prefix | Audience | Endpoints |
|---|---|---|
| `/token/`, `/token/refresh/` | staff | email+password login, refresh |
| `/api/users/` | both apps | `register/customer/`, `register/driver/` (multipart KYC + vehicle), `generate-otp/`, `verify-otp/`, `logout/`, `profile/` GET/PATCH, `driver/status/` (go online/offline → writes Redis + H3), `driver/location/` (HTTP ping, 5 s rate limit), `driver/ledger/`, `driver/stats/`, `drivers/` |
| `/api/trip/` | rider (public) | `nearby-drivers/`, `Pre-RequestCarrier/` (fares + availability), `EstimateTripPriceAllVehicleType/` (fares + polyline) |
| `/api/trip/` | rider (auth) | `RequestCarrier/` POST, `RequestCarrier/status/`, `destination-history/` |
| `/api/trip/` | driver | `RequestedTrips/`, `AcceptTrip/`, `RejectTrip/`, `DriverProximityAlert/`, `TripStartLoading/`, `TripLoadingFinished/`, `TripUnloadingStarted/`, `TripUnloadingFinished/` |
| `/api/trip/` | shared | `active/`, `CancelTrip/`, `history/`, `history/<uuid>/`, `trip-actions/`, `ratings/`, `rating-aspects/`, `receipt/<uuid>/`, `receipt/<uuid>/pdf/` |
| `/api/vehicle/`, `/api/config/` | public GET | car types, cargo types, makes, models, image-types; regions, districts |
| `/api/payments/` | apps | `payment-methods/` CRUD, `disbursements/` (driver payout), `settlements/` (driver debt pay-in via USSD push), `cards/preference/`, `cards/register/`, `webhooks/<gateway>/` (Selcom, unauthenticated, re-queries status) |
| `/api/messaging/` | apps | `messages/` GET/POST, `messages/read/`, `messages/unread-count/`, `quick-replies/?lang=`, `devices/` POST/DELETE (FCM token) |
| `/api/v3/pricing/catalogue/` | rider | which V3 inputs to collect |
| `/api/latra/jwks/` + `/.well-known/tz-e-ticketing-server` | LATRA | public key discovery |
| `/api/admin/` | dashboard | see §6 (drivers, riders, trips, finance, config, pricing, vehicles, staff, live, analytics, events, audit, logs, fraud, kyc, system/health) |
| WS `/ws/driver/location/`, `/ws/trips/<uuid>/`, `/ws/trips/admin/` | apps + dashboard | Java gateway; `?token=` JWT |

The Aug-28 `API_REFERENCE_AND_SWAHILI_TRANSLATION.md` is still accurate for the app-facing routes; additions since: `receipt/`, `/api/v3/pricing/catalogue/`, V3 fields on the quote/request bodies, `cargo_type_*` in driver `trip_request` WS payloads.

## 3. Trip lifecycle end to end

1. **Quote**: rider app calls `EstimateTripPriceAllVehicleType/` or `Pre-RequestCarrier/`. Google Directions gives road distance/time (cached on ~11 m-rounded coordinates), surge multiplier computed from H3 supply/demand and stored with the quote.
2. **Request**: `RequestCarrier/` → 409 if rider already has an active trip; reverse-geocode; cargo↔car-type compatibility; reuse cached quote + surge; lock zone / fixed route / pricing engine onto the Trip; pre-charge card if CARD; H3 res-9 index pickup+dropoff; save `REQUESTED`; after commit `dispatch_trip.delay()` and notify `user_{rider}` + `trip_admin` via Kafka.
3. **Matching** (`utils/songa_s1/suggest_driver.py`): expand H3 rings k=initial,2,5,10,20,50,100,160 over Redis `h3:{cell}:drivers`; filter each `driver:{id}:state` on online, verified, car type, cargo, telemetry freshness (≤300 s), busy, direction; cap by `MatchingConfig.max_radius_km` (30 km default, nearby-map radius 5 km); offer pushed to each `driver_{id}` group over WS. None → `NO_DRIVER_FOUND`; beat `reap_stale_trip_requests` (60 s) closes unaccepted requests.
4. **Drivers are notified only over WS** (Kafka → gateway). FCM is chat-only.
5. **States**: `REQUESTED → STARTED (AcceptTrip) → LOADING → EN_ROUTE_TO_DESTINATION → UNLOADING → COMPLETED`, plus `CANCELED_BY_DRIVER`, `CANCELED_BY_RIDER`, `NO_DRIVER_FOUND`. `ACCEPTED` and `EN_ROUTE_TO_PICKUP` exist in choices but no code path sets them. Driver cancel before en-route puts the trip back to `REQUESTED`, adds driver to `rejected_by_drivers`, re-dispatches ("rerouting").
6. **Every transition is an HTTP POST** that validates and hands off to a `process_*` Celery task in `trips/tasks/trip_operations.py`, which mutates status and broadcasts to `trip_{id}`, `user_{rider}`, `driver_{id}`. Inbound WS trip events are only re-broadcast as `trip_status_update`; they never change state.
7. **Driver location**: WS `location_update` → gateway writes Redis → Kafka → Django `broadcast_driver_location` pushes to `trip_{id}` + rider. HTTP `driver/location/` is the fallback. Beat syncs Redis→DB (5 min) and DB→Redis (3 min). On completion `archive_trip_path` copies the stream into `TripLocationHistory`.
8. **Completion** (`TripUnloadingFinished/` → `process_end_trip`): bill with the trip's locked engine (GPS distance if `GPS_DISTANCE` model, pickup-approach fee, booking fee, min-fare floor) → `process_trip_payment` → invoice/receipt → LATRA `submitTrip` queued → route archived → `payment_finalized` + `rating_required` WS events.

## 4. Pricing, payments, tax

**Pricing**
- One active `PricingModelConfig` row picks the engine (V1/V2/V3) and distance model (`ROUTE_ESTIMATE` vs `GPS_DISTANCE`) for NEW trips; `Trip.pricing_engine` + `quote_snapshot`/`billing_snapshot` freeze it per trip. Registry `pricing/engines/registry.py`, interface `engines/base.py` (`quote()`/`bill()`/`settings_schema()`).
- **V1** = `pricing/utils/pricing_algo.py` (distance/duration `PricingBracket`s per car type and zone, minimum fare, waiting after loading allowance, couriers, surge). **V2** = same formula, configurable switches, frozen tariff copy. **V3** = cargo-based (`CargoPricingProfile`, `VehicleTier`, `CategoryTariff`, `RecoveryVehicleClass`, `LiquidRate`; inputs `cargo_sub_type`, `weight_kg`, `volume_litres`, `recovery_class`; cancellation compensation recorded only). UAT/prod: **V1 active, ROUTE_ESTIMATE**, V2/V3 dark.
- **Location pricing** (`pricing/zones.py`): precedence manual price > fixed route > AREA zone > REGION zone > global; pickup decides the zone; H3 cells or PostGIS polygons; Redis catalogue + `pricing:key_version` generation counter. Admin `set-price/` → `pricing_source=MANUAL`. No zones seeded yet on UAT.
- **Fees**: pickup-approach (`CarType.pickup_approach_rate` per km, zone-overridable), booking fee (`ServiceChargeSetting.booking_fee`, 100% Songa, currently 0), surge (`SurgeConfig`), actual-distance billing capped at road route ×1.25.

**Payments** (`payments/`)
- `process_trip_payment` → `fare_breakdown.split_fare()` is the single place money is divided; invariant `rider_total = driver_earnings + songa_net + vat + levy`. Commission 10%, VAT 16% extracted from commission (`vat_base=COMMISSION`; `GROSS_FARE` option refused if it makes Songa negative), LATRA levy rate 0 (borne by SONGA by default). Double-entry `Transaction` + hash-chained `LedgerEntry`; liability accounts "VAT Payable - TRA" and "LATRA Levy Payable".
- **Cash trips** accrue the commission as driver **debt** on `DriverWallet`; **mobile-money/card** deduct from the driver's share. Debt holds: ≥5,000 immediate, ≥1,000 unpaid 7 days; <1,000 never holds (Selcom minimum). Hold forces driver offline; hourly `enforce_account_holds`.
- **Selcom**: two clients. Business API (RSA-SHA256) for payouts/balance/bank sweep; Checkout API (HMAC) for collections: driver USSD debt pay-in (`settlements/ pay_now`) and rider card charge-first (gated by admin switch + `User.card_payments_activated` + stored card; any failure → CASH). Webhook never trusts the payload; `reconcile_gateway_transactions` beat (2 min) is the authority. Payout triggers driver/admin/auto each switchable in `DisbursementSetting`. Rider mobile-money fares still complete with no real money movement (known, out of scope so far).
- **Receipts**: one `Invoice` per trip is the receipt (schema_version 2, money frozen by DB trigger), role-aware JSON + reportlab A5 PDF EN+SW, `lines[]` codes BASE_FARE/SURGE/WAITING/COURIER/PICKUP_APPROACH/MINIMUM_FARE_ADJUSTMENT/BOOKING_FEE/FIXED_ROUTE/MANUAL_PRICE.

**LATRA/CeTS** (`latra/`): RS256 JWT, JWKS at `songa.co.tz/.well-known/tz-e-ticketing-server`; submitVehicle/submitDriver on KYC approval, submitTrip on completion (stores `latraReferenceNumber`), daily report 21:30 UTC, retry ladder 1m/5m/30m/3h/daily via beat sweeper. Live on UAT. Undocumented CeTS codes 5016/5018/5019, zero-trip daily reports rejected, licence must be 10 digits.

## 5. Mobile apps — how they consume the API

Both are Flutter 3.7 / Riverpod apps by a separate developer (do not edit). Both hard-code the host `songa.kalen.co.tz` (UAT = prod droplet) in a constants file; no flavors, no `--dart-define`, no CI, release builds signed with debug keys. Firebase project `test-songa` in both; iOS push entitlement commented out in both. Google Directions key hard-coded client-side in both.

**Rider app** (`songa-rider-app/`, id `songa.kalen.co.tz.songa_rider_app`)
- HTTP: raw `package:http`, no shared client; each service adds `Authorization: Bearer` itself and retries once after refresh in ad-hoc places. Proactive refresh every 25 min / on resume (`session_keep_alive.dart`). Multipart bodies for OTP/profile/quote/request, form-encoded for cancel/nearby.
- Flow: `register/customer/` → `generate-otp/` (`login_type=rider`) → `verify-otp/` → `profile/` (rejects if `driver_id` present) → home polls `nearby-drivers/` every 12 s (no auth) + `cargo-types/` + `/v3/pricing/catalogue/` (any failure = V3 off) + `destination-history/` + `active/`.
- Quote = `Pre-RequestCarrier/` (no auth) → `car_types[]` with `approximate_fare`, `available_count`, ETA, SW translations. Book = `RequestCarrier/` (Bearer) with `pickup_*`/`dest_*`, `car_type`, `cargo_type`, `payment_method` CASH|CARD, V3 fields, Google addresses, and `pickup_geolocation_google='POINT (lat, lon)'` (lat-first, non-WKT).
- Waiting: polls `RequestCarrier/status/` every 4 s and reconciles with `active/`, plus WS. Only one socket: `/ws/trips/<trip_id>/?token=`, joined after booking or on resume; receive-only except chat. Backoff 1→8 s unlimited, 12 s silence watchdog, no client ping.
- Handles WS types: driver assigned (5 aliases), `driver_location_update`, `trip_status_update`, `driver_proximity_alert`, loading/unloading started/finished (+ `remaining_seconds` ticks), `trip_en_route_to_destination`, `trip_canceled`/`trip_cancelled`, rejected aliases, `no_driver_found`, `payment_finalized`/`payment_details`, chat frames. Treats `loading_duration` as minutes.
- Payment: cash default; Selcom card via `cards/preference/` + `cards/register/` → WebView, polls preference every 8 s until `has_card`; reads `card_fallback_reason` on the trip. Receipt from `payment_finalized.receipt` then `receipt/<id>/` + `/pdf/` in history. Rating `ratings/` with `score`+`comment` only (aspect chips folded into comment text).
- FCM: registers `/messaging/devices/` (`app=RIDER`) from home + `onTokenRefresh`; only `chat_message` taps handled; no foreground handler. Known race: skips registration if the access token is not yet loaded (`RIDER_FCM_REGISTRATION_BUG.md`).
- Repo state: **merge in progress** (`MERGE_HEAD` present, `home_user.dart` unmerged; staged `pricing_v3.dart`, `cargo_measure_field.dart`). Dead screens: `nearby_drivers_map_page.dart`, `searching_for_driver_page.dart`.

**Driver app** (`songa_driver_app/`, pubspec `songo_driver_app`, Android id `com.example.songo_driver_app`, iOS `kalen.co.tz.songoDriverApp`)
- HTTP: shared `ApiClient` (`data/token/app_client.dart`) = JSON, 15 s timeout, proactive refresh at <5 min, single shared refresh on 401/403 then retry, logout on refresh failure. Side effect: any business 403 triggers a refresh+retry. Trip-stage calls (`DriverProximityAlert`, `TripStartLoading`, `TripLoadingFinished`, receipts, photo upload) still use raw `http`, some without refresh.
- Flow: `generate-otp/` (`login_type=driver`) → `verify-otp/` → `profile/` (rejects if no `driver_id`; non-Active status → AccountPending) → home toggle `POST driver/status/` → `/drive_online` page posts status again, opens WS `/ws/trips/` (offers) + `/ws/driver/location/` (outbound GPS), starts foreground background service.
- **Two body shapes for `driver/status/`**: `{is_online:"true", lat, lng}` (home toggle) vs `{is_online:true, latitude, longitude}` (online page). Backend doc says `lat`/`lng` are required when going online, so the second shape should be checked.
- GPS: `location_update {driver_id, trip_id?, trip_status, latitude, longitude, timestamp(UTC)}` on 1 m movement + 25 s heartbeat when idle (honours the nearby-drivers contract); background isolate opens its own sockets, pings every 15/30 s; local notification for `trip_request` when backgrounded. WS reconnect: `{type:'ping'}` every 5 s, 2 s·2ⁿ backoff ≤10 attempts, 100-message outbound queue keeping only newest location.
- Offer: `trip_request` on `/ws/trips/` (pickup/destination as `[lng,lat]`, many fare/carrier key fallbacks, `loading_duration`), 30 s timer → `RejectTrip/`; `AcceptTrip/` `{trip_id, driver_id}` → join `/ws/trips/<id>/` → tracking page (phase `EN_ROUTE_TO_PICKUP` kept locally). Minor bug: pickup polyline drawn with swapped lat/lng.
- Trip stages (REST, driver_id in body): within 30 m → `DriverProximityAlert/` + WS `driver_arrived_at_pickup`; `TripStartLoading/` → countdown → `TripLoadingFinished/ {start_time, end_time, time_taken_seconds}` (**sent as EAT wall-clock, no offset → 3 h skew in DB**, known); en-route GPS `trip_status:'en_route_destination'`; within 30 m WS `driver_arrived_at_destination`; `TripUnloadingStarted/` → `TripUnloadingFinished/` → wait WS `payment_finalized {fare_data, receipt, wallet{balance,debt}}` → driver confirms cash → WS `trip_completed` → `RateRiderPage` → back online. Resume via `active/` + persisted `TripPhase` (local-only statuses `ARRIVED_AT_PICKUP`, `AWAITING_PAYMENT`).
- Money screens: wallet from `driver/stats/` (`balance, debt, net_balance, is_on_hold, hold_reason, debt_threshold, is_near_limit`), ledger `driver/ledger/?period=`, debt pay-in `settlements/ {amount, pay_now:true}` (120 s USSD wait), manual declare with reference, settlement history. Earnings computed client-side from `trip/history/` (up to 40 pages) because cash trips produce no wallet credit. **No payout/disbursement or payment-method UI** although backend `/api/payments/disbursements/` + `payment-methods/` exist and the app's own `BACKEND_PAYOUTS_INTEGRATION.txt` specifies them.
- Registration `register/driver/` multipart: sends both `car_type` and `car_type_id`, both `vehicle_model` and `vehicle_model_id`, `cargo_types` as the string `"[1, 2]"`, 7 image fields. Lookups: `config/district/concatenated/`, `vehicle/VehicleModel/concatenated/`, `vehicle/vehicleType/`, `vehicle/cargo-types/`.
- Dead code: `rejectTripRequest` → `https://songa.kalen.co.tz/driver/trip-requests/<id>/reject` (no `/api`), `driver_location_service.dart`, duplicate loading-alert fn; `offers/` and `scheduled/` UI have no API.

**Cross-app contract observations**
- Offers reach drivers only over WS; FCM is chat-only in both apps. Match outcome reaches the rider by REST polling + WS.
- Rider never sees the driver's `driver_arrived_at_*`/`trip_completed` frames; it relies on server broadcasts triggered by the REST stage endpoints.
- Both apps tolerate `canceled`/`cancelled`, many key aliases for driver/fare/location, and minutes for loading durations. This tolerance is a sign the payload contract drifted over time and is not pinned anywhere except `API_REFERENCE_AND_SWAHILI_TRANSLATION.md` and `docs/CHAT_INTEGRATION_GUIDE.md` / `MOBILE_RECEIPT_INTEGRATION.md`.
- Both apps have an EN→SW table for backend error strings (`l10n/backend_error_translations.dart`), i.e. the §10 catalogue was implemented client-side; the backend still has no `error_code` field.

## 6. Admin dashboard and website

**songa_admin** — Next.js 16 App Router (React 19, TS 5.9, Tailwind v4, shadcn/radix, TanStack Query + Table, react-hook-form + zod, recharts, maplibre + deck.gl + h3-js). Built on the "Studio Admin" template; many template screens (academy, ecommerce, kanban, mail, invoice, roles, legacy v1) are still present and call no API.
- **Backend link**: `NEXT_PUBLIC_API_URL` / `NEXT_PUBLIC_WS_URL` (local `.env.local` points at UAT `songa.kalen.co.tz`). One client `src/lib/api/client.ts` (`apiFetch`, `apiDownload`, `withReason`): Bearer header, one refresh-and-retry on 401, DRF error parsing, `X-Change-Reason` header on money/pricing writes for the audit trail.
- **Auth**: hand-rolled SimpleJWT. `POST /token/` → cookies `songa_access` (1d), `songa_refresh` (30d), `songa_user` (roles, `is_staff_dashboard`). `src/proxy.ts` guards `/dashboard*` by cookie presence only; backend enforces RBAC (11 Django groups listed in `src/config/roles.ts`, nav-only on the client).
- **Realtime**: `use-driver-socket.ts` → `${WS_URL}/ws/trips/admin/?token=` for live driver frames (Java gateway), falls back to REST polling.
- **Admin API consumed** (all under `/api/admin/`): analytics (overview/funnel/geography/trips/revenue), riders + drivers CRUD/moderation (suspend/reinstate/disable/enable, KYC approve/reject, `drivers/options/`), KYC queue/stats, trips (list/detail/route/actions/stats/cancel/set-price/receipt/receipt-pdf/timeline), events + export, config audit/history/as-of, finance (stats, settlements verify/complete/reject, wallets hold/release, ledger + verify-chain, invoices, accounts, payments, transactions, tax-liabilities), config (regions, districts, service-charge, disbursements, payment-gateways, pricing-model + engines), live (drivers, h3-density, h3-demand, trips, matching, surge), pricing (brackets, zones, zone-profiles, fixed-routes, preview, v3 cargo-profiles/vehicle-tiers/tariffs/recovery-classes/liquid-rates), vehicles (car-types, cargo-types, makes, models, fleet), staff, logs (services/tail/archive).
- **Real pages**: default, analytics, logistics (trip list/detail/route/timeline/receipt/set-price), drivers (+ profile, edit, verification tab), riders (+ profile, edit), live-map, pricing (tariffs / zones map / model / configuration), finance, fleet, config, audit, infrastructure (+ live/archived logs), users (staff).
- **Loose ends found**: sidebar links "Support Desk" (`/dashboard/support`) and "Broadcasts" (`/dashboard/broadcasts`) have no page; `/api/admin/me/` is configured but never called; 40 uncommitted entries = the unfinished Drivers/Riders split (staged deletes of `crm/`, untracked `drivers/*` + `riders/*`, unstaged edits to api/types/sidebar). Backend half of that split is also uncommitted.
- `docs/design-system.md`: navy `#001f54` primary, green `#00b384` accent only, Figtree/Plus Jakarta Sans, StatCard/ChartFrame/PageHeader, no raw colours in components.

**website/** — React 19 + Vite 8 + Tailwind 4 + GSAP/motion single page (Nav, Hero, CargoMarquee, HowItWorks, Manifesto, Fleet, Drivers, Trust, Cta, Footer). No backend calls. Ships `public/.well-known/tz-e-ticketing-server` which is the LATRA/CeTS JWKS discovery doc, so songa.co.tz must keep serving it (traefik routes that path to Django).

**Top-level docs**: `API_REFERENCE_AND_SWAHILI_TRANSLATION.md` (Aug-28 mobile API catalogue + EN→SW string table; recommends an `error_code` field, not done), `NEARBY_DRIVERS_BEHAVIOR_CONTRACT.txt` (online = status flag AND fresh GPS ≤300s; driver app must heartbeat every ~25s), `MOBILE_DRIVER_ONLINE_GUIDE.md` (Sept: gateway no longer creates driver state from a bare ping; REST go-online first), `RIDER_FCM_REGISTRATION_BUG.md` (rider app skips FCM token registration when access token not yet loaded), `DROPLET_ACCESS_AND_TESTING.md` (prod compose stack: Django, celery worker+beat, kafka consumer, Kafka, Redis, pgbouncer, watchtower).

**Misc dirs**: `LATRA/` (CeTS API 2.1.1 + Latra_Integration PDFs), `selcom/` (payment spec PDF), `KEYS/` (Apple `.p8` APNs key, not read), `build/` (~500 KB stray Flutter/Gradle output, deletable), `Songa-resources/` (brand SVG/PNG, duplicated into `website/brand-source/`), `.claude/skills/gpt-image/.env` holds an OpenAI key.

## 7. Environments, deploy, infra

- **Hosts**: UAT/prod = one DigitalOcean droplet `161.35.167.245` (`ssh root@…`), domains `songa.kalen.co.tz` + alias `uat.songa.co.tz` (traefik multi-host rule); `songa.co.tz` serves the marketing site from its own compose project except `/.well-known/tz-e-ticketing-server` + `/api/latra/jwks/` which route to Django. Managed Postgres (PostGIS) firewalled to the droplet, reached via pgbouncer; DO Spaces for media; Sentry; Gmail SMTP; NextSMS.
- **Images** (`.github/workflows/deploy.yml`, push to `main`): `ghcr.io/kalen-limited/songa_backend`, `netty_ws_gateway`, `songa_nginx`; CI runs `makemigrations --check` only, no tests. **Watchtower auto-deploys `:latest`** for the backend stack within minutes; it does NOT recreate the admin dashboard container (manual `compose pull && up -d`). **Pipeline never migrates** — coordinated deploy per `docs/SERVER_OPERATIONS_AND_DEPLOY_SKILL.md`: stop watchtower → push → pull → `migrate` → `ensure_event_partitions` → `up -d` → start watchtower. Running compose file is `/root/songa-backend-app-v2.yml` on the droplet, not the repo's `docker-compose.prod.yml`.
- **Containers (prod)**: nginx, songa_backend (uvicorn ×4), django_kafka_consumer (`run_kafka_consumer`), netty_ws_gateway, celery_worker, celery_beat, kafka, redis, pgbouncer, watchtower (+ flower in dev).
- **Celery beat (10 jobs)**: car-type cache warm 30 min; locations Redis→DB 5 min; stale-request reaper 60 s; DB→Redis driver sync 3 min; account holds 1 h; gateway reconciliation 2 min; auto payout 1 h; LATRA resync 60 s; LATRA daily report 21:30 UTC; event partitions 01:15.
- **Local dev**: no working local Python (Homebrew 3.14 expat bug). Backend tests run in Docker: `docker-compose.test.yml` + `Makefile` (`make test-up test-deps test APPS=…`), or `.pv2.sh`. ~165 tests green except 5 pre-existing `trips.tests` fixture errors. Dashboard checks: `npx tsc --noEmit`, Biome, `next build`, Playwright recipes in memory.
- **Management commands worth knowing**: `run_kafka_consumer`, `seed_admin_groups`, `grant_admin`, `generate_latra_keypair`, `sync_latra_trips`, `ensure_event_partitions`, `audit_pricing`, `send_test_push`, `backfill_*`.
- **Postman**: `songa_backend_revamp/Songa_Admin_API.postman_collection.json`, `Songa_Driver_Rider_API.postman_collection.json`.

## 8. Gaps, risks and open items I noticed

**Uncommitted / in-flight work (both repos must ship together where noted)**
- `songa_admin` main: 40 uncommitted entries = Drivers/Riders split + verification tab. Its backend half (`admin_api` driver PATCH car type/cargo, `drivers/options/`) is also uncommitted on backend main. Deploy needs backend first or the edit form 404s.
- Backend local main has uncommitted LATRA follow-ups from memory notes (cargo filter in `PrepareTrip.py`, `_cargo_type_fields`, VAT loss-guard widening). Verify with `git status` before any push, since push = auto-deploy.
- `songa-rider-app` has an in-progress merge (`MERGE_HEAD`). Not ours to resolve, but worth telling the app developer.
- Admin branches `feature/pricing-engine-v2` (edb985e/d0fab2d) still unpushed per notes; verify whether superseded by `2aa2a6e` on main.

**Contract mismatches to verify against Django**
- Driver app posts `driver/status/` in two shapes (`lat/lng` string bool vs `latitude/longitude` bool). Check `DriverStatusView` accepts both or the online page's call fails.
- Driver app sends loading `end_time` as EAT wall-clock without offset → `loading_time_taken` skewed 3 h, feeds waiting-time billing (known since LATRA work; backend stamps `trip_end_time` server-side as a workaround).
- Rider `pickup_geolocation_google='POINT (lat, lon)'` is lat-first; confirm the backend parses it that way or ignores the field (it reverse-geocodes from lat/lon anyway).
- `cargo_types` arrives as the string `"[1, 2]"` on driver registration; backend must `json.loads` it.
- Trip status choices `ACCEPTED`, `EN_ROUTE_TO_PICKUP` are never set server-side; apps also use local-only `ARRIVED_AT_PICKUP`, `AWAITING_PAYMENT`. Dashboard/analytics that group by status will never see the first two.
- Rider app ratings send `score`+`comment` only; backend supports structured `aspects` (from `RatingAspectConfig`) and `requires_rating.mandatory`. Aspect analytics will be empty.
- Driver payouts: backend `disbursements/` + `payment-methods/` exist and are admin-switchable, but the driver app has no UI for them. Drivers can only pay debt, not withdraw.

**Backend hygiene**
- `REST_FRAMEWORK.DEFAULT_PERMISSION_CLASSES` is commented out: any view without explicit permissions is public. Quote/nearby/OTP are intentionally open, but a future view that forgets `permission_classes` is open too.
- Dead/legacy code: `trips/consumers/*.py` (Channels, unrouted), `optimized_consumers/`, `KAFKA_TOPICS` setting, `trip_events` topic, `GetAllDrivers/`, `sbr/docker-compose.yml`, `.github_backup/`.
- `monitor_stationary_driver_stalling` task exists but is not on the beat schedule. `TripLocationHistory` declared partitioned but migrated as a plain table.
- No test run in CI (`deploy.yml` only `makemigrations --check`); 5 pre-existing `trips.tests` fixture errors.
- Telemetry staleness still 300 s (contract says drop to ~120 s only after the driver heartbeat is live in the field; the driver app now has the 25 s heartbeat, so step 3 of the rollout is unblocked once confirmed on real devices).
- Known money decisions still open for the owner/accountant: VAT 16% vs TZ 18%, VAT base, real LATRA levy rate, booking fee amount, who funds V3 cancellation compensation, Bajaj/TukTuk car type for CeTS `TRICYCLE`.
- Rider mobile-money fares still "complete" with no real collection (only card charge-first and driver USSD pay-in move money).

**Apps / release hygiene** (for the app developer)
- Both apps: hard-coded prod host, no flavors, debug signing on release, iOS push entitlement off, Directions key in source. Driver app Android id is the `com.example.*` placeholder and the Play "Update" link points at a third id.
- Rider FCM registration race (documented), no foreground push handler in either app.
- Push can only ever deliver chat; offers depend on the WS + background isolate staying alive on OEM power-managed phones.

**Workspace hygiene**
- `build/` (~500 KB stray Gradle output) can be deleted. `KEYS/AuthKey_*.p8` and `.claude/skills/gpt-image/.env` hold secrets in a non-git folder; fine locally but keep them out of any repo. `Songa-resources/` duplicates `website/brand-source/`.
- Dashboard sidebar links "Support Desk" and "Broadcasts" point to pages that do not exist; template screens (academy, ecommerce, kanban, mail…) still shipped in the admin build.

## Recommended approach / next steps (no code changes made)

This task was a scan; the deliverable is the understanding above. If Kalen wants to act, the highest-value follow-ups in order:
1. Commit and ship the Drivers/Riders split (backend first, then admin container recreate), after `git status` on backend main confirms what else is sitting there.
2. Verify the three request-shape mismatches (driver status body, loading `end_time` timezone, `cargo_types` string) against the views and either make the backend lenient or hand the app developer a one-page fix list.
3. Add `error_code` to API error responses so the apps' EN→SW tables stop keying on English strings.
4. Decide on telemetry staleness 300→120 s now that the driver heartbeat exists.
5. Delete dead Channels consumers and the stray `build/` dir; remove dead sidebar links.
