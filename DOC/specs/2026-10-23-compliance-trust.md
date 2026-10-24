# Spec: compliance and trust (Phase C): regulator, document expiry, fraud v1, PDPA

Written 2026-10-23 by the spec-writer agent (condensed; defaults in §9 apply until the owner answers). Builds on `2026-10-21-operating-areas.md` (countries carry `regulator`), `2026-10-21-debt-holds-payins.md` (holds) and `2026-10-21-fares-payouts-receipts.md` (completing transaction, receipts). Closes D12 #23 (detectors that can never fire) and keeps #30 (trips knows nothing about regulators). `internal/compliance`, `internal/fraud`, `internal/telemetry` and `internal/jobs` are empty today. Nothing persists GPS traces (presence is Redis with a 120 s staleness window), `documents.expires_on` exists but nothing reads it, `users.pdpa_consent_version` exists but nothing writes it, and dispatch can offer a rider's trip to the rider's own driver account.

**Build slices, in order.** Each one ships alone with its own migration, tests and factory review.
- **S1, Regulator** (fake adapter + LATRA CeTS skeleton): migration `00031`.
- **S2, Document expiry** (reminders, automatic hold, renewals): migration `00032`.
- **S3, Fraud v1** (fake GPS, self-booking, cancellation abuse, review queue): migration `00033`.
- **S4, PDPA** (consent versions, retention job, export/delete requests): migration `00034`.

## 1. Goal
**Regulator.** In every country whose brand entry names a regulator (`latra` for TZ, `none` for KE), the Platform does four things on its own:
- registers each driver and vehicle with the regulator when staff approve them;
- submits every completed ride once;
- sends the daily report;
- retries on a fixed ladder until the regulator confirms, storing the reference it returns.

Staff see what is pending, stuck or accepted. A per-country **fail-closed** switch stops unregistered drivers from going online. Credentials are not available yet, so a scripted `fake` adapter carries dev and CI, and the `cets` adapter is compiled and tested against a stub server.

**Documents.** Licence, insurance and inspection expiry dates are tracked. Drivers get push and SMS reminders 30, 7 and 1 days before a date. The day after a document expires, the driver is on a `DOCUMENT` hold. A renewal upload goes back to review, and approving it lifts the hold.

**Fraud.** Three detectors fire, each proven by a test: fake GPS, self-booking and cancellation abuse. They raise flags in a staff review queue and punish nobody automatically. A driver is never offered their own ride.

**Privacy.** Riders and drivers accept a versioned privacy notice at sign-up, and again when it changes. A nightly retention job deletes or strips data past its period. A user can ask for an export or for deletion, and staff handle the request within the due date.

## 2. Scope
**In, S1:**
- Port `regulator.Regulator` with adapters `fake` (scripted) and `cets` (RS256 JWT, envelope, payloads, code table), plus mode `pending` (queue only).
- A `regulator_submissions` outbox: DRIVER, VEHICLE, TRIP and DAILY_REPORT rows, ladder retry, stuck flag.
- Triggers:
  - driver approval and vehicle re-approval enqueue in the approving transaction;
  - a completed trip enqueues through a `CompletedTx` hook, only when the country has a regulator and the service's `compliance.<regulator>.submit_trips` is true;
  - the daily report goes out after 00:30 country time.
- 5016 (zero trips) closes the day locally.
- Licence validation from the country's `licence_pattern`; licence class captured.
- Per-country settings `enabled` and `fail_closed` (audited).
- JWKS discovery routes.
- Admin "Regulator" page and badges on the trip and driver pages.

**In, S2:**
- Expiry required for LICENCE, INSURANCE and INSPECTION (new optional vehicle kind).
- Expiry state on every document.
- Reminders at 30, 7 and 1 days (push + SMS, once each).
- A `DOCUMENT` hold placed and lifted by the system.
- Renewal uploads for approved drivers: the pending renewal sits beside the current document, and approval promotes it.
- Staff can correct an expiry date.
- Admin "Documents" page (expiring, expired, unknown, renewals).

**In, S3:**
- GPS check on every ping in Redis: impossible speed, jump, and the Android mock flag.
- Install id (`X-Install-Id`) recorded at sign-in.
- Self-booking signals at completion: shared install, or a passenger/receiver phone equal to the driver's.
- Dispatch hard-excludes `driver_id = rider_id`.
- Daily cancellation scan (driver cancel rate, driver no-show rate, rider cancel rate).
- `fraud_flags` with dedupe, and the admin "Fraud" queue (dismiss / confirm with a note, links to the existing suspend, hold and block actions).

**In, S4:**
- Consent documents as brand data (per app, versioned), append-only `user_consents`, consent gate on booking, going online and onboarding submit, and a consent screen in both apps.
- Retention registry and nightly job.
- `privacy_requests` (EXPORT, DELETE), a staff queue, a JSON export behind a 24 h link, and deletion as anonymisation with blockers.
- Admin "Privacy" page.

**Out:**
- LATRA vehicle-verification API (no contract yet).
- Bulk CeTS endpoints.
- Automatic suspension or blocking from fraud flags.
- Ride-pair collusion, promo abuse, device root/jailbreak checks.
- Persisting GPS traces (no trace table is added; future ones must register a retention rule).
- Editing consent texts or retention periods in admin (brand file only).
- DPO / regulator registration paperwork.
- Fiscal (TRA) receipts.
- The app translation layer: app copy ships in EN, with SW listed below for the deferred i18n slice; server messages are localised now.

## 3. Contract
All new error codes are added to `backend/i18n/{en,sw}.json`. All routes are declared with security in `backend/api/openapi.yaml`. New permissions: `compliance.manage` (S1, S2), `fraud.review` (S3), `privacy.handle` (S4). The migrations add them to the default Operations role and to Super Admin.

**S1, regulator**
- Schemas:
  - `RegulatorStatus {country, regulator, adapter: fake|cets|pending, configured, enabled, fail_closed, counts {pending, stuck, accepted_today, skipped_today}, last_accepted_at?, last_error?}`
  - `RegulatorSubmission {id, kind: DRIVER|VEHICLE|TRIP|DAILY_REPORT, subject {type, id, label}, status: PENDING|ACCEPTED|SKIPPED, attempts, next_attempt_at, last_code?, last_error?, reference?, accepted_at?, stuck}`
  - `RegulatorBadge {status, reference?, last_error?}`
