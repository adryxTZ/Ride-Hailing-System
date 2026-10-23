# Songa workspace scan — what I understand

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

(Sections below are filled in from the exploration agents.)

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

---

# PART 2 — Generic multi-service mobility platform (project folder "Rider & Hauler"; brand is a variable)

## Decisions taken by Kalen (2026-10-14)

| Topic | Decision |
|---|---|
| Backend language | **Go**. C was considered; Uber uses Go/Java for services and C/C++ only in hot libraries (H3, routing), so Go + the C H3 library via `h3-go` is the Uber pattern. |
| Project | **Greenfield** new project "Rider & Hauler" at `/Users/kalen/Desktop/ADRIAN/RIDER & HAULER/`. Songa is the reference system, not a migration source. |
| Data | **Clean start.** Only reference data carried over: car types, cargo types, regions/districts, pricing config values. Old Songa DB kept as read-only archive. |
| Scope | **Multi-service, service-first apps.** Rider app opens on a service picker: Taxi, Cargo hauling, Recovery/towing, Water (clean delivery + waste/septic), more later. **Ship hauling first**, then taxi, then others. |
| Apps | **React Native** rider + driver apps. Next.js admin kept and repointed. |
| Deliverable now | **Full redesign document** in `RIDER & HAULER/DOC/`, plus a copy of the Part 1 scan as the first working note. |

## Product vision from the Songa proposal docs (ADRIAN/*.pdf, Jul–Aug 2026)

Doc1 "Five revenue lines": fleet management (FleetOwner, DriverAssignment, owner wallet), cargo-weight pooling (weight/volume, TripLeg, remaining capacity), scheduling (`scheduled_for`, RRULE recurrence, pre-dispatch job), B2B corporate (CorporateAccount, credit terms, DeliveryJob, consolidated monthly invoice, proof of delivery, bulk CSV, ERP API), goods selling (vendor catalogue, 3-way ledger split). Doc2 "Songa Track": telematics SaaS (Tenant, Asset, Device, Geofence, AlertRule, Webhook, share links, hardware trackers, per-asset billing, PDPA consent). Doc3 audit: real lifecycle today skips pickup states; dispatch is broadcast-to-top-5 first-accept-wins; surge off; 10% commission; proposes driver paid subscription waiving commission, manual dispatcher assignment, leakage detector, back-to-back offers, zone/time-varying matching radius. None of the docs mention TRA/EFD; LATRA is required. Doc3 said passengers were out of scope then; Kalen's new direction adds Taxi as a service.

**Requirements the redesign must honour** (merged): service catalogue with per-service request fields, vehicle classes, lifecycle, pricing and LATRA mapping; explicit pickup states + server-measured loading/unloading; Redis/H3 matching with configurable radius per region/time; one deliberate dispatch model; manual assign (RBAC + audit); scheduled + recurring; weight/volume vs capacity; multi-leg (later); fleet owners (later); corporate (later); goods (later); loaders as priced add-on; brackets + waiting + floor + surge(clamped, locked at quote); fixed-route vs GPS-odometer billing; configurable commission, cash→debt, digital→auto-net; holds; hash-chained ledger generalised to many payers/payees; Selcom; invoices with VAT (+TIN later); fraud detectors that can fire; LATRA submit + fail-closed vehicle verification; PDPA; EN/SW; live H3 supply/demand; in-trip chat; push.

## Design (my draft; to be reconciled with the design agent's report)

### D0. Generic, white-label platform (Kalen: "the design should be generic, system name should be a variable")

The product is a **multi-service mobility and logistics platform**; "Songa" and "Rider & Hauler" are brands running on it, never identifiers in code.

