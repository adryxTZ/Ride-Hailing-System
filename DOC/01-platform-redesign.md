# 01 · Platform redesign — target architecture

> Target design for a **generic, white-label, multi-service mobility and logistics platform**. Written 2026-10-14 from the approved plan. Songa is the reference system being replaced (see `00-songa-system-scan.md`); "Rider & Hauler" is the working-folder name, not a product identifier. The booking and matching engine has its own document, `02-dispatch-engine.md`. Decisions are recorded as ADRs in `adr/`.

## Variables used in this document

| Variable | Meaning | Example value (first brand) |
|---|---|---|
| `PLATFORM_NAME` / `brand.name` | The customer-facing system name | Songa |
| `BRAND` | Brand code selecting config, theme, domains, catalogue | `songa` |
| `COUNTRY` | Country of operation; selects adapters | TZ |
| `CURRENCY` | ISO currency for all amounts | TZS |
| `REGULATOR` | Regulator adapter | LATRA CeTS |
| `GATEWAY` | Payment gateway adapter | Selcom |
| `SMS_PROVIDER` | OTP/SMS adapter | NextSMS |
| `ROUTING` | Routing/ETA provider | Google (phase 1), Valhalla (phase 2) |
| codename `platform` | Repo, binary and module codename; rename freely | — |

## How to read this

Sections 1–2 record what was decided and what the product must become. Section 3 (D0–D12) is the design: D0 generic/white-label foundation, D1 shape, D2 module layout, D3 service abstraction, D4 data model, D5 booking and dispatch (summary; full detail in `02-dispatch-engine.md`), D6 API, D7 React Native, D8 infra and delivery, D9 phases, D10 risks, D11 refinements adopted from an independent design review, D12 the Songa flaws this design removes (use it as a review checklist), D13 configurable branding and theming (full design in `03-branding-theming.md`).


## 1. Decisions (Kalen, 2026-10-14)

| Topic | Decision |
|---|---|
| Backend language | **Go**. C was considered; Uber uses Go/Java for services and C/C++ only in hot libraries (H3, routing), so Go + the C H3 library via `h3-go` is the Uber pattern. |
| Project | **Greenfield** new project "Rider & Hauler" at `/Users/kalen/Desktop/ADRIAN/RIDER & HAULER/`. Songa is the reference system, not a migration source. |
| Data | **Clean start.** Only reference data carried over: car types, cargo types, regions/districts, pricing config values. Old Songa DB kept as read-only archive. |
| Scope | **Multi-service, service-first apps.** Rider app opens on a service picker: Taxi, Cargo hauling, Recovery/towing, Water (clean delivery + waste/septic), more later. **Ship hauling first**, then taxi, then others. |
| Apps | **React Native** rider + driver apps. Next.js admin kept and repointed. |
| Deliverable now | **Full redesign document** in `RIDER & HAULER/DOC/`, plus a copy of the Part 1 scan as the first working note. |

## 2. Product vision (from the proposal documents in `reference/songa-vision/`)

Doc1 "Five revenue lines": fleet management (FleetOwner, DriverAssignment, owner wallet), cargo-weight pooling (weight/volume, TripLeg, remaining capacity), scheduling (`scheduled_for`, RRULE recurrence, pre-dispatch job), B2B corporate (CorporateAccount, credit terms, DeliveryJob, consolidated monthly invoice, proof of delivery, bulk CSV, ERP API), goods selling (vendor catalogue, 3-way ledger split). Doc2 "Songa Track": telematics SaaS (Tenant, Asset, Device, Geofence, AlertRule, Webhook, share links, hardware trackers, per-asset billing, PDPA consent). Doc3 audit: real lifecycle today skips pickup states; dispatch is broadcast-to-top-5 first-accept-wins; surge off; 10% commission; proposes driver paid subscription waiving commission, manual dispatcher assignment, leakage detector, back-to-back offers, zone/time-varying matching radius. None of the docs mention TRA/EFD; LATRA is required. Doc3 said passengers were out of scope then; Kalen's new direction adds Taxi as a service.

**Requirements the redesign must honour** (merged): service catalogue with per-service request fields, vehicle classes, lifecycle, pricing and LATRA mapping; explicit pickup states + server-measured loading/unloading; Redis/H3 matching with configurable radius per region/time; one deliberate dispatch model; manual assign (RBAC + audit); scheduled + recurring; weight/volume vs capacity; multi-leg (later); fleet owners (later); corporate (later); goods (later); loaders as priced add-on; brackets + waiting + floor + surge(clamped, locked at quote); fixed-route vs GPS-odometer billing; configurable commission, cash→debt, digital→auto-net; holds; hash-chained ledger generalised to many payers/payees; Selcom; invoices with VAT (+TIN later); fraud detectors that can fire; LATRA submit + fail-closed vehicle verification; PDPA; EN/SW; live H3 supply/demand; in-trip chat; push.