- `GET /admin/compliance/regulator?country=` → `RegulatorStatus[]` (`finance.view` or `compliance.manage`).
- `PATCH /admin/compliance/regulator/{country}` `{enabled?, fail_closed?, reason}` (`compliance.manage`). Errors:
  - `REGULATOR_NONE` 409: the country has none;
  - `REGULATOR_NOT_CONFIGURED` 409: turning on `fail_closed` while the adapter is `pending`;
  - `REASON_REQUIRED`.
- `GET /admin/compliance/submissions?country&kind&status&stuck&q&limit&cursor` → page of `RegulatorSubmission`.
- `POST /admin/compliance/submissions/{id}/retry {reason}` → due now. Error `SUBMISSION_NOT_PENDING` 409.
- `AdminTrip.regulator?: RegulatorBadge`. `DriverDetail.regulator {driver?: RegulatorBadge, vehicle?: RegulatorBadge}`.
- `PUT /driver/status` (online): 409 `REGULATOR_REGISTRATION_PENDING` when `fail_closed` is on and the driver or vehicle is not ACCEPTED. `DriverStatus.regulator_pending: boolean`.
- `DriverProfileInput.licence_class` (enum from the country). Field errors `licence_number: FORMAT`, `licence_class: REQUIRED|UNKNOWN`. `Onboarding.licence_classes[]` and `Onboarding.licence_hint {en, sw}`.
- Public, no auth:
  - `GET /.well-known/tz-e-ticketing-server` → `{jwks_uri}`;
  - `GET /v1/regulator/jwks` → JWKS.
  - Both `Cache-Control: public, max-age=300` with ETag. 503 `REGULATOR_NOT_CONFIGURED` without a key. Mounted only when a country uses `latra` with adapter `cets`.

**S2, documents**
- `Document` gains:
  - `expiry_state: VALID|EXPIRING|EXPIRED|UNKNOWN|NOT_TRACKED`;
  - `days_left?`;
  - `renewal?: Document` (the pending or rejected renewal).
- `DocumentInput.expires_on` is required for the expiring kinds. Field errors `expires_on: REQUIRED|FORMAT|PAST|TOO_FAR`.
- `GET /driver/documents` → `{documents: Document[], alert?: {kind, expires_on, days_left, state}}`.
- `POST /driver/documents/renewals {kind, file_key, content_type, expires_on}` → 201 `Document`. A second call replaces a pending renewal. Upload URLs come from the existing `POST /driver/onboarding/uploads`. Errors:
  - `UNKNOWN_DOCUMENT`;
  - `UPLOAD_MISSING`;
  - `DOCUMENT_NOT_RENEWABLE` 409 (kind without expiry);
  - `DRIVER_NOT_APPROVED` 409 (onboarding drivers use onboarding).
- `Hold.kind` += `DOCUMENT`. `Hold.cause` += `DOCUMENT_EXPIRED`. `Hold.details {documents: [kind]}`. A held driver going online gets the existing `DRIVER_ON_HOLD`.
- `GET /admin/compliance/documents?country&state=EXPIRING|EXPIRED|UNKNOWN|RENEWAL_PENDING&within_days=30&limit&cursor` → `ExpiringDocument {document, driver {id, name, phone}, plate?, days_left?, reminders_sent: [30|7|1]}`.
- `PATCH /admin/drivers/{id}/documents/{docId}/expiry {expires_on, reason}` (`drivers.review`, audited). The response re-evaluates the hold.
- The existing `POST /admin/drivers/{id}/documents/{docId}/review` approves or rejects renewals. Approving one promotes it.

**S3, fraud**
- `LocationSample.mocked?: boolean` and `DriverStatusInput.mocked?: boolean` (from expo-location on Android).
- Header `X-Install-Id` (UUID v4, ignored if malformed) on `/auth/otp/verify` and `/auth/refresh`. `DeviceInput.install_id?`.
- `FraudFlag`:
  - `{id, rule: GPS_SPOOF|SELF_BOOKING|DRIVER_CANCEL_RATE|DRIVER_NO_SHOW_RATE|RIDER_CANCEL_RATE, severity: LOW|MEDIUM|HIGH}`
  - `subject {kind: DRIVER|RIDER, id, name?, phone}`
  - `{trip_id?, evidence (object, rule-specific), hits, status: OPEN|DISMISSED|CONFIRMED}`
  - `{opened_at, last_seen_at, reviewed_by?, reviewed_at?, review_note?}`
- `GET /admin/fraud/flags?status=OPEN&rule&subject_kind&country&limit&cursor`.
- `GET /admin/fraud/flags/{id}`.
- `POST /admin/fraud/flags/{id}/review {decision: DISMISS|CONFIRM, note}` (`fraud.review`, audited). Errors `FLAG_ALREADY_REVIEWED` 409, `REASON_REQUIRED`.
- `AdminOverview.attention.fraud_open`. `DriverDetail.flags_open` and `AdminRider.flags_open`.

**S4, privacy**
- `GET /legal/consent?app=RIDER|DRIVER` (public, ETag, `Accept-Language`) → `ConsentDocument {app, version, title, summary, url, published_at}`.
- `TokenPair.consent_required: boolean`. `Me.consent {app, current_version, accepted_version|null, required}`.
- `POST /me/consents {app, version}` → 204. Error `CONSENT_VERSION_STALE` 409 `{current_version}`.
- 403 `CONSENT_REQUIRED` from `POST /trips`, `PUT /driver/status` (online) and `POST /driver/onboarding/submit`.
- `PrivacyRequest {id, kind: EXPORT|DELETE, status: REQUESTED|IN_PROGRESS|READY|DONE|REJECTED|CANCELLED, created_at, due_at, reason?, download_expires_at?}`.
- `POST /me/privacy-requests {kind}` → 201. Errors `PRIVACY_REQUEST_OPEN` 409 `{request_id}`, `PRIVACY_RATE_LIMITED` 429.
- `GET /me/privacy-requests`.
- `POST /me/privacy-requests/{id}/cancel`: only while REQUESTED, else `PRIVACY_REQUEST_NOT_OPEN`.
- `GET /me/privacy-requests/{id}/download` → `{url, expires_at}`. Errors `EXPORT_NOT_READY` 409, `EXPORT_EXPIRED` 410.
- `GET /admin/privacy/requests?status&kind&country&limit&cursor`.
- `GET /admin/privacy/requests/{id}` (with `blockers[]` preview).
- `POST /admin/privacy/requests/{id}/approve {reason}`:
  - EXPORT goes IN_PROGRESS, then READY;
  - DELETE goes DONE, or 409 `DELETE_BLOCKED {blockers: [ACTIVE_TRIP|DRIVER_DEBT|DRIVER_BALANCE|OPEN_PAYOUT|RIDER_DUES|OWED_TO_RIDER]}`.