- **Brand is data.** A `brands` table (single row at launch, `brand_id` column on tenant-scoped tables from day one): `name`, `short_name`, `legal_entity` (+ TIN/VAT registration), logos, colour tokens, support phone/email, domains (API, web, deep-link scheme), app-store ids, default locale, currency, timezone, country. Env `PLATFORM_BRAND=<code>` selects it for single-brand deployments.
- **Every user-facing string is templated**: SMS (`{{brand.name}}: Hi {{driver.first_name}}…`), emails, push, receipts/invoices (brand + legal entity block), WS messages, app strings. SMS sender ID per brand. No literal product name anywhere in `internal/`.
- **Country adapters behind interfaces**: `Regulator` (TZ: LATRA CeTS; others later), `PaymentGateway` (Selcom now; M-Pesa Daraja, Flutterwave later), `SMSProvider` (NextSMS; Africa's Talking, Twilio), `RoutingProvider` (Google; Valhalla), `ObjectStore` (DO Spaces; S3). Chosen per brand in config.
- **Money and locale generic**: amounts in minor units + ISO currency code; tax lines (VAT, levy) are configured rows, not hard-coded names; languages pluggable (EN/SW shipped); timezone per brand.
- **Service catalogue is per brand**, so one brand can be taxi-only and another hauling-only on the same code.
- **Tenancy**: one deployment per brand by default (simplest ops, hard isolation); `brand_id` everywhere makes shared-DB multi-brand possible later and feeds the telematics SaaS tenant model from the vision docs.
- **Apps are white-label builds** from one monorepo: `brands/<code>.json` (name, slug, bundle ids, icons, splash, colours, API URL, deep-link scheme) read by Expo `app.config.ts`; EAS build profile per brand; runtime theme/logo also served by `GET /v1/app/config` so a light rebrand needs no store release.
- **Admin** reads brand name/logo/colours from the API (today's Next.js has "Songa" hard-coded; that goes).
- **Naming**: repo and binary use a neutral codename (`platform` below; rename freely), DB and module names are domain words. Docs refer to "the Platform" and carry a variables table (`PLATFORM_NAME`, `BRAND`, `COUNTRY`, `CURRENCY`, `REGULATOR`, `GATEWAY`) at the top.

### D1. Shape of the system

One Go binary, `platform` (codename), with roles selected by flag: `platform serve --roles=api,realtime,worker,scheduler` (launch: one container running every role), later split into `--roles=api,realtime` ×2 and `--roles=worker,scheduler` ×1 by a compose change only. Also `platform migrate`, `platform seed reference`, `platform verify-ledger`, `platform keys rotate`. Everything else is Postgres (managed, PostGIS) and one Redis.

```
Caddy/Traefik (TLS) ──▶ platform api ×2  (REST /v1 + WS /v1/ws, ~50–80 MB each)
                        platform worker ×1 (River jobs, periodic schedule, outbox relay, ~40 MB)
Postgres+PostGIS (managed)  ── durable data, job queue (River), trip_events outbox
Redis                       ── presence (H3 sets + TTL hashes), GPS stream, WS fan-out pub/sub, rate limits, caches
DO Spaces                   ── documents/photos via presigned URLs
Next.js admin (kept)        ── repointed to /v1/admin via generated client
```

What disappears vs Songa: Java gateway, Kafka, Celery worker/beat/flower, uvicorn workers, pgbouncer (pgx pool), Django consumer, two-way Redis↔DB sync jobs. Container count ~11 → 5. RAM budget: whole backend < 500 MB at launch scale.

Key library choices (boring, maintained): `chi` router + `oapi-codegen` (server stubs + request validation from `api/openapi.yaml`), `pgx/v5` + `sqlc`, `goose` migrations (embedded, run by `rh migrate` in the pipeline), **River** job queue on Postgres (transactional enqueue, retries, unique + periodic jobs; replaces Celery and beat), `nhooyr.io/websocket`, `uber/h3-go` (wraps Uber's C H3), `go-redis`, `log/slog` JSON, OpenTelemetry (`otelhttp`, `otelpgx`), Sentry Go SDK, `santhosh-tekuri/jsonschema` for request schemas, `testcontainers-go` for integration tests. TypeScript client generated from the same OpenAPI with `orval` (TanStack Query hooks) for RN and admin.

Why River over asynq/NATS: dispatch and billing jobs are enqueued in the same transaction as the trip row, so "dispatch after commit" races and lost jobs disappear; zero extra infrastructure; load is trivial at thousands of trips/day. Why Redis pub/sub for WS fan-out: already present, one channel per group, enough for 2–5 instances; NATS only if we ever go beyond that.

### D2. Module layout (modular monolith, bounded contexts)

```
cmd/platform/               main + subcommands (codename; rename freely)
api/openapi.yaml            THE contract (single source for server stubs, TS client, docs)
db/migrations/              goose SQL
brands/                     brand config seeds (songa.json, …) — data, not code
internal/kernel/            config, db, redis, auth(jwt/otp), i18n, idempotency, outbox, http, otel, errors, brand loader
internal/providers/         Regulator / PaymentGateway / SMSProvider / RoutingProvider / ObjectStore interfaces + implementations
internal/identity/          users, roles, sessions, devices, KYC documents
internal/catalog/           services, vehicle classes, request schemas, lifecycle profiles, pricing policies, reference data
internal/fleet/             drivers, vehicles, eligibility, assignments (fleet owners later)
internal/presence/          online state, GPS ingest, H3 index, freshness/GC (Redis)
internal/dispatch/          candidate search, scoring, offer waves, accept lock, re-dispatch, manual assign
internal/trips/             quotes, requests, generic FSM driven by lifecycle profile, stops, trip_events
internal/pricing/           engines (bracket port, per-km/min, fixed menu), zones, fixed routes, surge, parity tests
internal/money/             chart of accounts, hash-chained ledger, fare split, wallets, invoices, settlements, payouts, holds
internal/providers/selcom/  PaymentGateway impl: business API (RSA) + checkout API (HMAC), webhooks, reconciliation
internal/compliance/        generic submissions table + Regulator interface; providers/latra = CeTS adapter, JWKS, daily report
internal/messaging/         chat, quick replies, push (FCM/APNs)
internal/realtime/          WS hub, protocol, subscriptions, Redis fan-out, replay
internal/admin/             staff endpoints, audit log, analytics queries, config editing with reasons
internal/events/            system_events activity log + outbox relay
internal/jobs/              River worker registry + periodic schedule (one file = the whole "beat")
```
Rules: modules talk via Go interfaces and domain events (outbox-backed, in-process bus); `trips` orchestrates; `money`, `compliance`, `messaging`, `realtime` react to `trip_events`. No module imports another's DB tables. Nothing under `internal/` may contain a brand name; a CI grep enforces it.

### D3. The service abstraction (the core of "service-first")

Hybrid: **data for everything that varies by data, a tiny Go plugin interface for behaviour that must be code.**

- **Service** row: code (`cargo_hauling`, `taxi`, `recovery`, `water_clean`, `water_waste`, …), names EN/SW, icon, category, status, sort order, `lifecycle_profile` (enum), `pricing_policy_id`, `request_schema_id`, `compliance` (LATRA category, submit_trip bool), `availability` (regions, hours), `matching` overrides (radius, offer window, wave size, candidates), rating aspects.
- **VehicleClass** rows per service (taxi: Standard/XL; hauling: Kirikuu/Canter/Tipper/Tanker/Towing with tonnage; water: 5k/10k L tanker; recovery: flatbed/tow): capacity attributes (seats, max_load_kg, volume_l), LATRA vehicle type, icon, tariff. A **Vehicle** holds eligibilities to many classes; a driver's offerable services = vehicle eligibilities ∩ driver qualifications ∩ KYC approvals.
- **RequestSchema**: versioned JSON Schema (2020-12 subset) + `x-ui` hints (widget, labels EN/SW, order). Taxi: `passengers`. Hauling: `cargo_type_id, weight_kg, volume_l, loaders, notes`. Water clean: `litres, delivery_point`. Water waste: `tank_size, access_notes`. Recovery: `vehicle_make_model, recovery_class, wheels_locked`. Server validates; apps render the form dynamically → **adding a service needs no app release**.
- **LifecycleProfile** (code, a handful): `pickup_dropoff` (REQUESTED→OFFERED→ACCEPTED→EN_ROUTE_TO_PICKUP→ARRIVED_AT_PICKUP→IN_PROGRESS→ARRIVED_AT_DROPOFF→COMPLETED), `load_haul_unload` (inserts LOADING / UNLOADING with server-measured timers and waiting fees), `multi_stop` (later). A profile defines phases, who may trigger each transition, timers, cancellation windows. The FSM engine is generic; a profile is ~100 lines of Go.
- **PricingPolicy**: engine (`bracket_distance_time` = Songa V1 port, `per_km_per_min`, `fixed_menu`, `quote_on_inspection`), tariff per vehicle class, add-ons (loaders, waiting, pickup approach, booking fee), surge on/off + clamp, min fare, cancellation fee, billing basis (route estimate vs GPS odometer, +25 % cap).
- **Plugin interface** (optional per service): `ValidateRequest`, `EnrichQuote`, `OnTransition`. Default no-op.
- Worked example, "add Water delivery": insert Service + 2 VehicleClasses + RequestSchema + PricingPolicy (per-km + volume surcharge) + LATRA mapping; choose `load_haul_unload` (fill at depot = LOADING, discharge = UNLOADING). Zero Go unless a special rule (e.g. waste must end at a licensed disposal site → one `OnTransition` hook).

### D4. Core data model

Postgres: `brands` (see D0; `brand_id` on every tenant-scoped table below); `users`, `user_roles`, `staff_groups`, `otp_codes`, `refresh_tokens`, `devices`, `kyc_documents`; `drivers`, `vehicles`, `vehicle_class_eligibility`, `driver_vehicle_assignments`; `services`, `vehicle_classes`, `cargo_types`, `regions`, `districts`, `request_schemas`, `pricing_policies`, `tariffs`, `pricing_brackets`, `pricing_zones`, `zone_profiles`, `fixed_routes`, `surge_configs`, `matching_configs`, `charge_settings` (commission, VAT rate/base, levy, booking fee) — all money/config tables versioned for "config as-of"; `quotes` (inputs, options per class, polyline, surge, tariff version, expiry), `trips` (uuid, public code, service, class, rider, driver, vehicle, status, pickup/dropoff geog + addresses, `request_fields` jsonb, quote id, fare quoted/final, `billing_snapshot`, payment method, pricing source, zone, fixed route, `scheduled_for`, cancel info, latra ref, `seq`), `trip_stops` (2 rows now, N later), `trip_events` (trip, seq, type, actor, payload, at — outbox + audit + WS replay), `trip_offers`, `trip_locations` (monthly partitions), `ratings`; `accounts`, `transactions`, `ledger_entries` (append-only, hash chain, trigger-protected), `wallets` (owner type/id, balance, debt, hold), `invoices` (receipt of record, immutable once issued, `lines[]` EN/SW), `settlements`, `payouts`, `gateway_transactions`, `payment_methods`; `compliance_submissions` (kind, target, payload, status, attempts, next_attempt_at, reference); `chat_messages`, `quick_replies`; `audit_log`, `system_events` (partitioned); River tables.

Redis: `drv:{id}` hash (lat, lng, h3, heading, speed, status, services[], classes[], trip_id, updated_at) with **TTL = freshness window (120 s)**; `h3:{cell}:drivers` sets; `drv:lastseen` zset for GC; `trip:{id}:path` stream; `ws:{topic}` pub/sub; OTP/rate-limit counters; quote cache. Presence expiry = offline; no DB sync loop (DB only records online/offline transitions for analytics).

### D5. Booking and dispatch — Uber's principles at Songa's scale (Kalen: "Songa is just reference; design like Uber")

**What Uber actually does, and what we adopt**

| Uber practice | Adopt? | How, at our scale |
|---|---|---|
| H3 hexagonal indexing of supply and demand at several resolutions | Yes | H3 res 9 for presence/matching, res 7 for surge and heatmaps, res 5 for analytics. Uber's C library via `h3-go`. |
| **ETA-based matching, not straight-line distance** | Yes | H3 pre-filter → road ETA for the top-K candidates via a routing engine, cached per (cell, cell, 15-min bucket). |
| **Batched matching**: hold requests a few seconds, solve all open requests × candidates as one assignment problem | Yes | 3 s batch window (per service/density). Min-cost bipartite assignment (Hungarian / min-cost flow) over tens × tens. One request in the batch degenerates to best-driver pick, so no downside. |
| Sequential offers to the chosen driver with a short acceptance window, re-solve on decline | Yes, default | 15 s offer; decline/timeout re-enters the next batch. Configurable **broadcast fallback** for sparse supply (cargo at night, upcountry). |
| Forward dispatch / trip chaining (match a driver who is about to finish) | Yes | Drivers in UNLOADING/ARRIVED_AT_DROPOFF with predicted ≤ 5 min to free are candidates with a time penalty. |
| Acceptance- and cancellation-probability models in the objective | Yes, heuristics first | Per-driver acceptance rate, distance decay, cancel-after-accept rate; logistic model later once data exists. |
| Dynamic pricing per hex with neighbour smoothing and demand forecasting | Yes, simple first | Surge = f(open requests / fresh supply) per res-7 cell, smoothed over k=1 neighbours, clamped 1.0–2.0, locked at quote. Forecasting later. |
| Driver positioning guidance (where demand is) | Yes | Res-7 demand heat and "hot zones" in the driver app; cheap, improves supply distribution. |
| Durable long-running trip workflows (Cadence/Temporal, born at Uber) | Principle yes, product no | Dispatch and trip lifecycle modelled as explicit **sagas** on River with persisted state and timers. Temporal would add 400 MB+ of services; revisit if we outgrow River. |
| In-memory geo-sharded supply (Ringpop) | Not yet | Redis presence is enough for thousands of drivers. Path to Redis Cluster or in-memory sharding later. |
| Upfront pricing with ML ETA (DeepETA) | Principle yes | Upfront quote from routing ETA × per-hex/hour correction factor learned from completed trips. No ML at launch. |
| Marketplace metrics and experimentation | Yes | Time-to-match, ETA-at-match, acceptance, cancel-after-match, unfulfilled rate per hex; dispatch parameters are per-region/service config flags for A/B. |

**Supply side**
- Go online = `PUT /v1/driver/status` with position + capabilities → Redis hash with TTL = freshness window (120 s) + H3 res-9 set; each GPS ping refreshes TTL and moves set membership; a 30 s GC sweeps expired drivers. Presence expiry is the one definition of offline.
- Driver state also carries: current trip + predicted free-at time (for forward dispatch), acceptance stats, services/classes eligible, heading.

**Booking (robust by construction)**
1. Quote: `POST /v1/quotes` with service, stops, request fields → validated against the RequestSchema → route + ETA from the routing engine → options per vehicle class (fare, ETA-to-pickup from nearest fresh supply, availability) → quote stored with tariff version, surge, expiry (3 min). Expired quote → re-quote with diff shown.
2. Request: `POST /v1/trips` with `quote_id`, option, payment method and `Idempotency-Key` → one active trip per rider enforced by a partial unique index, not application checks → trip `REQUESTED` and `DispatchSaga` job enqueued in the same transaction.
3. Rider sees live state via WS `trip.event` + `GET /trips/{id}/events?after_seq=`; "keep searching", "widen vehicle class", "schedule instead" offered on `NO_DRIVER_FOUND`.
4. Destination change mid-trip = new stop version with re-price (actual-distance billing with +25 % detour cap, from Songa).
5. Scheduled bookings: `scheduled_for` → pre-dispatch saga starts at lead time (per service), reminders, re-dispatch on failure.

**DispatchSaga (per request, persisted state, timers via River)**
```
GATHER   candidates = H3 kRing expand (k=1,2,4,8,16,32) until ≥N or radius cap(service, region, hour)
         filter: eligible(service,class) ∧ fresh ∧ verified ∧ ¬held ∧ ¬declined(this trip) ∧ (free ∨ forwardDispatchable)
SCORE    cost(d,r) = w1·ETA(d→pickup) + w2·(1−P(accept|d,r)) + w3·P(cancel|d) + w4·fairness(idle time) + w5·forwardPenalty
SOLVE    every 3 s: assignment over all open requests in the region × their candidates, minimise Σcost (Hungarian)
OFFER    offer.new to the assigned driver, 15 s timer; accept = atomic UPDATE trips … WHERE status='OFFERED' AND driver_id IS NULL
         decline/timeout → mark declined, back to SOLVE; sparse-supply mode → offer to top-3 simultaneously, first wins
EXHAUST  after total window (e.g. 120 s, per service) → NO_DRIVER_FOUND → rider options; dispatcher queue for manual assign
RECOVER  driver cancels before pickup → saga restarts excluding that driver; rider notified "finding a new driver"
```
Service config decides the knobs: taxi = 3 s batch, 10 s offer, 5 km cap, sequential; hauling = 5 s batch, 20 s offer, 30 km cap, sequential with broadcast fallback after 2 rounds.

**Routing / ETA engine**: Phase 1 Google Routes/Distance Matrix (already licensed) for quotes, with an H3 (cell,cell,hour) ETA cache for matching; Phase 2 self-hosted **Valhalla** on a Tanzania OSM extract on a second small droplet (tiled graph, a few hundred MB) to cut cost and latency, Google kept for traffic-aware rider quotes.

**Manual and assisted dispatch**: dispatcher console (admin) lists unfulfilled requests, can force-assign or nudge an offer; all audited. This is the human desk Doc3 found at every freight peer.

**Realtime transport**
- One endpoint `/v1/ws`, auth by first frame (no token in URL logs). Envelope `{id, type, seq?, ts, trip_id?, data}`. Client→server: `auth`, `subscribe`, `ping`, `location`, `chat.*`, `ack`. Server→client: `ready`, `pong`, **`trip.event`** (one type, `event` field — kills the alias zoo), `offer.new`, `offer.closed`, `driver.location`, `chat.message`, `error{code}`.
- Heartbeat: client 20 s, server drops after 60 s silence; driver location doubles as heartbeat. **Resume**: client sends `last_seq` per trip; server replays `trip_events` with `seq > last_seq`. At-least-once delivery, client dedups on `seq`. Cross-instance fan-out via Redis pub/sub published by the outbox relay.
- HTTP fallback `POST /v1/driver/location` (batch) when the socket is down; same freshness semantics. High-priority data push as a wake-up for offers when the socket is down.

**Quality loop**: every match writes `match_decisions` (candidates, costs, chosen, outcome) so the scoring weights can be tuned from data and the acceptance/cancel models trained later. Metrics per hex/hour on a Grafana board from day one.

### D6. API contract (`/v1`)

Groups: **public** (`services` catalogue incl. classes + schemas, `quotes`, `nearby-drivers`, `reference/*`, `health`), **auth** (`otp/request`, `otp/verify`, `refresh`, `logout`, `staff/login`), **me** (`me`, `devices`, `payment-methods`, `ratings`), **rider** (`trips` POST from `quote_id` + option + fields + payment method with `Idempotency-Key`; `trips/{id}`, `/cancel`, `/events?after_seq=`, `/receipt(.pdf)`, `/rating`, `/messages`; `trips` list cursor-paginated; `me/destinations`), **driver** (`driver/status`, `driver/location`, `offers`, `offers/{id}/accept|decline`, `trips/{id}/transition {to}` validated against the lifecycle profile, `driver/wallet|ledger|settlements|payouts|earnings|vehicles|documents`), **fleet** (later), **admin** (`/v1/admin/...` mirroring today's surface), **partner** (`webhooks/selcom`, `latra/jwks`, `.well-known/tz-e-ticketing-server`).

Errors: `{error:{code:"TRIP_ALREADY_ACTIVE", message, details, request_id}}`, message localized by `Accept-Language` (en/sw); codes enumerated in OpenAPI → apps never key on English. JSON only; uploads by presigned URL. Versioning: URL major, additive within `/v1`, deprecation headers, `GET /v1/app/config` for min-version/force-update and feature flags.

### D7. React Native

**Expo** (prebuild + EAS Build + EAS Update for OTA). pnpm + Turborepo monorepo: `apps/rider`, `apps/driver`, `packages/api` (orval-generated client + TanStack Query hooks), `packages/realtime` (reconnect, heartbeat, seq resume, offline queue), `packages/forms` (JSON-schema → RN form renderer for request schemas), `packages/ui` (Songa navy/green tokens, components), `packages/i18n` (EN/SW + error-code catalogue), `packages/maps` (react-native-maps wrapper, polylines, H3 helpers). Expo Router; TanStack Query for server state, Zustand for session + in-progress trip. Driver background location: `react-native-background-geolocation` (Transistorsoft, paid licence, the one that survives OEM power management) with Android foreground service; rider uses `expo-location`. Push: `expo-notifications` (FCM v1 + APNs); offers stay WS-first with a high-priority data push as wake-up when the socket is down. Service picker: `GET /v1/services` → tiles → schema-driven form → quote options → book. Driver app shows only services its vehicle is eligible for.

### D8. Infra and delivery

Before (Songa): nginx, uvicorn×4, kafka consumer, JVM gateway, celery worker, beat, flower, kafka, redis, pgbouncer, watchtower, admin ≈ 11 containers, several GB. After: caddy, rh-api ×2, rh-worker, redis, admin ≈ 5 containers, < 1 GB; managed Postgres external. Image: distroless, ~25 MB. CI (GitHub Actions): golangci-lint → `go test` (testcontainers) → golden parity tests → build/push GHCR → deploy job runs `rh migrate` then rolling restart (no watchtower). Environments local/uat/prod from one image via env. Secrets via SOPS or DO. Observability: OTel → Grafana Cloud free tier (metrics/logs/traces) + Sentry. Backups: managed PG PITR; Redis needs none.

### D9. Phases and effort (rough; assumes 1–2 Go devs + 1–2 RN devs + Kalen with AI tooling)

| Phase | Scope | Backend wk | RN wk |
|---|---|---|---|
| 0 Foundations | repo, CI, kernel pkg, brand config + provider interfaces, OTP auth, OpenAPI skeleton + TS client, RN monorepo with white-label build config, design tokens | 2–3 | 1–2 |
| 1 Catalogue + presence + realtime | services/classes/schemas, driver online/GPS/H3, WS hub + replay, nearby drivers | 3 | 2 |
| 2a Dispatch engine | DispatchSaga on River, H3 candidate search, ETA cache, cost function, batched assignment solver, offers, forward dispatch, broadcast fallback, match_decisions + metrics | 3 | — |
| 2b Hauling trips | bracket engine port + golden tests, quote→request→dispatch→load/haul/unload→complete, cancel, chat | 4 | 4 |
| 3 Money | ledger, fare split, invoices/PDF, wallets/debt/holds, Selcom settlements/payouts/card, reconciliation | 3–4 | 2 |
| 4 Compliance + admin | LATRA submissions/daily/JWKS, `/v1/admin` parity, repoint Next.js | 3 | — |
| 5 Pilot + cutover | UAT, reference-data import, driver re-registration campaign, go-live, Songa archived | 2 | 1 |
| 6 Taxi | `pickup_dropoff` profile, per-km/min tariff, classes, LATRA passenger mapping | 2–3 | 1 |
| 7 Recovery, Water | config + small hooks each | 1–2 each | ≤1 each |

Hauling parity ≈ 15–18 backend weeks + 10 RN weeks ≈ 3–4 calendar months with the small team above. Later tracks (from the vision docs): scheduling/recurring, fleet owners, corporate + consolidated invoices + POD, goods catalogue, telematics SaaS, driver subscriptions, leakage detector.

### D10. Top risks

1. Parity underestimated (two years of Songa edge cases) → use the Part 1 scan as the checklist + golden tests from exported Songa quotes/splits.
2. Driver GPS dying on cheap Android → Transistorsoft + TTL presence + "you look offline" banner.
3. Selcom collections have no sandbox → shadow-run on prod with small amounts before cutover.
4. LATRA identity for a new system → reuse `songa.co.tz` JWKS domain, coordinate re-submission of vehicles/drivers early.
5. Clean start wipes driver balances/debt → settle or zero with comms before cutover.
6. Team Go ramp-up → boring libs, strict review, AI pairing.
7. Over-engineering the service abstraction → ship hauling with exactly two profiles.
8. Admin rework bigger than hoped → generated client + thin adapter layer.
9. PDPA consent/retention for tracking → consent screen + retention job from day one.

### D11. Refinements adopted from the independent design review (reconciled 2026-10-14)

An independent design agent produced a parallel design from the same scan. Where it agreed with D0–D10 nothing changes. These are the refinements I adopted, and the two places I overrode it.

**Adopted**
- **Launch topology**: one all-roles container (≈120 MB idle, ≈250 MB at 600 online drivers) instead of three; split by compose when traffic justifies. Whole stack ≈ 500 MB on a 2 GB droplet. **Separate UAT and prod droplets** (today they share one).
- **Money arithmetic**: `shopspring/decimal`, `numeric(14,2)` in Postgres, decimal strings on the wire; half-up quantisation reproduces Songa to the cent.
- **Auth**: JWT **EdDSA** with `kid` and a public JWKS (`/v1/auth/jwks`), access 15 min, refresh 30 d rotating with family revocation; staff and future partners verify tokens without a shared secret.
- **WS library**: `coder/websocket` (maintained successor of nhooyr). Server frame `presence.stale` tells a driver whose pings stopped for 60 s that riders cannot see them.
- **Presence model**: `presence:{driver}` hash (TTL 2× staleness) + **`cell:{h3}` sorted set with score = last ping time**, so "fresh drivers in a cell" is `ZRANGEBYSCORE now−120s +inf` and GC is `ZREMRANGEBYSCORE`; `cells:active` set for the live map and GC; `offer:{driver}` `SET NX EX` guarantees one outstanding offer per driver.
- **Ledger generalised from day one**: `parties` (PLATFORM, DRIVER, FLEET, CORPORATE, VENDOR, TAX_AUTHORITY, REGULATOR, GATEWAY) are the counterparties of every account; **`posting_rules`** express cash → driver DEBT / commission revenue, digital → gateway float / wallet / revenue, VAT and levy extraction as data, so a vendor or fleet split is new rows, not code; **`fee_policies`** are versioned with `effective_from` and each trip pins the policy id it was settled under (open VAT/levy/booking-fee decisions become data changes). Hash chain kept global under an advisory lock, verified by `platform verify-ledger` and a daily job.
- **Dispatch mechanics**: `trip_assignments` rows (wave, score, offered/expires/responded, outcome OFFERED|ACCEPTED|REJECTED|TIMED_OUT|SUPERSEDED|CANCELLED, source AUTO|MANUAL); River `offer_timeout` job unique per (trip, wave); accept = `SELECT … FOR UPDATE` on the trip, losers get `409 OFFER_TAKEN`; offers go over WS **and** an FCM high-priority data push (Songa relied on WS alone); 30 s reaper closes requests older than the per-service window; manual assign has `force=false` (normal offer flagged manual) and `force=true` (phone-arranged jobs), both audited.
- **Lifecycle**: `AWAITING_PAYMENT` phase and `POST …/confirm-cash`; `EXPIRED` terminal for scheduled trips never dispatched; arrival transitions verified against the last presence ping (`409 TOO_FAR_FROM_STOP` beyond 30 m); client timestamps stored only as `client_at` (kills the 3 h skew bug).
- **Water is two services** sharing one plugin: `water_delivery` (DROPOFF_ONLY: fill at depot, discharge at site) and `septic_pumpout` (PICKUP_ONLY), because their stop rules differ. Service map at launch: hauling (BOTH, loading+unloading, `bracket_meter`), taxi (BOTH, pickup wait clock, per-km + per-min), recovery (loading = hook-up, `dimension` engine with V3 parity), water ×2 (`dimension`, per-litre + distance).
- **Module order enforced** by `golangci-lint depguard`: `identity ← catalog ← vehicles ← pricing ← dispatch ← trips ← ledger ← billing ← payments ← compliance ← messaging ← fraud ← admin`; cross-module reads via caller-defined ports wired by hand in `cmd/platform/wire.go`; side effects only via `trip_events` + registered transition hooks (the outbox). Plus the D2 brand-name grep.
- **Testing**: golden parity suite of ≥ 500 cases exported once from the Songa archive (bracket fare, fare split, invoice lines, ledger entries) failing CI on any cent of drift; `testcontainers-go` integration; every response validated against the OpenAPI spec in tests; two-hub WS test sharing one Redis; k6 load script (600 drivers pinging every 10 s + 1 request/s, p95 < 150 ms, RAM < 400 MB); a **fakes container** (Selcom, LATRA, SMS, FCM HTTP fakes) for local and CI.
- **API conventions**: `X-App-Version` → `426 APP_UPDATE_REQUIRED`; ETags on catalogue GETs; `details.fields[]` on validation errors; Google Places/Directions proxied server-side (no Maps key in the apps); `POST /v1/uploads/presign` for all files.
- **Apps**: `openapi-typescript` + `openapi-fetch` + `openapi-react-query` for the generated client (lighter than orval); CI fails if the generated client is stale; MMKV persistence for session, presence and last trip seq; driver phase screen renders buttons from `trip.allowed_transitions[]` and labels from the profile's `label_key`, so new services need no app release beyond strings; Maestro flows for the two golden journeys against UAT. Background location: Transistorsoft `react-native-background-geolocation` remains my recommendation for the driver app (the reviewer proposed `expo-location` with deferred updates; keep it as the free fallback if licence cost is refused).
- **Admin** moves into the apps monorepo and swaps its hand-written `src/lib/api/*.ts` for the generated client; live map polls `GET /v1/admin/live/drivers?bbox` every 5 s, WS for admins only for trip watch and alerts.
- **Observability**: one Grafana Alloy container ships logs/metrics/traces to Grafana Cloud free tier; dashboards for request p95, WS connections, fresh presence, dispatch funnel (offers → accepts, time-to-accept, no-driver rate), River queue depth, ledger verify, LATRA backlog, Selcom reconciliation lag.
- **Delivery**: PR gate = vet, lint (depguard), sqlc diff, tests, OpenAPI lint, client-diff, golden suite, RN tsc/jest, admin build; main → distroless image → UAT auto-deploy (`compose run migrate` under advisory lock → `up -d --wait` → smoke); prod on tag with GitHub Environment approval; EAS Update channels `uat`/`prod`. Backups: managed PG PITR + weekly encrypted dump to Spaces, Spaces versioning for KYC/receipts, quarterly restore drill.

**Overridden**
1. The reviewer ported Songa's scoring and wave model as the dispatch design. Kalen asked for Uber's approach, so **D5 stands**: batched ETA-based assignment with sequential offers and a broadcast fallback. Songa's score is only the **v0 cost function** inside that solver (keeps golden parity at pilot); v1 adds road ETA from the (cell, cell, hour) cache, acceptance rate and fairness; v2 learns P(accept), P(cancel) and ETA correction from `match_decisions`.
2. The reviewer hard-coded a product name (`rh`) into env vars, channels, trip codes and the module path. **D0 stands**: neutral codename, brand only as data, trip-code prefix from brand config.

**Merged effort** (1 senior Go + 1 senior RN + Kalen ~50 % + QA ~25 %; credentials for Selcom sandbox and LATRA UAT from week 1):

| Phase | Calendar wk | Effort |
|---|---|---|
| 0 Foundation (repo, CI, kernel, brands, auth/JWKS, OpenAPI + client, Expo boot, admin moved) | 1–2 | 6 ew |
| 1 Hauling core (catalogue + reference import, quotes, state machine, presence + WS + replay, dispatch v0→v1, bracket engine + golden suite, fare split + ledger + invoices, chat + push, ratings; both apps' hauling flows; admin KYC/trips/live/tariffs/manual assign) | 3–8 | 24 ew |
| 2 Money + compliance (Selcom pay-in/card/payouts/reconciliation/holds, LATRA queue + daily report + fail-closed toggle, fraud v1, PDPA consents, load test; app wallet/payouts/cards) | 9–13 | 14 ew |
| → Hauling at parity, UAT pilot with ~20 drivers, shadow-compare fares vs Songa | 13–14 | — |
| 3 Taxi (service rows + plugin, pickup-wait clock, per-minute brackets, passenger LATRA mapping, dispatch v2 data collection) | 14–17 | 8 ew |
| 4 Recovery + water + scheduling + fleet MVP (dimension engine with V3 parity, `scheduled_for` + pre-dispatch, fleets as parties with owner wallet) | 18–23 | 14 ew |
| 5+ Corporate + consolidated invoices + POD, RRULE, pooling, goods catalogue, subscriptions, telematics tenants | later | scoped per phase |

To taxi live ≈ 52 engineer-weeks ≈ 17 calendar weeks; with 20 % contingency ≈ 5 months.

**Parity reference files in the Songa workspace** (what the Go port must reproduce; all under `songa_backend_revamp_v2/songa_backend_revamp/`): `payments/fare_breakdown.py` (split invariant, VAT `r/(1+r)`, levy bearer, line codes), `pricing/utils/pricing_algo.py` (bracket fare, locked surge, floor, route vs GPS distance, waiting, loaders), `utils/songa_s1/suggest_driver.py` (v0 score + fairness ZSET), `pricing/v3/models.py` (dimension catalogue), `payments/models/LedgerEntry.py` (hash formula), `payments/gateways/base.py` + `selcom_business.py` (gateway interface), `latra/serializers/*.py` (CeTS payloads, `CAR|MOTORCYCLE|TRICYCLE`), `songa_admin/src/config/roles.ts` (11 staff roles), and `NEARBY_DRIVERS_BEHAVIOR_CONTRACT.txt` (presence/heartbeat contract).

### D12. Songa design flaws removed (Kalen: "this is the time to remove all bad design choices")

Nothing below is carried forward. Each row is a check item for the design document and for code review later.

| # | Songa choice (from the Part 1 scan and the Doc3 audit) | Replaced by |
|---|---|---|
| 1 | Four runtimes (Python, Java, Celery, Kafka) bridged by two Kafka topics just to do websockets | One Go binary; websockets in-process; River jobs on Postgres; no Kafka, no JVM |
| 2 | Redis as a second source of truth with Redis→DB (5 min) and DB→Redis (3 min) sync loops | Presence is ephemeral and derived; Postgres is the only truth; no sync jobs |
| 3 | Trip transitions = REST → Celery task → broadcast; responses say "is being processed"; state changes asynchronously | Transition is one synchronous transaction that returns the new state; events published after commit via the outbox |
| 4 | Status values that no code ever sets (`ACCEPTED`, `EN_ROUTE_TO_PICKUP`); apps invent `ARRIVED_AT_PICKUP`, `AWAITING_PAYMENT` locally | One explicit state machine per lifecycle profile; server returns `allowed_transitions`; apps render from status |
| 5 | Loading/unloading times taken from client wall-clock without offset (3 h skew feeding billing) | All durations measured server-side from transition timestamps; client time stored only as `client_at` |
| 6 | Mixed multipart / form-encoded / JSON bodies; duplicated fields (`car_type` + `car_type_id`); arrays sent as strings; lat-first `POINT (lat, lon)` | JSON only, one OpenAPI-validated schema, uploads by presigned URL, GeoJSON order everywhere |
| 7 | English error strings are the contract; apps keep EN→SW lookup tables keyed on exact text | Stable `error.code` enumerated in OpenAPI; message localized by `Accept-Language` |
| 8 | ~20 websocket event aliases (`trip_canceled` and `trip_cancelled`, five "driver assigned" variants) | One `trip.updated` frame carrying full status; frame types generated from the spec |
| 9 | JWT in the websocket URL query string (ends up in logs) | First-frame auth, token never in a URL |
| 10 | HS256 with one shared `SECRET_KEY` across Django and the Java gateway | EdDSA keys with `kid`, public JWKS, rotation without redeploying clients |
| 11 | No API versioning; PascalCase RPC-style paths (`RequestCarrier`, `TripUnloadingFinished`); two quote endpoints with different parameter names | `/v1` resources in kebab-case; one `POST /quotes`; one `POST /trips/{id}/transitions` |
| 12 | No role on the user; "driver" means "has a Driver row" | Explicit `user_roles` and staff roles; role and brand claims in the token |
| 13 | DRF default permissions commented out, so every new view is public unless it remembers not to be | Deny by default; every route declares its security scheme in OpenAPI; a test fails on undeclared routes |
| 14 | Dead code shipped: Channels consumers, `optimized_consumers`, unused Kafka topics, dead app screens | Module boundaries enforced by depguard; unused code fails lint; no template pages in the admin |
| 15 | Fare, displayed total and invoice disagreed (`fare*1.16` in one place, `fare−commission+VAT` in another) | One `Split()` function, invariant asserted on every split, invoice is the only receipt |
| 16 | Integer primary key plus a separate UUID `trip_id` on the same row (caused a real FK bug) | UUIDv7 primary keys only; public short codes are a separate column |
| 17 | Floats and `×1.16` literals in money paths; VAT rate and 10 % commission hard-coded in several files | `decimal` + `numeric(14,2)`; all rates in versioned `fee_policies` pinned per trip |
| 18 | Dispatch: broadcast to top 5, first accept wins, straight-line distance scoring, "waves" computed but never used | Batched assignment on road ETA with sequential offers and an explicit broadcast fallback; waves are real and configurable |
| 19 | Nearby-drivers cached 60 s per cell on top of a 300 s staleness window (drivers visible up to 6 min after vanishing) | 120 s freshness as a sorted-set query; no result cache |
| 20 | "Online" was two invisible conditions; nobody could tell why a driver was not visible | Single TTL definition of online; `presence.stale` frame to the driver; dashboard metric for fresh supply |
| 21 | Rider polls match status every 4 s and also listens on WS; no replay after reconnect | WS with per-trip sequence numbers and replay from `trip_events`; polling only as a fallback endpoint |
| 22 | Rider mobile-money fares "complete" with no money moving; payment-method enum mismatch between apps and backend | Explicit payment states (PENDING → COLLECTED / CASH_CONFIRMED / FAILED); a trip cannot reach COMPLETED without a terminal payment state; one enum in the spec |
| 23 | Fraud detectors that can never fire; cancellation fee that can never be granted; a default coordinate with lat/lng swapped | Every detector and fee rule has a test that makes it fire; no silent default coordinates |
| 24 | Watchtower auto-deploys whatever lands on `main`; migrations by hand; CI runs no tests; UAT and prod share a droplet | Tests and golden suite gate every merge; migrations run in the deploy job; explicit deploys; separate UAT and prod |
| 25 | Hot-fixes patched into running containers (LATRA cargo filter lived only in the container for days) | Immutable images; config only via env and secrets; no shell patching |
| 26 | Dev settings default to the real remote database and Sentry | Config fails fast on missing values; local defaults never point at a shared environment |
| 27 | Apps hard-code the production host, Google keys and brand; no flavors; release builds signed with debug keys; placeholder bundle ids | Brand config per build, EAS profiles, server-proxied Maps, real signing from phase 0 |
| 28 | Admin with hand-written API clients, brand name in code, dead sidebar links, template screens in the production build | Generated client from the spec; brand from the API; pruned routes |
| 29 | `logger.error` emails the admins for every application error | Structured logs and metrics to Grafana; alert rules, not mail storms |
| 30 | One 1,400-line `trip_operations.py` orchestrating pricing, payment, LATRA and broadcasts | Bounded modules reacting to `trip_events`; `trips` knows nothing about money or regulators |
| 31 | Ratings support structured aspects on the backend but the apps send free text | Aspects are first-class in the API and rendered by the apps |
| 32 | Reverse geocoding on the booking request path (latency and cost on every request) | Addresses come from the quote; enrichment runs asynchronously |
| 33 | Postman collections and a Markdown file as the API documentation | The OpenAPI spec is the documentation, the server contract and the client |

One deliberate carry-forward, time-boxed: Songa's scoring function is used only as the v0 cost inside the new solver until the ETA cache exists (end of phase 1); the pilot runs on the ETA-based v1.

## Execution steps after approval

1. Create `RIDER & HAULER/DOC/` structure: `00-songa-system-scan.md` (Part 1 of this file, cleaned), `01-redesign-rider-hauler.md` (the design above, expanded into a proper document with diagrams), `02-dispatch-engine.md` (D5 expanded: saga, scoring, solver, config knobs, metrics), `reference/songa-vision/` (copies of `Songa-Proposal-Doc1/2/3` PDF + HTML and `ADRIAN/docs/*.md`, originals untouched), `adr/` (architecture decision records: 0001 Go, 0002 modular monolith + River + Redis fan-out, 0003 service abstraction, 0004 dispatch model, 0005 white-label brand-as-data), `README.md` index. Every document carries the D0 variables table and refers to "the Platform", never to a brand, except where Songa is cited as the reference system.
2. Create `RIDER & HAULER/.claude/settings.json` (permissions for the folder) and `RIDER & HAULER/CLAUDE.md` (project conventions: docs live in DOC, Songa is reference only, decisions table) so the project has its own Claude Code context; tell Kalen to open future sessions from that folder.
3. Rename `"DOC "` → `DOC` only if Kalen confirms (not done by me otherwise).
4. Save memory: new project location, conventions, decisions (Go, greenfield, service-first, RN, Uber-style dispatch).
5. Nothing is built yet; repo scaffolding is a separate approval.

## Working conventions (set by Kalen 2026-10-14)

- Working directory for this effort: `/Users/kalen/Desktop/ADRIAN/RIDER & HAULER/`.
- All docs and working notes go in its `DOC` subfolder. Currently empty.
- Gotcha: the folder was created as `"DOC "` with a trailing space. Quote the exact name or rename it before scripts reference it.
- Sibling context in `/Users/kalen/Desktop/ADRIAN/`: Songa proposal PDFs (Doc1 Platform, Doc2 Tracking, Doc3 Analysis), a `docs/` folder, `mixx-forge-ui` (design system source for the admin), and other client folders.

## Verification

Read-only scan; nothing to run. Cross-checks performed: app endpoint strings vs backend `urls.py` routes (all app-called paths exist on the backend; one dead driver-app path `/driver/trip-requests/<id>/reject` does not), Aug-2026 API doc vs current routes (doc still accurate, missing receipts and V3 catalogue), memory notes vs current git state (dates and commit ids match). Anything marked "check" above was not executed against a running backend.