## 3. Design

### D0. Generic, white-label platform (Kalen: "the design should be generic, system name should be a variable")

The product is a **multi-service mobility and logistics platform**; "Songa" and "Rider & Hauler" are brands running on it, never identifiers in code.

- **Brand is data.** A `brands` table (single row at launch, `brand_id` column on tenant-scoped tables from day one): `name`, `short_name`, `legal_entity` (+ TIN/VAT registration), logos, colour tokens, support phone/email, domains (API, web, deep-link scheme), app-store ids, default locale, currency, timezone, country. Env `PLATFORM_BRAND=<code>` selects it for single-brand deployments.
- **Every user-facing string is templated**: SMS (`{{brand.name}}: Hi {{driver.first_name}}…`), emails, push, receipts/invoices (brand + legal entity block), WS messages, app strings. SMS sender ID per brand. No literal product name anywhere in `internal/`.
- **Country adapters behind interfaces**: `Regulator` (TZ: LATRA CeTS; others later), `PaymentGateway` (Selcom now; M-Pesa Daraja, Flutterwave later), `SMSProvider` (NextSMS; Africa's Talking, Twilio), `RoutingProvider` (Google; Valhalla), `ObjectStore` (DO Spaces; S3). Chosen per brand in config.
- **Money and locale generic**: amounts in minor units + ISO currency code; tax lines (VAT, levy) are configured rows, not hard-coded names; languages pluggable (EN/SW shipped); timezone per brand.
- **Service catalogue is per brand**, so one brand can be taxi-only and another hauling-only on the same code.
- **Tenancy**: one deployment per brand by default (simplest ops, hard isolation); `brand_id` everywhere makes shared-DB multi-brand possible later and feeds the telematics SaaS tenant model from the vision docs.
- **Apps are white-label builds** from one monorepo: `brands/<code>.json` (name, slug, bundle ids, icons, splash, colours, API URL, deep-link scheme) read by Expo `app.config.ts`; EAS build profile per brand; runtime theme/logo also served by `GET /v1/app/config` so a light rebrand needs no store release.
- **Admin** reads brand name, logo and colours from the API (today's Next.js has "Songa" hard-coded; that goes). Colours, logos and icons are chosen in the admin from curated palettes, a Color Hunt URL or custom values and published as a versioned theme (D13).
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

Key library choices (boring, maintained): `chi` router + `oapi-codegen` (server stubs + request validation from `backend/api/openapi.yaml`), `pgx/v5` + `sqlc`, `goose` migrations (embedded, run by `platform migrate` in the pipeline), **River** job queue on Postgres (transactional enqueue, retries, unique + periodic jobs; replaces Celery and beat), `coder/websocket`, `uber/h3-go` (wraps Uber's C H3), `go-redis`, `log/slog` JSON, OpenTelemetry (`otelhttp`, `otelpgx`), Sentry Go SDK, `santhosh-tekuri/jsonschema` for request schemas, `testcontainers-go` for integration tests. TypeScript client generated from the same OpenAPI with `openapi-typescript` + `openapi-fetch` + `openapi-react-query` for RN and admin.

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

**Expo** (prebuild + EAS Build + EAS Update for OTA). pnpm + Turborepo monorepo: `apps/rider`, `apps/driver`, `packages/api` (client generated with `openapi-typescript` + `openapi-fetch` + `openapi-react-query`), `packages/realtime` (reconnect, heartbeat, seq resume, offline queue), `packages/forms` (JSON-schema → RN form renderer for request schemas), `packages/ui` (Songa navy/green tokens, components), `packages/i18n` (EN/SW + error-code catalogue), `packages/maps` (react-native-maps wrapper, polylines, H3 helpers). Expo Router; TanStack Query for server state, Zustand for session + in-progress trip. Driver background location: `react-native-background-geolocation` (Transistorsoft, paid licence, the one that survives OEM power management) with Android foreground service; rider uses `expo-location`. Push: `expo-notifications` (FCM v1 + APNs); offers stay WS-first with a high-priority data push as wake-up when the socket is down. Service picker: `GET /v1/services` → tiles → schema-driven form → quote options → book. Driver app shows only services its vehicle is eligible for.

### D8. Infra and delivery

Before (Songa): nginx, uvicorn×4, kafka consumer, JVM gateway, celery worker, beat, flower, kafka, redis, pgbouncer, watchtower, admin ≈ 11 containers, several GB. After (launch): traefik, `platform` (all roles in one container), redis, admin, alloy ≈ 5 containers, ≈ 500 MB; managed Postgres external. Split `platform` into api,realtime ×2 + worker,scheduler ×1 by compose when traffic justifies. Image: distroless, ~25 MB. CI (GitHub Actions): golangci-lint → `go test` (testcontainers) → golden parity tests → build/push GHCR → deploy job runs `platform migrate` under an advisory lock, then `compose up -d --wait` and a smoke check (no watchtower). Environments local/uat/prod from one image via env. Secrets via SOPS or DO. Observability: OTel → Grafana Cloud free tier (metrics/logs/traces) + Sentry. Backups: managed PG PITR; Redis needs none.

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


### D13. Configurable branding and theming (Kalen, 2026-10-14: colours, logo and icons selectable from the admin, from palettes or custom)

Full design: `03-branding-theming.md`. Summary:
- **Theme is versioned data per brand** (`brand_themes`: palette light/dark, derived tonal scales and on-colours, typography from an allow-list, shape, assets for logo full/mark/on-dark, app icon, splash, favicon, map markers; `wcag_report`; DRAFT → PUBLISHED with reason, one-click rollback). `theme_presets` holds a curated gallery.
- **Three ways to choose colours in the admin**: pick a preset; **import from a Color Hunt URL** (the four hexes are parsed from `colorhunt.co/palette/<24 hex>`, no scraping); or custom hex per role with a live contrast badge. Roles are auto-assigned from a 4-colour palette (lightness → background/surface, chroma → accent, remainder → primary) and can be dragged; tonal scales, on-colours, semantic colours and dark mode are derived server-side.
- **Delivery**: `GET /v1/app/config` (ETag) carries the published theme; apps apply it through a `ThemeProvider` at start and on foreground; admin sets CSS variables from the same payload; receipts, emails and push read the theme at render time. Launcher icon, app name and native splash are generated from the same theme into `brands/<code>.json` + icon files by `platform brand export` and need an EAS build; the admin labels those "requires app release".
- **Guards**: publish blocked below WCAG 4.5:1 for body and button text or when primary and accent are indistinguishable; status colours never used for charts; SVG sanitised; theme versions immutable.
- **Presets** (19, each verified on colorhunt.co on 2026-10-14; links in the doc): Songa current identity (`#001F54`/`#00B384`), Deep Navy `0B2447 19376D 576CBC A5D7E8` (**recommended default**), Classic Navy `112D4E 3F72AF DBE2EF F9F7F7`, Royal Blue & Gold `191D88 1450A3 337CCF FFC436`, Blue & Mustard `0C356A 0174BE FFC436 FFF0CE`, Ocean Teal `071952 0B666A 35A29F 97FEED`, Harbor Blue `164863 427D9D 9BBEC8 DDF2FD`, Steel Blue `27374D 526D82 9DB2BF DDE6ED`, Marine & Coral `3D5A80 98C1D9 E0FBFC EE6C4D`, Slate & Teal `222831 393E46 00ADB5 EEEEEE`, Dark Teal `2C3333 2E4F4F 0E8388 CBE4DE`, Night Green `0F0F0F 232D3F 005B41 008170`, Deep Indigo `070F2B 1B1A55 535C91 9290C3`, Midnight & Coral `1A1A2E 16213E 0F3460 E94560`, Indigo & Apricot `2D3250 424769 7077A1 F6B17A`, Navy & Sunset `2D4059 EA5455 F07B3F FFD460`, Teal & Sunrise `1D5B79 468B97 EF6262 F3AA60`, Safety Orange `FF6000 FFA559 FFE6C7 454545` (**driver-app accent option**), Forest & Sand `3A4D39 4F6F52 739072 ECE3CE`, Racing Green & Gold `004225 F5F5DC FFB000 FFCF9D`.
- **Phasing**: theme model + app config + ThemeProvider + first preset in Phase 0 (needed for brand-as-data anyway); full Branding page, import, assets, preview, publish/rollback and build-asset export in Phase 2. Adds ≈ 3 engineer-weeks (2 backend/admin, 1 apps).
- **Songa flaw removed** (adds to D12): brand name, colours and logo hard-coded in two Flutter apps, the admin and SMS templates → all theme data, changeable without a release.

## 4. Open decisions for Kalen

1. ~~Codename~~ decided 2026-10-14: git repo "Hauler & Rider" (`hauler-rider`), internal codename `nexa` for the binary and app projects; the display name, colours and icons remain brand data.
2. ~~DOC folder rename~~ done 2026-10-14.
3. **Driver background location**: Transistorsoft `react-native-background-geolocation` (paid licence, robust on OEM-throttled Android) is recommended; `expo-location` is the free fallback.
4. **Separate UAT and prod droplets** (recommended) versus one droplet with two stacks.
5. **Admin into the apps monorepo** (recommended) versus keeping its own repo.
6. **Money policy values** to enter as `fee_policies` data: VAT 16 % vs 18 %, VAT base, LATRA levy rate, booking fee, who funds cancellation compensation.
7. ~~Repo scaffolding~~ started 2026-10-14 (see `notes/2026-10-14-phase0-progress.md`).
8. **Theme defaults for the first brand**: keep the current Songa identity as the published theme, or adopt the recommended `deep-navy` preset (and `safety-orange` accent for the driver app)? See `03-branding-theming.md` §6.