- `POST /admin/privacy/requests/{id}/reject {reason}`.
- `GET /admin/privacy/retention` → rules `{name, days, last_run_at, rows_last_run, error?}`.
- All privacy admin routes need `privacy.handle`.

## 4. Data
**`00031_regulator.sql`**
- `countries` gains:
  - `licence_pattern text` (TZ `^\d{10}$`);
  - `licence_classes text[] NOT NULL DEFAULT '{}'`.
  - Both are loaded from the brand file; validation fails fast on a bad regex.
- `driver_profiles.licence_class text`.
- `regulator_settings (brand_id, country_code, enabled bool DEFAULT false, fail_closed bool DEFAULT false, updated_by, updated_at, PK (brand_id, country_code), FK countries)`.
- `regulator_submissions`:
  - `id uuid PK, brand_id, country_code, regulator text`
  - `kind text CHECK (DRIVER|VEHICLE|TRIP|DAILY_REPORT)`
  - `subject_key text` (uuid or `YYYY-MM-DD`)
  - `driver_id, vehicle_id, trip_id` (nullable FKs)
  - `status text CHECK (PENDING|ACCEPTED|SKIPPED) DEFAULT 'PENDING'`
  - `attempts int DEFAULT 0, next_attempt_at timestamptz DEFAULT now(), last_attempt_at, last_code, last_error`
  - `reference, payload jsonb` (last sent), `accepted_at, stuck_at, created_at, updated_at`
  - `UNIQUE (brand_id, regulator, kind, subject_key)`
  - Indexes: `regulator_due_idx (next_attempt_at) WHERE status='PENDING'`, `(brand_id, country_code, kind, status, created_at DESC)`, `(trip_id)`.
- Seed: `seed/catalog/parcel.json` sets `compliance.latra.submit_trips=false` (see §9 Q3).
- Env:
  - `NEXA_REGULATOR_LATRA_ADAPTER=fake|cets|pending`. Required when an ACTIVE country uses `latra`; startup fails if unset.
  - `NEXA_REGULATOR_LATRA_BASE_URL`, `_JWT_ISSUER`, `_JWT_AUDIENCE` (default `cets`), `_JWT_KID`, `_JWT_KEY_FILE`, `_JWKS_URI`, `_TIMEOUT` (15s). All are required with `cets`, and startup fails if one is missing.
  - `NEXA_FAKE_REGULATOR_MODE=ok|reject|down` (default ok).
- `nexa keys regulator latra --out <file>` writes an RSA-2048 PEM.

**`00032_document_expiry.sql`**
- `documents` gains `renewal_of uuid REFERENCES documents(id)`, `archived_at`, `purged_at`. `file_key` becomes nullable (purged files).
- Drop `documents_current_idx`. Create:
  - `documents_current_uq (owner_kind, owner_id, kind) WHERE archived_at IS NULL AND renewal_of IS NULL`;
  - `documents_renewal_uq` (same columns, `renewal_of IS NOT NULL`);
  - `documents_expiry_idx (brand_id, expires_on) WHERE archived_at IS NULL AND renewal_of IS NULL AND expires_on IS NOT NULL`.
- Backfill LICENCE `expires_on` from `driver_profiles.licence_expiry`. That column is no longer written and is dropped in a later cleanup.
- `document_reminders (document_id, threshold_days, sent_at, PK (document_id, threshold_days))`.
- `driver_holds`: kind CHECK += `DOCUMENT`, cause CHECK += `DOCUMENT_EXPIRED`, `details jsonb NOT NULL DEFAULT '{}'`.

**`00033_fraud.sql`**
- `user_installs (user_id, install_id uuid, app, first_seen_at, last_seen_at, PK (user_id, install_id))`, index `(install_id)`.
- `devices.install_id uuid`.
- `fraud_flags`:
  - `id, brand_id, country_code, rule, severity, subject_kind, subject_id, trip_id`
  - `evidence jsonb, hits int DEFAULT 1, dedupe_key text`
  - `status DEFAULT 'OPEN', opened_at, last_seen_at, reviewed_by, reviewed_at, review_note`
  - `UNIQUE (brand_id, dedupe_key)`
  - Indexes `(brand_id, status, opened_at DESC)`, `(subject_kind, subject_id)`.
- The cancel scan uses `trips_brand_requested_idx` (00025); EXPLAIN is recorded.

**`00034_privacy.sql`**
- `consent_documents (brand_id, app, version int, title jsonb, summary jsonb, url jsonb, published_at, PK (brand_id, app, version))`, synced from the brand file at startup; bumps `cfg:version` scope `legal`.
- `user_consents (id, user_id, app, version, accepted_at, install_id, ip inet, user_agent, UNIQUE (user_id, app, version))` with an append-only trigger.
- Drop the unused `users.pdpa_consent_version`.
- `privacy_requests`:
  - `id, brand_id, user_id, kind, status, reason, created_at, due_at`
  - `handled_by, handled_at, export_key, export_expires_at, blockers jsonb`
  - One open per user and kind (partial unique), index `(brand_id, status, due_at)`.
- `retention_runs (id, rule, run_date, started_at, finished_at, rows int, error, UNIQUE (rule, run_date))`.

**Brand file** (`backend/brands/<code>.json`):
- `countries[].licence_pattern`, `countries[].licence_classes`;
- `legal.consent[] {app, version, title{en, sw}, summary{en, sw}, url{en, sw}, published_at}`;
- `privacy.request_due_days` (30);
- `privacy.retention` (days, §5 table).
- Validation fails fast.

**Redis keys:**
- `fraud:gps:{brand}:{driver}`: strike counter, TTL 10 min.
- `fraud:gps:flagged:{brand}:{driver}:{yyyymmdd}`: SETNX, TTL 26 h, keeps Postgres off the ping path.
- `consent:{brand}:{user}:{app}`: accepted version, TTL 24 h, written on accept; a miss reads Postgres.
- `privacy:rl:{brand}:{user}`: 3 requests/day.

No fee-policy fields change.

## 5. Behaviour
**S1, regulator**
- **Port** (`internal/providers/regulator/regulator.go`): `type Regulator interface { Name() string; RegisterDriver(ctx, DriverRecord) (Ack, error); RegisterVehicle(ctx, VehicleRecord) (Ack, error); SubmitTrip(ctx, TripRecord) (Ack, error); SubmitDailyReport(ctx, DailyReport) (Ack, error) }`.
- **Errors.** `ErrNotConfigured`, `ErrUnavailable`, and `*Rejection{Code, Message, Class}` with Class `RETRY|FIX_DATA|NOT_NEEDED|PREREQUISITE`. Records are typed, in Platform terms (GeoJSON points, `decimal`, UTC times).
- **The `cets` adapter does all formatting at the edge:**
  - local `YYYY-MM-DD HH:MM:SS` in the country timezone;
  - coordinates `"lat,lng"`, confined to `cets/format.go`;
  - plate upper-cased without spaces;
  - `vehicleTypeEnum` from `vehicle_classes.regulator_type` (CAR|MOTORCYCLE|TRICYCLE; no name guessing);
  - `totalPaidAmount` as an integer.
- **CeTS paths:** `rideVehicle/submitVehicle`, `rideDriver/submitDriver`, `rideHailing/submitTrip`, `rideReport/submitReport`.
- **CeTS success** is `statusCode 5000` only. A 2xx with another code is a rejection.
- **CeTS code table** (`cets/codes.go`):
  - 5016 on a report → NOT_NEEDED.
  - 5018 and 5019 (undocumented): PREREQUISITE (driver and vehicle are re-registered before the next attempt), with the raw body kept. To be confirmed on the CeTS test server.
  - Any other code → RETRY.
- **JWT** (`cets/jwt.go`): RS256 with header `kid`; claims `iss=sub=issuer`, `aud`, `iat`, `exp=+300 s`, `jti`. A fresh token per request.
- **Fake adapter:**
  - mode `ok` returns reference `FAKE-<8 chars>`; mode `down` → `ErrUnavailable`; mode `reject` → 5001.
  - Plate prefix `TFAIL` → 5001; licence `9999999999` → 5001; plate prefix `TDOWN` → unavailable.
  - `Script(kind, outcomes...)` queues outcomes for Go tests.
- **Enqueue** (`compliance.EnqueueTx(tx, …)`): upsert by `(regulator, kind, subject_key)`. An existing ACCEPTED or PENDING row is reset to PENDING with `attempts=0` and due now (details changed). Callers:
  - `fleet.Approve` and vehicle re-approval, through a new `fleet.OnApprovedTx` hook;
  - trips `OnCompletedTx` (a pure `ShouldSubmit(country, serviceCompliance)` decides);
  - the daily report job.
- **Rows are always queued** for a regulated country, whatever `enabled` says. `enabled=false` or adapter `pending` means the sweeper sends nothing and counts no attempt. The backlog drains when sending is turned on.
- **Sweeper** `compliance.regulator.sweep`:
  - Runs every `RegulatorSweepInterval=1m` and is the only clock (Songa lesson).
  - Claims up to `RegulatorSweepBatch=200` due rows with `FOR UPDATE SKIP LOCKED`, setting `next_attempt_at=now+ClaimLease(2m)`.
  - Sends with `SendConcurrency=4`, each call bounded by the adapter timeout.
  - TRIP rows wait until their DRIVER and VEHICLE rows are ACCEPTED, and the sweeper makes those due.
  - A record is validated locally before sending (e.g. licence does not match `licence_pattern`). A failure records `last_code=LOCAL_INVALID` without a network call.
  - Each payload is built at send time from one query (trip + stops + driver + vehicle + passenger + rider→driver rating, 0 if none + fare split `driver_earnings`).
  - A passenger is the "who is riding" contact when set, else the rider.
- **Outcome** (`ladder.go`, pure `NextAttempt(attempts int, now time.Time) time.Time`):
  - ACCEPTED stores the reference (`latraReferenceNumber`) and the payload.
  - NOT_NEEDED → SKIPPED.
  - Anything else: `attempts+1`, next due from `RetryLadder=[1m, 5m, 30m, 3h, 24h]` (the last rung repeats).
  - At `StuckAfterAttempts=5`, set `stuck_at` and log one ERROR (alert rule). Nothing is ever abandoned.
  - A driver profile or vehicle edit re-enqueues (due now).
- **Daily report** `compliance.regulator.daily`:
  - Hourly. Acts once per regulated country day at or after `DailyReportAt=00:30` local.
  - Totals: COMPLETED trips with `completed_at` in the local day, count and sum of `final_fare` (TRIP-eligible services only).
  - Zero trips → SKIPPED without a call.
  - Earlier PENDING days are retried by the same ladder.
  - A day already sent is never recomputed.
- **Fail-closed:** `fleet.Eligibility` asks the `RegulatorGate` port. When `fail_closed` is on and the country has a regulator, the driver and current vehicle rows must be ACCEPTED, otherwise `REGULATOR_REGISTRATION_PENDING`. The gate result is cached in-process for 60 s per driver. A ride is never blocked by trip submission.
- **Settings and retry** changes by staff are audited (`regulator.settings`, `regulator.retry`).

**S2, documents**
- Expiring kinds: `ExpiringKinds = {LICENCE, INSURANCE, INSPECTION}`. INSPECTION ("Vehicle inspection sticker" / "Stika ya ukaguzi wa gari") is optional at sign-up (not in `missing`) but tracked when present.
- Upload rules: `expires_on` must be today or later, and at most `MaxExpiryYears=10` ahead.
- **Pure state** `ExpiryState(expiresOn *civil.Date, today civil.Date) State`, where today is the country's local date:
  - nil → UNKNOWN (never holds; listed for staff);
  - `days_left < 0` → EXPIRED (a document is valid through its expiry date);
  - `days_left ≤ 30` → EXPIRING;
  - otherwise VALID.
  - Kinds without expiry → NOT_TRACKED.
- **Sweep** `compliance.documents.sweep`, every `DocumentSweepInterval=1h` (`RunOnStart`):
  - Reminders: per current document (no `archived_at`, no `renewal_of`), `ReminderDue(daysLeft, sent []int)` returns the tightest threshold in `ReminderDays=[30, 7, 1]` that is ≥ days_left and not yet sent. It sends at most one per run, only at or after `ReminderHour=08:00` local.
    - Push (DRIVER app) plus SMS through `sms.Sender` with brand-templated text.
    - The `document_reminders` row is inserted before sending (at most once).
    - A pending renewal suppresses reminders.
  - Holds: if any of the driver's current documents (driver-owned, or owned by their current vehicle) is EXPIRED, `holds.PlaceSystem(tx, driver, DOCUMENT, DOCUMENT_EXPIRED, details{documents})` runs, then Enforce (offline unless on a trip; the trip-ended hook enforces again). Otherwise `holds.ReleaseSystem(tx, driver, DOCUMENT)`. The same `EvaluateDocuments(driverID)` runs after renewal approval and after a staff expiry edit.
  - The audit actor is the system.
- **Renewal:**
  - The new row has `renewal_of=current.id`, status PENDING. Re-upload replaces a pending renewal.
  - Staff approving it, in one transaction: old row `archived_at=now`, new row `renewal_of=NULL`, APPROVED, then `EvaluateDocuments`.
  - Rejection keeps the current document and tells the driver the reason.
  - A renewed LICENCE or a vehicle document re-enqueues the regulator registration (S1).

**S3, fraud** (no automatic punishment; staff act through existing suspend, hold and block)
- **Self-match guard:** `dispatch` drops candidates whose user id equals `trip.rider_id`. Covered by a test; no flag.
- **GPS** (`fraud/gps.go`, pure `CheckPing(prev, cur Ping) Strike`):
  - `presence.Update` reads the previous `lat, lng, at` in the same pipeline (one HMGET; Postgres is never touched).
  - Ignored: a gap `< MinGapS=2`, or a distance `< MinJumpM=500`.
  - Strike when the implied speed is `> MaxPlausibleSpeedMps=55` (198 km/h).
  - `mocked=true` is a strike that flags at once (severity HIGH).
  - Counter `INCR` with TTL `GPSStrikeWindow=10m`. At `GPSStrikesToFlag=3`, and the per-day SETNX is free, River job `fraud.gps_flag` is enqueued with the last three ping pairs as evidence.
  - Severity MEDIUM. Dedupe `GPS_SPOOF:{driver}:{local date}`; repeats increment `hits`.
  - The ping itself is still accepted.
- **Self-booking:** River job `fraud.trip_check` is enqueued in `OnCompletedTx` (unique per trip). Pure `SelfBookingSignals(TripParties) []Signal`:
  - `SHARED_INSTALL`: the rider and driver share any `install_id` seen in the last `InstallLookback=90d`; HIGH.
  - `PASSENGER_IS_DRIVER`: the pickup contact or parcel receiver phone equals the driver's phone; MEDIUM.
  - Dedupe `SELF_BOOKING:{trip}`.
- **Cancellation abuse:** `fraud.cancel_scan` runs hourly and acts once per country day after `CancelScanAt=03:00` local. One grouped query over `CancelWindow=7d`:
  - `DRIVER_CANCEL_RATE`: driver cancels after accept, excluding no-show, divided by accepted trips. Flag when ≥ `DriverCancelRateFlag=0.30` with at least `DriverCancelMinSample=10` accepted.
  - `DRIVER_NO_SHOW_RATE`: no-show cancels divided by arrivals at pickup. Flag when ≥ `NoShowRateFlag=0.30` with at least 10 arrivals.
  - `RIDER_CANCEL_RATE`: rider cancels after a driver was assigned, divided by assigned trips. Flag when ≥ `RiderCancelRateFlag=0.40` with at least `RiderCancelMinSample=10`.
  - Evidence holds the counts, rate and trip ids. Dedupe `{rule}:{subject}:{ISO week}`.
- **Review:** DISMISS or CONFIRM with a note (audited `fraud.review`). A reviewed flag is immutable. A new occurrence after DISMISS opens a new flag in the next dedupe period.

**S4, privacy**
- **Consent:**
  - The current version per app is cached in-process and keyed by `cfg:version`. `required` means `accepted_version < current_version`.
  - The gate middleware `requireConsent(app)` on the three routes reads Redis and falls back to Postgres.
  - Accepting inserts `user_consents` (ip, user agent, install id) and sets the Redis key.
  - A stale version → `CONSENT_VERSION_STALE`.
  - Declining in the app signs out. Nothing is stored and the account stays.
  - A new version in the brand file makes the next app start show the "updated" screen.
- **Retention** (`privacy/retention.go`): a registry of `Rule{Name, Days, Run func(ctx, cutoff) (int, error)}`. Job `privacy.retention` runs hourly and acts once per day after `RetentionAt=02:00` brand time (`retention_runs` unique). It works in batches of `RetentionBatch=1000` and records the rows touched. Defaults (brand file, days):

| Rule | Default | Action |
|---|---|---|
| `otp_codes` | 1 after expiry | delete |
| `sessions` (refresh tokens, sessions) | 30 after expiry or revoke | delete |
| `trip_messages` | 180 after trip end | delete |
| `share_tokens` | 1 after trip end | set NULL |
| `gps_evidence` | 90 | strip coordinates from `fraud_flags.evidence` |
| `fraud_flags_dismissed` | 365 after review | delete |
| `devices_installs` | 365 unseen | delete |
| `regulator_payloads` | 90 after ACCEPTED | strip personal fields from `payload` (reference kept) |
| `kyc_rejected` | 180 after last activity (REJECTED or ONBOARDING) | delete files, `purged_at` |
| `kyc_closed` | 730 after account deletion | delete files, `purged_at` |
| `documents_archived` | 730 after archive | delete files, `purged_at` |
| `privacy_exports` | 7 | delete files |

  Trips, receipts, ledger, audit and consents are kept (tax and legal records; no deletion in v1). Live GPS exists only in Redis with TTLs.
- **Requests:**
  - `due_at = created + request_due_days`. One open request per kind. A Redis rate limit applies.
  - Staff approve EXPORT → River `privacy.export` builds one JSON file (profile, consents, places, trusted contacts, trips with stops and fares, payments, receipts list, ratings given/received, messages sent, driver documents metadata, holds, payouts). It is stored at `exports/<user>/<id>.json` and the request goes READY with `download_expires_at = now + ExportLinkTTL (24h)`. An SMS tells the user. The download returns a signed 15-minute storage URL while the request is READY.
  - Staff approve DELETE → blockers check (any → 409 with the list) → one transaction:
    - `users`: name and email nulled, `phone = 'deleted:' || id`, `status=DELETED`, `deleted_at`;
    - photo file deleted;
    - places, trusted contacts, devices, installs, sessions and refresh tokens deleted;
    - sent message bodies set to `[removed]`;
    - the driver is offline; `kyc_closed` retention starts;
    - audit `privacy.delete`.
  - The phone can sign up again as a new account.
  - Overdue requests (past `due_at`) are highlighted in admin and counted in Overview attention.

## 6. Apps and admin
**Driver app** (`@platform/ui`, theme tokens, eased 240–280 ms entrances, `ScalePressable`, no springs):
- S1:
  - `ProfileFormScreen` gets a licence class picker and a licence format hint.
  - Home shows `RegulatorPendingBanner` when `regulator_pending`.
- S2:
  - `screens/account/DocumentsScreen.tsx` lists each document with an expiry chip and a Renew button that opens `RenewDocumentSheet` (photo → date → send).
  - Home `DocumentAlertBanner`, plus the existing hold panel copy for `DOCUMENT_EXPIRED`.
  - Hooks and services: `hooks/useDocuments.ts`, `services/documents.ts`, `models/document.ts`.
- S3: the location service sends `mocked` from expo-location.
- S3: `services/install.ts` creates and keeps an install UUID in SecureStore and sends `X-Install-Id`.
- S4 (both apps): `screens/auth/ConsentScreen.tsx` after OTP when `consent_required`, and on start when `Me.consent.required`. Account → `PrivacyScreen.tsx` (download my data, delete my account, request status).

**Rider app:** S3 install id; S4 consent and privacy screens, as for drivers.

**Admin:** new collapsible nav group "Trust" with:
- `/compliance/regulator` (`features/compliance/RegulatorScreen.tsx`): country cards with switches (reason dialog), a submissions data table with filters and Retry.
- `/compliance/documents` (`DocumentsScreen.tsx`): tabs Expiring, Expired, Unknown, Renewals; editing an expiry date asks for a reason.
- `/fraud` (`features/fraud/FraudScreen.tsx` + `FlagDrawer.tsx`): the queue table and an evidence drawer. GPS evidence draws ping pairs on the map. Dismiss and Confirm buttons, plus links to Suspend, Hold or Block.
- `/privacy` (`features/privacy/PrivacyScreen.tsx`): a requests table with due-date colouring, Approve/Reject with a reason, blockers shown, and a Retention card.

Trip drawer and driver page show the regulator badge. Overview attention gets "Stuck regulator submissions", "Expired documents", "Open fraud flags" and "Privacy requests due".

**Copy (EN / SW):**
- `REGULATOR_REGISTRATION_PENDING`: "Your registration with the transport regulator is still in progress. Try again later." / "Usajili wako kwa mamlaka ya usafiri bado unashughulikiwa. Jaribu tena baadaye."
- `licence_number: FORMAT`: "Enter the 10-digit licence number." / "Weka namba ya leseni yenye tarakimu 10."
- Expiring banner: "Your {document} expires on {date}. Upload the new one to keep driving." / "{document} yako inaisha tarehe {date}. Pakia mpya ili uendelee kuendesha."
- Expired hold: "Your {document} has expired. Upload a valid one; you can go online once it is approved." / "{document} yako imeisha muda. Pakia iliyo halali; utaweza kuingia mtandaoni ikishaidhinishwa."
- Renewal sent: "New {document} sent for review." / "{document} mpya imetumwa kukaguliwa."
- SMS: "{brand}: your {document} expires on {date}. Upload the new one in the app to keep driving." / "{brand}: {document} yako inaisha {date}. Pakia mpya kwenye programu ili uendelee kuendesha."
- Consent title: "Your privacy" / "Faragha yako".
- Consent body: "To use {brand} we process your phone number, name, trip locations and payment details as set out in our Privacy Notice and Terms." / "Ili kutumia {brand} tunachakata namba yako ya simu, jina, maeneo ya safari na taarifa za malipo kama ilivyoelezwa kwenye Taarifa ya Faragha na Masharti yetu."
- Consent buttons: "I agree" / "Nakubali"; "Read the Privacy Notice" / "Soma Taarifa ya Faragha"; "Decline and sign out" / "Kataa na utoke".
- Consent updated: "We have updated our Privacy Notice." / "Tumesasisha Taarifa yetu ya Faragha."
- `CONSENT_REQUIRED`: "Please accept the Privacy Notice to continue." / "Tafadhali kubali Taarifa ya Faragha ili kuendelea."
- Privacy screen:
  - "Download my data" / "Pakua taarifa zangu"; "Delete my account" / "Futa akaunti yangu".
  - "Requested on {date}. We will reply by {due}." / "Imeombwa tarehe {date}. Tutajibu kabla ya {due}."
  - "Ready to download until {time}." / "Iko tayari kupakuliwa hadi {time}."
- Delete confirmation: "This removes your name, phone, places and contacts. Trip and payment records are kept as the law requires." / "Hii inaondoa jina, simu, maeneo na watu wako wa karibu. Kumbukumbu za safari na malipo zinahifadhiwa kama sheria inavyotaka."
- `DELETE_BLOCKED`: "Finish your trip and settle your balance first." / "Maliza safari yako na ulipe salio lako kwanza."

## 7. Tests
**Go, S1:**
- `ladder_test` (rung per attempt; last rung repeats; stuck at 5).
- `ShouldSubmit` table (TZ taxi yes, TZ parcel no, KE none).
- `cets` with an httptest stub:
  - JWT verifies with the public key, `kid`, `aud`, `exp` = 300 s;
  - 5000 → reference; 2xx with 5001 → rejection; 5016 → NOT_NEEDED; 5018 → PREREQUISITE; 500 and timeout → unavailable;
  - payload golden files for all four kinds (local time, `lat,lng`, plate, vehicle type from class, integer total).
- Fake modes and plate/licence triggers; script queue.
- Compliance service (DB):
  - approve enqueues DRIVER+VEHICLE in the same tx (rollback → none);
  - COMPLETED trip enqueues one TRIP, a parcel none, KE none;
  - the sweeper waits for prerequisites;
  - `enabled=false` or `pending` makes no attempts;
  - 10 concurrent sweeps → each row sent once;
  - LOCAL_INVALID without a call;
  - edits reset to due;
  - zero-trip day SKIPPED, sent day not recomputed.
- Fail-closed: `PUT /driver/status` 409 until ACCEPTED, 200 after; switch audited; refused with `pending`.
- Config fails fast for `cets` with a missing key.
- JWKS routes: 503, then 200 with an ETag.

**Go, S2:**
- `ExpiryState` and `ReminderDue` tables (catch-up sends the tightest threshold once; nothing before 08:00 local; timezone boundary).
- Sweep (DB): reminders once each (a second run sends nothing); expired → DOCUMENT hold and offline; on-trip driver finishes first; renewal approval promotes, archives and lifts the hold; renewal rejection keeps the hold; staff expiry edit re-evaluates; MANUAL and DEBT holds untouched; UNKNOWN never holds.
- Uniqueness: two pending renewals impossible.

**Go, S3:**
- `CheckPing` table: jitter ignored, 2 s gap and 5 km jump strike, `mocked` strike.
- Presence behaviour test: 3 strikes in 10 min → one flag, a fourth → `hits=2`; zero Postgres queries per ping (counted).
- `SelfBookingSignals` table.
- Trip check (DB): a shared install flags; a receiver phone equal to the driver's flags.
- Dispatch never offers the rider's own driver account.
- Cancel scan (DB, seeded trips): 3 of 10 cancels flags, 2 of 10 does not, 9 trips below sample does not; no-show and rider variants; weekly dedupe.
- Review is audited and immutable.

**Go, S4:**
- Consent: gate 403 then 204 then 201; stale version 409; Redis miss path; new version re-requires.
- Retention, each rule against seeded old and new rows (only old touched; `retention_runs` once per day).
- Export JSON contains every section and no other user's data.
- Delete: anonymises, blockers each 409, receipts and ledger untouched (hash chain verifies), the phone can sign up again.
- Requests: one open per kind, 429.
- Download: link expired 410.

**Go, httpx:** all new routes declared with security; permissions enforced; error codes present in both i18n files.

**E2E** (each prints PASS lines):
- `apps/tools/e2e-regulator.mjs` (fake): approve a driver → DRIVER and VEHICLE ACCEPTED; complete a ride → TRIP ACCEPTED with reference; a `TFAIL` plate driver retries and goes stuck after a forced ladder; fail-closed 409 then 200; daily report via `nexa jobs run compliance.regulator.daily`.
- `e2e-documents.mjs`: staff sets the insurance expiry to yesterday, the sweep runs, 409 `DRIVER_ON_HOLD`; renewal upload, approve, online 200.
- `e2e-fraud.mjs`: three pings 10 km and 2 s apart → GPS flag; shared install id ride → SELF_BOOKING flag; dismiss.
- `e2e-privacy.mjs`: new rider 403 `CONSENT_REQUIRED` then accept and book; export request, approve, download JSON; delete blocked during a ride, allowed after.
- `e2e-admin.mjs` extended (four new pages).
- `shot-compliance.mjs` screenshots.
- New subcommand `nexa jobs run <kind>` (inserts the job and waits ≤ 30 s) for tests.

## 8. Acceptance
- [ ] Migrations 00031–00034 apply, roll back and re-apply; LICENCE expiry backfilled; parcel seed `submit_trips=false`.
- [ ] Regulator port with `fake` and `cets`; startup fails on a missing adapter or incomplete `cets` config; `pending` queues only.
- [ ] Approval registers driver and vehicle; every completed TZ taxi ride is submitted once with a stored reference; KE and parcels submit nothing.
- [ ] Retry follows 1m/5m/30m/3h/daily from one clock; stuck after 5 with an ERROR log; 5016 days closed locally; 5018/5019 re-register.
- [ ] Fail-closed blocks unregistered drivers only when on; switches audited; JWKS discovery served.
- [ ] Licence must match the country pattern (10 digits in TZ); class captured.
- [ ] Expiry required for expiring kinds; reminders 30/7/1 by push and SMS, once each; the hold the day after expiry; renewal re-enters review and approval lifts the hold.
- [ ] Driver never offered their own ride; each fraud detector has a test that makes it fire; flags dedupe; review audited; no automatic punishment.
- [ ] `/driver/location` makes no Postgres query; p95 before/after recorded.
- [ ] Consent recorded per app and version; the gate on booking, online and submit; a new version re-asks.
- [ ] Retention job runs once a day, touches only rows past their period, records counts; trips, receipts and ledger untouched.
- [ ] Export and delete requests handled by staff with reasons; blockers enforced; the deleted user is anonymised; the chain verifies.
- [ ] Admin Trust pages and Overview attention items; app screens with entrances and press states; EN copy in apps, EN/SW server messages.
- [ ] OpenAPI + both clients regenerated; gofmt, vet, golangci-lint, Biome, tsc and brand grep pass; `doc.go` in `compliance`, `fraud`, `privacy`, `providers/regulator`, `fake`, `cets`.
- [ ] p95 and queries recorded for trip completion (+ submission row + job), `PUT /driver/status`, `POST /trips` (consent), the admin lists, and EXPLAIN for the cancel scan and expiry queries.
- [ ] D12: no regulator logic in `trips` (#30), detectors fire (#23), stable codes (#7), routes declared (#13), install id in a header not a URL (#9), config fails fast (#26).

## 9. Owner questions (defaults in use)
1. LATRA CeTS access: test and production base URLs, our registered issuer domain, the `aud` value, and the public JWKS URL. Until then: `fake` in dev, `pending` in UAT and production.
2. Fail-closed is off until CeTS accepts our test submissions; turn it on at launch?
3. Parcel (Boda "Package") trips are not submitted to LATRA (default no); confirm with LATRA.
4. Coordinate order in CeTS strings `"lat,lng"` (the documentation's caption says longitude first); confirm with LATRA in writing.
5. Licence classes offered (default TZ `A, A1, A2, A3, B, C, C1, C2, C3, D, E`) and the 10-digit rule. Existing drivers with a non-matching licence are fixed by staff.
6. Inspection certificate optional at sign-up (tracked when uploaded); make it required? No grace period after expiry (the hold the next day).
7. Reminders 30/7/1 days by push **and** SMS (the SMS cost is the Platform's).
8. Fraud thresholds: 55 m/s (198 km/h), 3 strikes in 10 min; driver cancel ≥ 30 %, no-show ≥ 30 %, rider cancel ≥ 40 %, each with at least 10 in 7 days. Flags only; staff decide.
9. PDPA legal texts: Privacy Notice and Terms in EN and SW (URLs and version 1), DPO contact, and data-controller registration with Tanzania's Personal Data Protection Commission. Hosting location matters for cross-border transfer rules and affects Phase E.
10. Retention periods as in the §5 table. Trips, receipts and ledger kept at least 5 years (tax records); confirm with counsel.
11. Privacy requests answered within 30 days; exports delivered in-app through a 24 h link (no email); deletion is anonymisation that keeps trip and payment records.

## Factory log
- 2026-10-23: spec written. Slices queued S1 (regulator, fake adapter) → S2 (documents) → S3 (fraud) → S4 (PDPA). S1 can start now, because CeTS credentials are only needed to switch from `pending` to `cets`.
- 2026-10-23: owner direction (Kalen): LATRA and Selcom are Tanzania-only, not priorities; other countries have other regulators and gateways. Regulators and payment gateways stay generic ports chosen per country.
- 2026-10-23: **S1 built** (generic first):
  - **Regulator port** `providers/regulator`, with the `fake` adapter and the `cets` adapter (LATRA's). Each country's regulator gets its adapter from `NEXA_REGULATOR_<NAME>_ADAPTER`. The default is `pending` (queued, not sent); startup never fails for a missing adapter.
  - **Outbox and sweeper** in `internal/compliance`: one clock every minute, retry ladder 1m/5m/30m/3h/daily, stuck flag after 5 attempts, `LOCAL_INVALID` held without sending. Trips wait for their driver and vehicle registrations. Days with no trips close locally.
  - **Triggers:** approval and provisioning enqueue in their transaction, and completion enqueues the trip in its transaction. Existing approved drivers and vehicles were backfilled.
  - **Fail-closed gate** on going online, cached for 60 s. It cannot be turned on without a real adapter.
  - **Licence rules** per country from the brand file: the pattern and the class list (TZ: 10 digits, A–E). The onboarding form gets a class picker and a format hint.
  - **Admin:** Trust → Regulator page with country switches (audited) and the submissions table with Retry. Permission `compliance.manage`.
  - **Tests:** Go tests for the ladder, `ShouldSubmit`, outcomes, enqueue rollback, 10 concurrent sweeps sending each row once, nothing sent while switched off, the gate, and `LOCAL_INVALID`. Fake mode was checked on the local API: vehicles, drivers and trips accepted, and the zero-trip day skipped.
  - **Deferred** (Tanzania-specific, low priority): the JWKS discovery routes, driver and trip regulator badges, the driver-app "registration pending" banner, and CeTS assumptions to confirm with LATRA (coordinate order, phone format, whole-shilling amounts, 5018/5019).
- 2026-10-23: **payment gateways per country.** Each country's `payment_gateway` picks its collection and payout provider, keyed by currency. A country without one has no mobile money: rides fall back to cash and cash-out is off. Webhooks are routed by gateway name. The fake gateway only runs through the `NEXA_PAYMENT_GATEWAY=fake` development override.

- 2026-10-23: owner direction (Kalen): staff choose per country whether each document (licence, insurance, police clearance, ...) is mandatory, optional or not asked, and how its expiry is handled.
- 2026-10-23: **S2 built** (migration 00032):
  - **Document types per country** (`document_types`): requirement MANDATORY / OPTIONAL / OFF, `tracks_expiry`, `expiry_required`, `blocks_when_expired`, EN/SW labels and order. Seeded with the previous set, plus police clearance and vehicle inspection as optional. Staff can add new kinds. Changes are audited, and the list is cached for 30 s.
  - **Checklist:** only mandatory documents block approval; optional ones are shown as "(optional)"; OFF is hidden.
  - **Expiry date:** asked when uploading if the type tracks it. It is checked (required, format, not past, at most 10 years ahead). The licence expiry moved from the profile onto the document.
  - **Renewals:** a new copy for an approved driver sits beside the current one until staff approve it. The old copy is then archived.
  - **Daily sweep:** reminders 30, 7 and 1 days before expiry, once each, from 08:00 local time, sent as push and SMS. A `DOCUMENT` hold is placed when a blocking document has expired, and lifted when a valid renewal is approved.
  - **Driver app:** Account → Documents (state chips, Renew), the expiry date sheet in onboarding and renewals, and hold copy on Home.
  - **Admin:** Trust → Documents, with a Requirements tab (per-country selects, expiry checkboxes, "Add a document") and the Expiring / Expired / Missing / Renewals / No date tabs. Staff can correct an expiry date.
  - **Tests:** Go tests for expiry checks, reminder thresholds, the hold placed and lifted, and the checklist rules; app model tests; `e2e-onboarding` (expiry, hold, renewal) and `e2e-web-registration` pass.
  - Not yet run through the four factory reviewers.
- 2026-10-23 S3 built as `00038_fraud.sql` and S4 as `00039_privacy.sql` (00033/00034 were taken).
  - S3: `internal/fraud` (pure CheckPing, SelfBookingSignals, CancelFindings; Redis GPSGuard queues one job per
    burst instead of the per-day SETNX, so repeats in later windows add hits; scan reads trip_events because a driver
    cancel resets the trip row; contact phones normalised before comparing); presence `OnPing` hook reads the previous
    position with the existing HMGET; dispatch `offerable` excludes the rider's own account; `X-Install-Id` recorded on
    verify/refresh; admin `/compliance/fraud` (+ flag pages, person cards, overview row). Tests: rules, Redis guard,
    DB dedupe/review/audit, scan SQL; `e2e-fraud.mjs` PASS.
  - S4: `internal/privacy` (consent from the brand file `legal.consent[]`, Redis then Postgres; requests with
    blockers saved after the rolled-back transaction; anonymisation; export of 13 sections via `storage.Writer`;
    12 retention rules in batches with `nexa.retention` letting the fraud-flag guard allow stripping/deleting);
    `tools/accept-consent.mjs` for the fixed test accounts. Tests: consent gate and new version, delete blocked then
    anonymised, retention once a day and only old rows, every rule's SQL, every export section's SQL.
- 2026-10-24 reviews (security + code) on S3/S4, pricing S3 and trip money; all findings fixed (`00041_review_fixes.sql`):
  privacy requests carry the person's country (list, get, approve, reject and the overview count are scoped);
  deletion also clears driver ID/licence numbers and sign-in IP/device; a driver's export leaves out riders' stops;
  retention statements filter by brand; rejected-application files only when the files themselves are old; exports
  purged by link expiry and one failed file no longer stops the batch; rate limit window set atomically; scan and
  retention claims are re-taken an hour after a failed or unfinished run; install records written at most daily;
  a GPS flag escalates to the highest severity and keeps the previous evidence; pricing CSV needs finance.view,
  escapes formula cells and aborts the stream on a mid-way error; pricing history needs country-level access.
