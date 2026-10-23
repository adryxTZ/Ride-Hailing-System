# Spec: pricing profiles, extra charges, per-trip pricing record, geofences

Written 2026-10-23 by the spec-writer agent. Where the owner has not answered yet, the defaults in §9 apply. This spec builds on `2026-10-21-operating-areas.md` (countries, areas, scoped fee policies), `2026-10-21-fares-payouts-receipts.md` (the fare fixed at AWAITING_PAYMENT, receipts, the completing transaction) and `2026-10-21-debt-holds-payins.md` (the ledger accounts).

Owner's request (Kalen, 2026-10-23): "The system can have multiple profiles. The first is the default profile, and the others are profiles assigned to regions, districts, places, or geofenced zones. These profiles can have different processes or additional costs like toll fees, parking, levy, etc. Some profiles might be active, some not active. Profiles are a very sensitive matter: each trip must keep a RECORD of what profile was used, for auditing. The system must also keep a record of what profile it used to charge a certain trip and which profile was default at which region or fenced area (and when). Also a geofencing page."

## What the code does today (verified 2026-10-23)

- **Fee policies.** `fee_policies` (00004, with columns added in 00020, 00021, 00022, 00026, 00028 and 00030) stores versioned rows scoped by `(brand_id, country_code, area_id)`. The unique key is `fee_policies_scope_uq (brand, country, area, version)`.
  - They are **not** scoped to vehicle type: no column, no code. The owner's 2026-10-21 amendment ("a policy applies to … a vehicle type") was never built.
  - An area policy repeats the country's tax, debt and payout fields. `pricing.NewPolicy` cascades a new country version into every area policy.
- **Quotes.** `pricing.Service.Quote` (`internal/pricing/service.go`) takes the area policy, falling back to the country one (`tariffs.policy`). It uses that policy for the booking fee, rounding, waiting cap and cancellation terms. Nothing about the policy is stored on the quote or the trip, except those values inside `quote_option`.
- **The split. This is the bug that matters most here.**
  - `feeSplitter` (`cmd/nexa/cmd_serve.go:324`) calls `pricing.Service.FeePolicy(ctx)`, which returns the brand's **home-country default**. A trip in a region with its own commission, or in another country, is split on the wrong rates.
  - `trips.fixSplitTx` (`internal/trips/payment.go`) re-splits at AWAITING_PAYMENT with whatever policy is in force **at that moment**. If staff publish between booking and the end of the ride, the trip is split on rates it was never quoted. This breaks D12 #17 ("rates … pinned per trip").
  - `money.Breakdown` records only `policy_version`. That number repeats across scopes, so it cannot identify which policy was used.
  - `FeePolicyFor(country, area)` exists but nothing calls it.
- **The pricing cache.** It loads policies `WHERE effective_from <= now()`, but it only reloads when the `pricing` config version changes. A version dated in the future therefore never takes effect unless some other write happens later.
- **Debt and payout limits.** Debt-hold limits (`wire_payments.go:141`) and payout rules (`wire_payouts.go:42`) also read the home-country policy. This is a multi-country gap but outside this spec; see §2 Out.
- **Lines and receipts.**
  - Fare lines are `engine.Line {code, amount, meta}`.
  - Receipts label lines from the `receipts/labels.go` table, and `labelFor` turns an unknown code into words.
  - The rider app (`apps/rider/src/components/taxi/FareBreakdown.tsx`) labels lines from a hard-coded EN table, falling back to the raw code. The admin `TripDetail.tsx` has its own `LINE_LABEL` table.
  - The driver app renders no fare lines.
  - The OpenAPI `FareLine.code` enum has already drifted: it lacks `ROUNDING` and `PREVIOUS_CANCELLATION`, which the server sends.
  - `FareBreakdown.tsx` hard-codes "Includes LATRA levy", a country literal inside a generic app.
- **Areas and geometry.**
  - Areas are flat sets of res-7 cells. The `area_cells` primary key forbids overlap, so areas cannot nest (no districts inside regions).
  - Area cells are replaced in place, with no history.
  - Named places exist only inside `fixed_routes`, as origin and destination with a radius (00006).
- **Migrations.** The latest file is `00033_people.sql`, and `00034_branding.sql` is being built in parallel. This spec uses **00035, 00036 and 00037**.
  - The compliance spec planned 00033 and 00034 for its S3 and S4. Both numbers are now taken, so those slices must take the next free numbers when they are built.
- **Files owned by the branding build.** Do not edit `internal/kernel/theme`, `internal/kernel/seed` or `handlers_public.go`. The pricing seed lives in `internal/pricing/store.go` (`seedPolicy`), so it is unaffected.

## Build slices, in order
Each slice ships alone, with its own migration, tests and factory review.
- **S1. Profiles, charges, trip record** (`00035_pricing_profiles.sql`)
  - Profiles replace fee policies, and the existing rows are migrated.
  - Profiles can be assigned to the country (as the default) and to areas (regions).
  - Each profile carries extra charge lines, and charges have payees.
  - `Split()` is extended for charges, and the ledger gets `CHARGES_PAYABLE`.
  - Every trip gets an immutable pricing record.
  - The Fees page is redesigned as a profile list plus a profile page, and the trip page gains a "Pricing record" card.
  - **The split bug is fixed: the trip's pinned terms are used, never the home country's.**
- **S2. Geofences and places** (`00036_geofences.sql`)
  - Districts, zones and places, with geometry versions.
  - An in-process index, so a quote never touches Postgres to resolve a geofence.
  - Profile assignments to DISTRICT, ZONE and PLACE scopes.
  - Charges triggered by drop-off and by the route crossing a zone.
  - The Geofences page (draw polygons, place circles, GeoJSON import, overlap warnings).
- **S3. History, "as of" and simulator** (`00037_pricing_history.sql`, which only adds indexes and a view)
  - The question "what would have priced point X at time T".
  - The assignment and default timeline per country and scope.
  - "Simulate a trip" on a map.
  - A CSV export of pricing records.

## 1. Goal
Staff manage **pricing profiles** per country.
- One profile is the country's **default**. Other profiles are assigned to a region (S1), or to a district, a geofenced zone or a named place (S2).
- A profile carries the trip money rules (commission, booking fee, rounding, waiting cap, cancellation terms) and a list of **extra charges**, such as toll, parking, levy, airport or entry fee, or a night surcharge. Each charge has an EN/SW label, an amount rule, a recipient and a condition for when it applies.
- Profiles are DRAFT, ACTIVE or INACTIVE, with an optional activation window. Only ACTIVE profiles inside their window ever price a trip.
- When a rider asks for a quote, the Platform picks exactly one profile at the pickup, by fixed precedence and without querying Postgres. It adds any charges that apply, shows them as labelled lines on the quote and the receipt, and splits the money with one `Split()`: platform revenue, the driver, VAT and levy, and third-party payables in the ledger.
- Every trip keeps an **immutable pricing record**: the profile, its version and scope, why it won (the candidates considered), the country default's version, every charge applied with its amount and recipient, and a hash. The database refuses any change once the trip is final.
- Staff see the record on the trip page and can export it.
- An append-only history answers "which profile was the default, or assigned, for this region or zone, from when to when, who changed it and why". It also answers "what would have priced a trip at this point at that time" (S3).
- A Geofences page lets staff draw zones and places (S2).

## 2. Scope
**In, S1:**
- New module `internal/profiles`: profiles, immutable versions, charges, payees, assignments (COUNTRY, AREA), events, the pure resolver, charge evaluation and the trip pricing records.
- The migration turns every `fee_policies` scope into a profile with the same version numbers. The old table becomes read-only `fee_policies_legacy`.
- Country-only fields (VAT, levy, debt and payout rules) live only on the default profile. Other profiles store NULL (meaning "inherit"), which removes the cascade in `pricing.NewPolicy`.
- `money.Split` takes the charge allocations. `money.Breakdown` gains profile ids, charges and `third_party`.
- The ledger gets account kind `CHARGES_PAYABLE`, owned per payee.
- Charges: FLAT, PER_KM or PERCENT_OF_FARE, with optional min and max. Recipient FARE, PLATFORM, DRIVER or THIRD_PARTY. Condition PICKUP or EITHER (scopes are country or area in S1). Optional filters by service and vehicle class, an "on fixed fares" flag, and a weekday and local-time window.
- Quotes store the per-class pinned terms (`quotes.pricing`). Booking pins them in `trip_pricing_records`. The split reads the pin, and completion finalises the record.
- Admin:
  - Profiles list at `/pricing/fees`, the profile page `/pricing/fees/[id]` and `/pricing/fees/new`.
  - A Payees tab.
  - The trip page "Pricing record" card, with a JSON download.
  - A Finance "Owed to third parties" tile.
- Labels for CHARGE lines in the rider app, the admin and receipts.
- `POST /admin/pricing/policy` is kept as a deprecated shim, so the existing e2e scripts keep working.

**In, S2:**
- New module `internal/geofences`: DISTRICT and ZONE (polygon or multipolygon) and PLACE (point and radius). Geometry is versioned.
- Validation with PostGIS on save, an H3 res-7 prefilter and an exact in-process containment index.
- GeoJSON import (dry run, then apply) and an overlap report.
- Assignment scopes DISTRICT, ZONE and PLACE. Charge conditions DROPOFF, BOTH and ROUTE_CROSSES.
- Admin `/pricing/geofences` (list and map) and `/pricing/geofences/[id]`.

**In, S3:**
- `GET /admin/pricing/as-of` uses the same pure resolver over a snapshot read from history.
- `GET /admin/pricing/history` returns a timeline per country and scope.
- `POST /admin/pricing/simulate` runs a real quote without saving it, optionally previewing a DRAFT profile.
- `GET /admin/pricing/records.csv`.
- Admin `/pricing/fees/simulate` and `/pricing/fees/history`.

**Out:**
- Per-vehicle-type profiles. Tariffs are already per type, and charges can be limited by class.
- Nested operating areas. Districts are geofences, not areas.
- Area-cell history. "As of" for an AREA scope uses today's cells and says so.
- Settling third-party payables with the payee (Finance pays outside the app, as with VAT).
- Charges re-priced from the actual driven route. Charges are fixed at the quote, like the fare.
- Charges on cancellation and no-show fees.
- Promotions and discounts.
- Driver-entered tolls with photo receipts.
- App translation layer (the apps stay EN-only; SW copy is listed for the deferred i18n slice).
- Making debt-hold limits and payout rules follow the driver's country (`wire_payments.go`, `wire_payouts.go`). This stays a separate fix, listed in the roadmap.
- Any edits to `internal/kernel/theme`, `internal/kernel/seed` or `handlers_public.go`.

## 3. Contract
All new routes go in `backend/api/openapi.yaml` with `security`, and the both TypeScript clients are regenerated. All new error codes go into `backend/i18n/{en,sw}.json`. Staff writes need `reason` and are audited through `audit.Record`. Staff with a country or region scope are checked with `allowedScope`, and profile writes need country-level scope (`SCOPE_FORBIDDEN` otherwise, the existing code). GETs return an ETag (`writeJSONWithETag`). No new permission: profiles and payees use `fees.edit`, geofences use `areas.edit`, and the records CSV uses `finance.view`. Any staff member who can open a trip can read its pricing record.

**Schemas (S1):**
- `ChargeRule`:
  - `{code, label {en, sw}, category, amount_type, amount, min_amount?, max_amount?, recipient, payee_id?, taxable, applies_when, services[], vehicle_classes[], on_fixed_routes, days[], from_time?, to_time?}`.
  - `code` matches `^[A-Z][A-Z0-9_]{1,31}$`.
  - `category` is one of TOLL, PARKING, LEVY, AIRPORT_FEE, ENTRY_FEE, NIGHT, OTHER. It is used only for reports.
  - `amount_type` is FLAT, PER_KM or PERCENT_OF_FARE.
  - `recipient` is FARE, PLATFORM, DRIVER or THIRD_PARTY.
  - `applies_when` is PICKUP, DROPOFF, EITHER, BOTH or ROUTE_CROSSES. DROPOFF, BOTH and ROUTE_CROSSES are refused until S2 with `CHARGE_CONDITION_UNAVAILABLE`.
  - `days` holds ISO weekdays 1–7 (empty means every day). `from_time` and `to_time` are `HH:MM` in the country's timezone, and the window may wrap past midnight.
- `ProfileTerms`:
  - Fields `commission_rate, booking_fee, waiting_cap_rate, fare_rounding, cancel_free_s, no_show_after_s, cancel_driver_share` are nullable on non-default profiles (null means inherit from the default).
  - Country-only fields: `vat_rate, vat_base, levy_rate, levy_borne_by, debt_soft_limit, debt_hard_limit, debt_soft_grace_days, payin_min_amount, payout_min_amount, payout_auto_approve`.
- `ProfileVersion {id, version, effective_from, terms: ProfileTerms, charges: ChargeRule[], content_hash, published_by {id, name}, reason, created_at}`
- `ProfileScope {assignment_id, kind: COUNTRY|AREA|DISTRICT|ZONE|PLACE, id, name, priority, valid_from, valid_to?}`
- `ProfileSummary {id, country, name, is_default, status: DRAFT|ACTIVE|INACTIVE, active_from?, active_until?, current_version?, next_version? {version, effective_from}, charges_count, scopes: ProfileScope[] (open ones), updated_at, updated_by?}`
- `Profile`: `ProfileSummary` plus `{description, versions: ProfileVersion[] (latest 50), assignments: ProfileScope[] (open and closed, latest 100), events: ProfileEvent[] (latest 100)}`
- `ProfileEvent {at, kind: CREATED|MIGRATED|PUBLISHED|ACTIVATED|DEACTIVATED|ASSIGNED|UNASSIGNED|MADE_DEFAULT|DEFAULT_REPLACED|RENAMED|WINDOW_CHANGED, version?, scope?, actor {id, name}|SYSTEM, reason}`
- `Payee {id, country, name, kind: TAX_AUTHORITY|REGULATOR|AIRPORT|MUNICIPALITY|PARKING_OPERATOR|OTHER, reference?, status: ACTIVE|INACTIVE}`
- `PricingRecord`:
  - `{trip_id, quote_id, stage: QUOTED|FINAL, country, profile {id, name, version, version_id, content_hash}, scope {kind, id, name}, default_profile {id, name, version, version_id}, terms (effective, merged), charges: AppliedCharge[], trace: ResolutionTrace, config {pricing_version, geofences_version, resolver_version}, final? {breakdown, completed_at}, record_hash?, created_at, finalized_at?, source: LIVE|BACKFILL|FALLBACK}`
  - `AppliedCharge {code, label, category, recipient, payee? {id, name}, taxable, applies_when, matched_at: PICKUP|DROPOFF|ROUTE, from_profile {id, version}, amount, vat}`
  - `ResolutionTrace {point: pickup|dropoff, candidates: [{scope {kind, id, name}, profile {id, name, version}, priority, outcome: WON|LOWER_PRECEDENCE|INACTIVE|DRAFT|OUT_OF_WINDOW|NO_VERSION_IN_FORCE|TIE_BROKEN}], winner_rule}`
- `FareLine`:
  - The `code` enum gains `ROUNDING`, `PREVIOUS_CANCELLATION` (drift fix) and `CHARGE`.
  - New optional `label {en, sw}`. CHARGE lines carry `meta {charge, category, recipient}`.
- `FinanceSummary` gains `charges_payable: [{payee {id, name, kind}, amount}]`.

**Routes (S1):**
- `GET /admin/pricing/profiles?country` returns `{data: ProfileSummary[]}`, with the default first and then the rest by name.
- `POST /admin/pricing/profiles {country, name, description?, reason}` returns 201 `Profile` (DRAFT, no version).
  - 409 `PROFILE_NAME_TAKEN`.
  - 409 `PROFILE_LIMIT_REACHED {max}`.
- `GET /admin/pricing/profiles/{id}` returns `Profile`, or 404 `PROFILE_NOT_FOUND`.
- `PATCH /admin/pricing/profiles/{id} {name?, description?, active_from?, active_until?, reason}` returns `Profile`.
  - 400 `PROFILE_WINDOW_INVALID` (until ≤ from, or until in the past).
  - 409 `PROFILE_IS_DEFAULT` if a window is set on the default.
  - 409 `PROFILE_PRECEDENCE_TIE`.
- `POST /admin/pricing/profiles/{id}/versions {terms, charges[], effective_from?, reason}` returns 201 `ProfileVersion`.
  - 400 `VALIDATION_FAILED {fields}`.
  - 400 `CHARGE_INVALID {index, fields}`.
  - 409 `CHARGE_CODE_DUPLICATE {code}`.
  - 400 `TOO_MANY_CHARGES {max}`.
  - 409 `POLICY_FIELD_COUNTRY_ONLY {fields}` (existing code; on a non-default profile).
  - 409 `PROFILE_INCOMPLETE {fields}` (the default must set every field).
  - 400 `EFFECTIVE_FROM_INVALID` (in the past by more than `EffectiveFromSkew`, or later than `EffectiveFromMaxAhead`).
  - 409 `PAYEE_INVALID` (missing, inactive or another country).
  - 409 `CHARGE_CONDITION_UNAVAILABLE`.
- `POST /admin/pricing/profiles/{id}/status {status: ACTIVE|INACTIVE, reason}` returns `Profile`.
  - 409 `PROFILE_IS_DEFAULT` (the default cannot be deactivated).
  - 409 `PROFILE_NOT_PUBLISHED` (no version).
  - 409 `PROFILE_PRECEDENCE_TIE {conflicts}`.
  - 409 `PROFILE_STATUS_UNCHANGED`.
- `POST /admin/pricing/profiles/{id}/assignments {scope_kind, scope_id, priority?, reason}` returns 201 `ProfileScope`.
  - 404 `SCOPE_NOT_FOUND`.
  - 409 `SCOPE_COUNTRY_MISMATCH`.
  - 409 `SCOPE_ALREADY_ASSIGNED {profile {id, name}}` (close the other one first).
  - 409 `PROFILE_PRECEDENCE_TIE {conflicts}`.
  - 400 `SCOPE_KIND_UNAVAILABLE` (COUNTRY goes through make-default; DISTRICT, ZONE and PLACE before S2).
- `DELETE /admin/pricing/profiles/{id}/assignments/{assignment_id}?reason=` returns 204. It closes the assignment with `valid_to = now()`.
  - 409 `ASSIGNMENT_CLOSED`.
  - 409 `PROFILE_IS_DEFAULT` (the COUNTRY assignment is only replaced, never closed).
- `POST /admin/pricing/profiles/{id}/make-default {reason}` returns `Profile`.
  - In one transaction, under the country's advisory lock, it closes the old COUNTRY assignment and opens this one.
  - 409 `PROFILE_NOT_ACTIVE`.
  - 409 `PROFILE_INCOMPLETE {fields}`.
  - 409 `PROFILE_HAS_WINDOW`.
  - 409 `PROFILE_HAS_SCOPES` (close its other assignments first, so one profile is not both the default and a region override).
- `GET /admin/pricing/payees?country` and `POST /admin/pricing/payees {country, name, kind, reference?, reason}` (201), plus `PATCH /admin/pricing/payees/{id} {name?, status?, reason}`.
  - 409 `PAYEE_IN_USE` when deactivating a payee that an in-force version uses.
- `GET /admin/trips/{id}/pricing-record` returns `PricingRecord`.
  - Once final, the ETag is the `record_hash` and the response is `Cache-Control: private, max-age=86400, immutable`.
  - 404 `PRICING_RECORD_NOT_FOUND`.
  - 403 `SCOPE_FORBIDDEN`.
- `GET /admin/pricing`: `policy` and `policy_history` are now built from the scope's winning profile, and gain `profile {id, name}`. The field is deprecated and is removed in S3.
- `POST /admin/pricing/policy`: deprecated shim. It publishes a new version of the scope's own profile, creating an area profile assigned to that area if there is none, then returns as today.
- `POST /quotes`: unchanged in shape. Options may contain CHARGE lines with a `label`. Rider responses never include rates, profiles or traces.
- `GET /admin/finance` adds `charges_payable`.

**Routes (S2):**
- `Geofence`:
  - `{id, country, kind: DISTRICT|ZONE|PLACE, code, name, status: ACTIVE|INACTIVE|ARCHIVED, version, geometry?: GeoJSON Polygon|MultiPolygon (lng, lat order), center?: Point, radius_m?, area_m2, vertex_count, bbox, areas: AreaRef[] (operating areas it touches), profile?: {id, name, status} (open assignment), updated_at}`.
  - `GeofenceVersion {version, geometry|center+radius, created_by, reason, created_at}`.
- `GET /admin/geofences?country&kind&status` returns `{data: Geofence[]}` (ETag on the `geofences` config version).
- `POST /admin/geofences {country, kind, code, name, geometry | (center, radius_m), reason}` returns 201.
  - 400 `GEOFENCE_INVALID {reason: SELF_INTERSECTION|RING_NOT_CLOSED|TOO_FEW_POINTS|TOO_MANY_VERTICES|TOO_LARGE|TOO_SMALL|RADIUS_OUT_OF_RANGE|WRONG_TYPE, detail}`.
  - 409 `GEOFENCE_CODE_TAKEN`.
  - 409 `GEOFENCE_IN_OTHER_COUNTRY` (its centroid falls in another country's area).
- `GET /admin/geofences/{id}` returns the geofence with `versions[]` (latest 50).
- `PUT /admin/geofences/{id}/geometry {geometry | (center, radius_m), reason}` writes the next version.
  - It has the same 400 errors as create.
  - 409 `PROFILE_PRECEDENCE_TIE`.
  - 409 `GEOFENCE_ARCHIVED`.
- `PATCH /admin/geofences/{id} {name?, status?, reason}`.
  - 409 `GEOFENCE_IN_USE {profile}` (archive or deactivate while an assignment is open).
- `POST /admin/geofences/import {country, kind, features: GeoJSON FeatureCollection, dry_run, reason}` (body ≤ `ImportMaxBytes`, ≤ `ImportMaxFeatures`).
  - Each feature needs `properties.name` and `properties.code`.
  - Returns `{results: [{code, outcome: CREATE|UPDATE|INVALID|SKIP, error?}]}`. With `dry_run=false` it applies all valid features in one transaction, or 400 `IMPORT_INVALID` when any feature is invalid.
- `GET /admin/geofences/overlaps?country` returns `{data: [{a, b, kind: SAME_KIND|NESTED, both_assigned, same_priority}]}`.

**Routes (S3):**
- `POST /admin/pricing/simulate {service, vehicle_class?, pickup, dropoff?, waypoints?, at?, preview_profile_id?}`.
  - Returns `{resolution: PricingRecord (stage QUOTED, not saved), options: QuoteOption[] with lines, splits: Breakdown[] per class}`.
  - `at` must be within ±7 days of now (`SIMULATE_AT_OUT_OF_RANGE`).
  - `preview_profile_id` makes a DRAFT or INACTIVE profile take part as if ACTIVE, in this response only.
  - Nothing is persisted, and no quote row is written.
- `GET /admin/pricing/as-of?country&lat&lng&at&dropoff_lat?&dropoff_lng?` returns `{resolution: ResolutionTrace, profile, scope, default_profile, charges: ChargeRule[] that would match, approximate: bool, notes[]}`.
  - `approximate` is true when an AREA scope's cells changed after `at`.
  - 400 `AS_OF_IN_FUTURE`.
- `GET /admin/pricing/history?country&scope_kind?&scope_id?&from&to` returns `{assignments: ProfileScope+profile[], events: ProfileEvent[]}` (range ≤ 366 days).
- `GET /admin/pricing/records.csv?country&from&to` streams a CSV with one row per finalised record (≤ `RecordsExportMaxDays`).
  - 400 `RANGE_TOO_LARGE`.

**i18n (EN / SW)**, examples. The builder adds every code.

| Code | EN | SW |
|---|---|---|
| `PROFILE_IS_DEFAULT` | "The default profile can't be switched off or limited. Make another profile the default first." | "Wasifu chaguo-msingi hauwezi kuzimwa. Weka wasifu mwingine kuwa chaguo-msingi kwanza." |
| `PROFILE_PRECEDENCE_TIE` | "Another active profile covers the same place with the same priority. Change one priority." | "Wasifu mwingine hai unahusu eneo hilo kwa kipaumbele sawa. Badilisha kipaumbele kimoja." |
| `SCOPE_ALREADY_ASSIGNED` | "This place already has a profile. Remove it first." | "Eneo hili tayari lina wasifu. Uondoe kwanza." |
| `CHARGE_INVALID` | "Check the charge's details." | "Kagua maelezo ya tozo." |
| `GEOFENCE_INVALID` | "The shape isn't valid: {reason}." | "Umbo si sahihi: {reason}." |
| `PRICING_RECORD_NOT_FOUND` | "This trip has no pricing record." | "Safari hii haina kumbukumbu ya bei." |

## 4. Data

### `00035_pricing_profiles.sql` (S1)
The file is `NO TRANSACTION` because of `ALTER TYPE`, so every statement is idempotent.
- `ALTER TYPE account_kind ADD VALUE IF NOT EXISTS 'CHARGES_PAYABLE'`. The `owner_id` is the payee id; `ledger_accounts.owner_id` has no FK, which suits this.
- `charge_payees`:
  - Columns: `id uuid PK, brand_id FK, country_code char(2), name text CHECK 1–80, kind text CHECK (…), reference text, status text CHECK ACTIVE|INACTIVE DEFAULT ACTIVE, created_by, created_at, updated_at`.
  - FK `(brand_id, country_code) → countries`, and `UNIQUE (brand_id, country_code, name)`.
- `pricing_profiles`:
  - Columns: `id uuid PK, brand_id, country_code, name text CHECK 1–80, description text CHECK ≤ 500, status text CHECK DRAFT|ACTIVE|INACTIVE DEFAULT 'DRAFT', active_from timestamptz, active_until timestamptz, created_by, created_at, updated_at`.
  - Constraints: `CHECK (active_until IS NULL OR active_from IS NULL OR active_until > active_from)`, FK to countries, `UNIQUE (brand_id, country_code, name)`.
  - Index `(brand_id, country_code, status)`.
- `pricing_profile_versions`:
  - Columns: `id uuid PK, profile_id FK, version int, effective_from timestamptz NOT NULL`.
  - The trip-rule columns have the same types as `fee_policies`, **nullable**.
  - The country-only columns are also nullable, with `CHECK` constraints that mirror `fee_policies_debt_limits_chk` when they are set.
  - Further columns: `content_hash text NOT NULL, published_by uuid, reason text NOT NULL, legacy_fee_policy_id uuid, created_at`.
  - `UNIQUE (profile_id, version)` and index `(profile_id, effective_from DESC)`.
  - Trigger `pricing_profile_versions_immutable`: BEFORE UPDATE OR DELETE, RAISE.
- `pricing_profile_charges`:
  - Columns: `id uuid PK, version_id FK, code text CHECK (code ~ '^[A-Z][A-Z0-9_]{1,31}$'), label jsonb NOT NULL CHECK (label ? 'en' AND label ? 'sw'), category text CHECK, amount_type text CHECK, amount numeric(14,4) CHECK ≥ 0, min_amount numeric(14,2), max_amount numeric(14,2) CHECK (max ≥ min), recipient text CHECK, payee_id uuid REFERENCES charge_payees, taxable boolean DEFAULT false, applies_when text CHECK, services text[] DEFAULT '{}', vehicle_classes text[] DEFAULT '{}', on_fixed_routes boolean DEFAULT true, days smallint[] DEFAULT '{}', from_minute smallint CHECK 0–1439, to_minute smallint CHECK 0–1439, sort_order int`.
  - `CHECK ((recipient = 'THIRD_PARTY') = (payee_id IS NOT NULL))`.
  - `CHECK (NOT taxable OR recipient = 'PLATFORM')`.
  - `CHECK (amount_type <> 'PERCENT_OF_FARE' OR amount <= 1)`.
  - `CHECK ((from_minute IS NULL) = (to_minute IS NULL))`.
  - `UNIQUE (version_id, code)`, plus the same immutable trigger as versions.
- `pricing_profile_assignments`:
  - Columns: `id uuid PK, brand_id, country_code, profile_id FK, scope_kind text CHECK COUNTRY|AREA|DISTRICT|ZONE|PLACE, scope_id text NOT NULL (the country code for COUNTRY, else the uuid), priority int DEFAULT 0 CHECK -1000..1000, valid_from timestamptz NOT NULL DEFAULT now(), valid_to timestamptz, assigned_by, reason text NOT NULL, closed_by, close_reason`.
  - `CHECK (valid_to IS NULL OR valid_to >= valid_from)`.
  - Partial unique index `pricing_assign_open_uq (brand_id, scope_kind, scope_id) WHERE valid_to IS NULL`: one open profile per scope, and therefore exactly one open COUNTRY default per country.
  - Indexes `(profile_id, valid_from DESC)` and `(brand_id, country_code, valid_from, valid_to)` (for as-of).
  - Trigger `pricing_assignments_close_only`: DELETE is refused. UPDATE is allowed only when `OLD.valid_to IS NULL`, only `valid_to`, `closed_by` and `close_reason` change, and `NEW.valid_to IS NOT NULL`.
- `pricing_profile_events`:
  - Columns: `id bigserial, brand_id, profile_id, kind text, version int, assignment_id uuid, actor_id uuid (NULL = system), reason text, data jsonb, at timestamptz DEFAULT now()`.
  - Index `(profile_id, at DESC)` and an append-only trigger.
- `quotes ADD COLUMN pricing jsonb`. It holds the per-class pinned terms and the shared trace. It is written in the same `INSERT` in `saveQuote`.
- `trip_pricing_records`:
  - Columns: `trip_id uuid PK REFERENCES trips ON DELETE RESTRICT, brand_id, quote_id uuid, country_code, profile_id uuid NOT NULL, profile_version_id uuid NOT NULL, profile_version int, default_profile_id uuid, default_version_id uuid, scope_kind text, scope_id text, scope_name text, terms jsonb NOT NULL, charges jsonb NOT NULL DEFAULT '[]', trace jsonb NOT NULL, config jsonb NOT NULL, source text CHECK LIVE|BACKFILL|FALLBACK, stage text CHECK QUOTED|FINAL DEFAULT 'QUOTED', final jsonb, record_hash text, created_at, finalized_at`.
  - Indexes `(profile_id, created_at DESC)` and `(brand_id, country_code, finalized_at DESC) WHERE stage='FINAL'` (for the CSV).
  - Trigger `trip_pricing_records_guard`:
    - DELETE is refused.
    - UPDATE is refused unless `OLD.finalized_at IS NULL AND NEW.stage='FINAL' AND NEW.finalized_at IS NOT NULL AND NEW.record_hash IS NOT NULL`, and every pinned column (`profile_*`, `default_*`, `scope_*`, `terms`, `charges`, `trace`, `config`, `quote_id`, `source`) is unchanged (`IS NOT DISTINCT FROM`).
- **Data migration:**
  - One profile per distinct `(brand_id, country_code, area_id)` in `fee_policies`. The name is "Country default" for the country row and `areas.name` for area rows. Status is ACTIVE.
  - One version per policy row (same `version`, `effective_from`, `legacy_fee_policy_id`, `reason = 'migrated from fee_policies'`). For area rows only the country-only columns become NULL (they were copies of the country's); trip-rule columns keep their explicit values so migrated area profiles price exactly as before. The default rows keep every value.
  - Assignments: COUNTRY for the country rows and AREA for the area rows, with `valid_from = min(created_at)` of the group.
  - One `MIGRATED` event per profile.
  - `content_hash = encode(sha256(convert_to(<canonical text>, 'UTF8')), 'hex')` (the Go side recomputes and checks it at startup in tests).
  - Then `ALTER TABLE fee_policies RENAME TO fee_policies_legacy` and add an immutable trigger. A cleanup migration may drop it after owner acceptance.
- **Backfill** `trip_pricing_records` for every trip with `fees IS NOT NULL`: `source='BACKFILL'`, with the home country's default profile and the version numbered `fees->>'policy_version'`. This is a fact, not a guess: `feeSplitter` always used the home-country policy. Fields:
  - `trace = {"backfilled": true, "note": "split used the home-country policy"}`;
  - `charges='[]'`;
  - COMPLETED trips get `stage='FINAL'`, `final = {breakdown: fees}`, a `record_hash` and `finalized_at = completed_at`;
  - other trips stay `QUOTED`.
- `areas/store.go` readiness check: `policies` now counts the open COUNTRY assignment of the area's country.
- Down migration: recreate `fee_policies` from `fee_policies_legacy`, then drop the new tables in reverse order. The enum value stays, because Postgres cannot drop it.

### `00036_geofences.sql` (S2)
- `geofences`:
  - Columns: `id uuid PK, brand_id, country_code, kind text CHECK DISTRICT|ZONE|PLACE, code text CHECK '^[a-z0-9][a-z0-9-]{1,47}$', name text CHECK 1–80, status text CHECK ACTIVE|INACTIVE|ARCHIVED DEFAULT 'ACTIVE', version int NOT NULL, geom geography(MultiPolygon,4326), center geography(Point,4326), radius_m int CHECK 50–5000, area_m2 double precision, vertex_count int, created_at, updated_at`.
  - `CHECK ((kind='PLACE') = (center IS NOT NULL AND radius_m IS NOT NULL AND geom IS NULL))`.
  - `UNIQUE (brand_id, country_code, code)`.
  - Indexes: GiST on `geom`, GiST on `center`, and `(brand_id, country_code, kind, status)`.
- `geofence_versions`:
  - Columns: `geofence_id FK, version int, geom, center, radius_m, created_by, reason text NOT NULL, created_at`.
  - PK `(geofence_id, version)`, with an append-only trigger. Each geometry save writes a version row and updates `geofences.version` and `geom`/`center` in the same transaction.
- `INSERT INTO config_versions (scope) VALUES ('geofences')`.
- `pricing_profile_assignments.scope_kind` already allows DISTRICT, ZONE and PLACE. S2 only lifts the service-level `SCOPE_KIND_UNAVAILABLE`.

### `00037_pricing_history.sql` (S3)
- Index `geofence_versions (geofence_id, created_at)`.
- View `pricing_assignment_timeline`: assignments joined to the profile name and the scope name, both current and archived.
- No new tables.

**Redis.** No new keys. The quote hot path uses the in-process caches, refreshed through `config_versions`:
- `pricing` (existing; bumped by profile, version, assignment and payee writes);
- `geofences` (S2).

**Seeds.**
- `backend/seed/pricing/fee_policy.json` keeps its shape. `seedPolicy` (`internal/pricing/store.go`) now publishes a new version of the home country's default profile, and creates "Country default" with a COUNTRY assignment on an empty database, when the values differ.
- No charges, payees or geofences are seeded: those money values belong to the owner (§9).

**Constants.**
- `internal/profiles/model.go`:
  - `MaxChargesPerVersion = 20`
  - `MaxProfilesPerCountry = 200`
  - `EffectiveFromMaxAhead = 90 * 24h`
  - `EffectiveFromSkew = 5 * time.Minute`
  - `ResolverVersion = 1`
  - `PrecedencePlace = 400`, `PrecedenceZone = 300`, `PrecedenceDistrict = 200`, `PrecedenceArea = 100`, `PrecedenceCountry = 0`
  - `RecordsExportMaxDays = 31`
  - `HistoryMaxDays = 366`
- `internal/geofences/model.go`:
  - `IndexResolution = 7`
  - `MaxVertices = 2000`
  - `MinAreaM2 = 1_000`
  - `MaxAreaKm2 = map[Kind]float64{District: 50_000, Zone: 1_000}`
  - `PlaceRadiusMinM = 50`, `PlaceRadiusMaxM = 5_000`
  - `ImportMaxFeatures = 200`, `ImportMaxBytes = 2 << 20`
  - `MaxRoutePoints = 5_000`

## 5. Behaviour

### Module layout
- `internal/profiles/`:
  - `doc.go`;
  - `model.go`: Profile, Version, Terms, ChargeRule, Assignment, Event, Payee, PricingRecord, constants, errors;
  - `resolve.go`: the pure `Resolve(snapshot, Input) Resolution`;
  - `charges.go`: the pure `ApplyCharges(rules, ChargeInput) []AppliedCharge`;
  - `terms.go`: the pure `Merge(default, own) Terms` and `Validate`;
  - `snapshot.go`: the in-memory `Snapshot` and its builder;
  - `store.go`;
  - `service.go`: admin use cases;
  - `records.go`: `PinTx`, `FinalizeTx`, `Get`, `ExportCSV`;
  - `ports.go`: `GeofenceIndex`, `AreaIndex`, `Auditor`.
- `internal/geofences/`:
  - `doc.go`, `model.go`;
  - `index.go`: the pure index, containment, route crossing;
  - `geometry.go`: pure GeoJSON parsing and ring checks;
  - `store.go`: PostGIS validation and persistence;
  - `service.go`, `ports.go`.
- Handlers: `internal/kernel/httpx/handlers_profiles.go` and `handlers_geofences.go`, each registering its routes like `handlers_areas.go`.
- `pricing` depends on `profiles` only through a port (`pricing/ports.go`: `ProfileResolver`). Trips stays money-free: it gets new hook ports only.

### Statuses and windows
- A profile moves DRAFT → ACTIVE through `/status`, which needs a published version. It then moves between ACTIVE and INACTIVE freely, with a reason each time.
- A profile is never deleted (records reference it).
- A profile **prices** only when all of these hold:
  - its status is ACTIVE;
  - `now` is in `[active_from, active_until)`;
  - it has a version with `effective_from <= now`;
  - it has an open assignment whose scope contains the point.
- DRAFT and INACTIVE profiles never price. Simulate can preview them (S3).
- The default:
  - it is always ACTIVE, has no window and is complete (every field set);
  - it cannot be deactivated or unassigned, only replaced through make-default;
  - make-default writes `MADE_DEFAULT` on the new profile and `DEFAULT_REPLACED` on the old one.
- **Versions are immutable.** Editing means publishing version n+1 with an optional future `effective_from`.
- The cache keeps every version whose `effective_from` falls within the past 1 version and the future 90 days. The resolver picks the version in force **at quote time** in memory, which fixes the reload bug for future-dated versions.
- A River job `pricing.version_tick` runs every minute. It bumps nothing; it only logs `PUBLISHED`-effective transitions as `ProfileEvent{kind: PUBLISHED, data: {in_force: true}}` once, so history shows when a version actually took effect.

### Resolution (`profiles.Resolve`, pure, no I/O)
**Input:** `country`, `pickup`, `dropoff?`, `route polyline?`, `service`, `class`, `fixedRoute bool`, `at`, and `pickupArea` (already resolved by `areas` in `pricing.resolveArea`).

1. Collect the candidate scopes containing the pickup:
   - COUNTRY (always);
   - the pickup area (AREA);
   - in S2, every geofence whose index entry contains the point (DISTRICT, ZONE, PLACE).
2. For each scope, take its open assignment and profile, then mark the outcome:
   - DRAFT;
   - INACTIVE;
   - OUT_OF_WINDOW;
   - NO_VERSION_IN_FORCE;
   - otherwise eligible.
3. Rank the eligible candidates by `(precedence(kind), priority, -valid_from)` descending. The winner is first; the others are `LOWER_PRECEDENCE`.
   - Two eligible candidates with equal kind and priority should be impossible because of save-time validation. If one happens (for example, a geometry was edited by a migration), the older assignment wins, the other is marked `TIE_BROKEN`, and a WARN log plus the metric `pricing_profile_tie_total` are emitted.
4. **Terms** are `Merge(defaultVersion, winnerVersion)`: each null field of the winner takes the default's value. Country-only fields always come from the default.
5. **Charges** go through `ApplyCharges`, sources in order (a charge `code` is applied **at most once** per trip; the first source wins):
   - the winner's rules with PICKUP, EITHER or BOTH. BOTH also needs the drop-off to resolve to the same profile.
   - (S2) the drop-off winner's rules (the same procedure at the drop-off point) with DROPOFF or EITHER. A COUNTRY or AREA winner at the drop-off contributes too, which is how "every trip into the airport place pays the entry fee" works.
   - (S2) ROUTE_CROSSES rules from every eligible ZONE-scoped profile whose zone the quoted route crosses.
6. The trace lists every candidate at the pickup (and the drop-off in S2) with its outcome, the winner rule, and the default's version.

**Filters** in `ApplyCharges`:
- `services` and `vehicle_classes` (empty means all);
- `on_fixed_routes` (skipped when the option is a fixed fare and the flag is false);
- `days`, `from_time` and `to_time`, evaluated at the quote time in the country timezone (`countries.timezone`). A window may wrap past midnight: 22:00–05:00 matches 23:30 and 04:59.

**Amounts:**
- FLAT: `amount`.
- PER_KM: `amount × route_distance_m / 1000`, using the quoted road distance.
- PERCENT_OF_FARE: `amount × fareBase`. `fareBase` is the option's fare after surge and minimum, before the booking fee, the charges and rounding.
- Then the result is clamped to `[min_amount, max_amount]` and rounded to 2 dp. A zero amount is skipped.

### Quote (`pricing.Service.Quote`)
- After the area is resolved: `res := s.profiles.Resolve(ctx, in)` reads the profiles cache and the geofence index; **0 Postgres queries**.
- The `booking_fee`, `fare_rounding`, `waiting_cap_rate` and cancellation terms now come from `res.Terms`, replacing `tariffs.policy`.
- Per class, the charges become `engine.Line{Code: "CHARGE", Label: {en, sw}, Amount, Meta: {charge, category, recipient}}`, appended **after** the engine or fixed-route lines and before `rounded()`.
  - `engine.Line` gains `Label map[string]string \`json:"label,omitempty"\``.
  - The ROUNDING line is computed on the full total, as today, and counts as fare.
  - `WaitingCap` is computed on the fare **without** the charges.
- `saveQuote` writes `quotes.pricing = {trace, profile, default, config, per_class: {class: {terms, charges}}}`.

### Pin, split and finalise
- **New trips hook:** `trips.Service.OnInsertedTx(h InsertedTxHook)`, where `InsertedTxHook func(ctx, tx pgx.Tx, tripID, quoteID, vehicleClass string) error`.
  - It runs right after `store.insert` and **before** `storeFees`. A failure aborts the booking.
  - Wiring: `profiles.Records.PinTx` makes one statement, `INSERT INTO trip_pricing_records … SELECT … FROM quotes WHERE id=$quote` (the class slice of `quotes.pricing`). It sets `source='LIVE'`.
- **`trips.FeeFunc` changes** to `func(ctx, tx pgx.Tx, tripID string, total, bookingFee decimal.Decimal, currency string) (json.RawMessage, error)`.
  - `storeFees` and `fixSplitTx` pass their transaction.
  - The new `feeSplitter` in `cmd/nexa/wire_pricing.go` (moved out of `cmd_serve.go`) reads `terms` and `charges` from `trip_pricing_records` by primary key in that transaction. It builds a `money.Policy` (with `ProfileID`, `ProfileVersionID`, `Version`, `DefaultVersionID`) and the `[]money.ChargeShare`, then calls `money.Split`.
  - **The policy in force is never read.** A trip without a record (booked before deploy and still open) falls back once to the trip's country/area default, with the record written as `source='FALLBACK'` and a WARN log.
- **`money.Split(total, bookingFee, currency, p Policy, charges []ChargeShare)`**:
  - `ChargeShare {Code, Amount, Recipient, PayeeID, Taxable}`.
  - `carved` = the sum of the PLATFORM, DRIVER and THIRD_PARTY amounts. FARE charges stay inside the ride fare and are split like it, including commission and levy.
  - The existing algorithm runs on `total − carved`. VAT on GROSS_FARE uses that same base.
  - Then for each charge:
    - PLATFORM: `vat = taxable ? extract(amount, VATRate) : 0`; `PlatformNet += amount − vat`; `VAT += vat`.
    - DRIVER: `DriverEarnings += amount`.
    - THIRD_PARTY: `ThirdParty += amount`, with a per-payee entry in `Breakdown.Charges`.
  - Invariant, checked on every call: `DriverEarnings + PlatformNet + VAT + Levy + ThirdParty == Total`, and no part is negative. If `carved > total − bookingFee`, it returns an error.
  - `Breakdown` gains `profile_id, profile_version_id, default_version_id, charges [{code, recipient, payee_id?, amount, vat}], third_party`. `policy_version` is kept and equals the profile version.
- **Ledger** (`internal/money/ledger.go`, `fare.go`): the new constant `ChargesPayable = "CHARGES_PAYABLE"`.
  - `PostCashTripCompletedTx`: `owed = Total − DriverEarnings` (as today, now including third-party money). It adds one credit leg `{ChargesPayable, OwnerID: payee, −amount}` per THIRD_PARTY charge.
  - `PostTripPaidTx`: the same third-party legs.
  - Both stay balanced, and the hash chain is unchanged.
  - The `money/admin.go` finance summary adds `charges_payable` as one grouped query over `ledger_accounts` of that kind.
- **Finalise:** `tripSvc.OnCompletedTx(records.FinalizeTx)` sets `stage='FINAL'`, `final={breakdown: trips.fees, completed_at}` and `record_hash = sha256(canonical(record))`, in the completing transaction, alongside the receipt.
  - Cancelled or expired trips keep their QUOTED record. That record is just as immutable, because nothing else updates it.
  - A cancellation fee uses the snapshotted option terms, unchanged.
- **Receipts:** `receipts.Build` labels a line from `line.Label` when it is present; otherwise from `labelFor(code)`. FactLine gains `Label`. The PDF shows the charge label in the receipt's language. `Taxes` is unchanged; the third-party amounts are inside the visible charge lines.

### Validation on save (`profiles.Service`)
- Every write runs in one transaction under `pg_advisory_xact_lock(hash(brand, country, 'pricing'))`. It bumps `config_versions('pricing')`, appends a `pricing_profile_events` row and calls `audit.Record`.
- **Ties.** Activating, assigning, changing a priority or a window, or (S2) changing geometry finds the other open assignments that meet all of these:
  - same country;
  - same `scope_kind`;
  - same priority;
  - profile ACTIVE, or the profile being activated;
  - overlapping activation windows;
  - overlapping geometry (S2: `ST_Intersects` for polygons, `ST_DWithin` for places; areas never overlap).
  Any match returns 409 `PROFILE_PRECEDENCE_TIE {conflicts: [{profile, scope}]}`. Overlap across kinds is allowed, because precedence decides it.
- **Charges.**
  - Payee rules: a payee is required for THIRD_PARTY and must be ACTIVE and in the same country; `taxable` only for PLATFORM.
  - PERCENT amounts must be ≤ 1, and `min ≤ max`.
  - Class and service keys must exist in the catalogue (`catalog.Find`).
  - Labels need both `en` and `sw`, each 1–40 characters.
  - At most 20 charges per version.
  - S1 allows only PICKUP and EITHER.
- **Version terms.**
  - Rates must be in `[0, 1)` (the existing `validatePolicy`) and debt limits are checked (`validateDebtLimits`) on the default.
  - A non-default version must leave the country-only fields null (`POLICY_FIELD_COUNTRY_ONLY`).
  - The default must have every field set (`PROFILE_INCOMPLETE`).
  - `content_hash` is computed in Go from the canonical JSON of terms and charges.

### Geofences (S2)
- **Save path** (staff only; Postgres and PostGIS are fine here).
  - Parse the GeoJSON in `geometry.go`, which checks the type, closed rings, at least 4 points and lng/lat ranges.
  - Then in SQL:
    - `ST_IsValid(geom::geometry)`, with `ST_IsValidReason` mapped to SELF_INTERSECTION;
    - `ST_NPoints` ≤ `MaxVertices`;
    - `ST_Area(geom)` within `[MinAreaM2, MaxAreaKm2[kind]]`;
    - a radius range check for places.
  - The centroid's res-7 cell must not belong to another country's area. A geofence outside every area of its country is allowed (drop-offs can be there) and shown as a warning.
  - Insert the version, update the row, bump `config_versions('geofences')`, and audit.
- **Index** (`geofences.index.go`, rebuilt on a `geofences` version change, `checkTTL 5s`):
  - `cellCandidates map[h3.Cell][]geofenceID`. A zone's cover is `PolygonToCells(res 7)` plus a `GridDisk(cell, 1)` around every boundary vertex's cell: a conservative superset. A place's cover is `GridDisk(CellOf(center), ceil(radius_m / 1220) + 1)`.
  - Exact tests run in memory: even-odd point-in-polygon on lng/lat for zones (holes respected), haversine distance ≤ radius for places.
  - `Contains(p) []Hit` costs O(candidates).
  - `Crossed(polyline) []geofenceID` decodes the quoted polyline (≤ `MaxRoutePoints`). It tests only zones whose bbox meets the route bbox, using segment–edge intersection or an endpoint inside. Without a polyline (route provider "none"), no ROUTE_CROSSES charge applies.
- Archiving or deactivating a geofence with an open assignment is refused. INACTIVE geofences drop out of the index.
- Assignments to a geofence keep their own validity. Records store the geofence id **and version**.

### History and "as of" (S3)
- `profiles.store.SnapshotAt(ctx, country, at)` reads, as of `at`:
  - assignments where `valid_from ≤ at < coalesce(valid_to, ∞)`;
  - the profiles with their status at `at`, replayed from `pricing_profile_events` (ACTIVATED and DEACTIVATED) and the window fields current at `at` (WINDOW_CHANGED events carry the old and new values);
  - the versions with `effective_from ≤ at`;
  - the geofence versions current at `at`;
  - today's area index.
- It builds the same `Snapshot` and runs the same `Resolve`, so there is one code path. `approximate = true` when any AREA candidate's `areas.updated_at > at`.
- The simulator calls `pricing.Service.Preview(ctx, req, PreviewOptions{At, PreviewProfileID})`, which is `Quote` without `saveQuote`.

### Jobs
- `pricing.version_tick`: every 1 minute; logs versions coming into force.
- No other new jobs.

### Performance (definition of done)
- `POST /quotes`: p95 and queries before and after are recorded. **Profile and geofence resolution add 0 queries** (asserted with a query-counting pool in `pricing` tests).
- `POST /trips`: +1 statement (pin).
- AWAITING_PAYMENT: +1 read (terms by primary key).
- COMPLETED: +1 update (finalise).
- Admin list endpoints: one query each, with `ANY($1)` for assignments and counts, no N+1.
- EXPLAIN is recorded for the as-of query and the CSV query.

## 6. Apps and admin
**Rider app:**
- `components/taxi/FareBreakdown.tsx`: `FareLineView` gains `label?: {en: string; sw: string}`, and the shown label is `l.label?.en ?? LABEL[l.code] ?? l.code`. The app is EN-only until the i18n slice.
- Replace the literal "Includes LATRA levy" with "Includes levy", because the receipt already uses generic regulator wording.
- `models/receipt.ts` is unchanged (lines pass through).

**Driver app:** no change. Pass-through charges show up in the earnings the server already reports.

**Admin.** All screens use the existing `@/components/ui` primitives (Card, Table, Chip, Tabs, PageHeader, ReasonDialog, Field, Button), theme tokens only, the 10px radius, the `motion.ts` eased entrances (no springs), and a reason dialog on every write. Profiles get separate pages, not drawers (owner preference). New files:
- `features/pricing/profiles/`:
  - `ProfilesScreen.tsx`: a data table of name, where it applies, status, version, charges, in force since and last change. The default row is pinned first with a "Default" chip. INACTIVE and DRAFT rows are muted. There is a "New profile" button.
  - `ProfileScreen.tsx`: the header has a status chip and the Activate/Deactivate, Make default and Publish changes actions. Tabs:
    - **Rules**: `TermsForm.tsx`, which grows out of `PolicyCard.tsx`. On a non-default profile each field has an "Use default (x)" toggle, and the country-only fields are read-only.
    - **Extra charges**: `ChargesTable.tsx` and `ChargeForm.tsx`. Charges are edited as a draft list, then published as a new version.
    - **Where it applies**: `ScopesCard.tsx`, for assigning to a region (S2: district, zone or place pickers) and setting the priority.
    - **Versions**: `VersionsTable.tsx`, with "Compare" opening `VersionDiff.tsx`, a client-side field and charge diff of two versions.
    - **Activity**: `EventsTable.tsx`.
  - `NewProfileScreen.tsx`.
  - `PayeesCard.tsx`, shown as a "Payees" tab on the list page.
  - `hooks.ts`.
- `features/trips/TripPricingRecord.tsx`: a "Pricing record" card on `TripDetail.tsx`. It shows:
  - the profile name and version (a link), the scope, and the default version;
  - a charges table (label, recipient, payee, amount, VAT);
  - a "Why this profile" collapsible trace;
  - a stage chip (Quoted or Final), the short record hash, and "Download JSON".
  - `TripDetail.tsx` `LINE_LABEL` falls back to `line.label.en`.
- `features/finance`: an "Owed to third parties" tile and a per-payee table.
- S2:
  - `features/geofences/`:
    - `GeofencesScreen.tsx`: a list beside the map, with filters for kind and status and an overlap banner;
    - `GeofenceScreen.tsx`: the page per geofence, with geometry editing and versions;
    - `GeofenceMap.tsx`;
    - `usePolygonEditor.ts`: click to add vertices, drag vertices, close the ring, undo;
    - `usePlaceCircle.ts`;
    - `ImportDialog.tsx`: file pick, dry-run table, apply;
    - `hooks.ts`.
  - They use `@/components/map/useMap` with `follow=false` and area outlines from `areaLayers.ts`.
  - Route `/pricing/geofences`. `nav.ts` adds "Geofences" (lucide `MapPinned`) under Pricing, and `isActive` excludes `/pricing/geofences` from "Vehicle types".
- S3: `/pricing/fees/simulate` is `SimulateScreen.tsx`, a map to drop the pickup and drop-off, plus service, class and "as of". It shows the winner, the trace, the charge lines and the split per class. `/pricing/fees/history` is `HistoryScreen.tsx`, a timeline per scope.

**Copy (EN / SW):**

| Key | EN | SW |
|---|---|---|
| nav.fees | Pricing profiles | Wasifu wa bei |
| nav.geofences | Geofences | Maeneo maalum |
| profile.default | Default | Chaguo-msingi |
| profile.status.ACTIVE / INACTIVE / DRAFT | Active / Off / Draft | Hai / Imezimwa / Rasimu |
| profile.make_default | Make default | Weka kuwa chaguo-msingi |
| profile.inherit | Use default ({value}) | Tumia chaguo-msingi ({value}) |
| profile.publish | Publish changes | Chapisha mabadiliko |
| profile.deactivate.confirm | New trips will stop using this profile. Trips already priced keep it. | Safari mpya hazitatumia wasifu huu. Safari zilizokwisha kupewa bei zinaubaki nao. |
| charges.title | Extra charges | Tozo za ziada |
| charges.recipient.FARE / PLATFORM / DRIVER / THIRD_PARTY | Part of the fare / Platform / Driver keeps it / Paid to {payee} | Sehemu ya nauli / Jukwaa / Dereva anabaki nayo / Inalipwa kwa {payee} |
| charges.when.PICKUP / DROPOFF / EITHER / BOTH / ROUTE_CROSSES | Pickup here / Drop-off here / Pickup or drop-off here / Pickup and drop-off here / Route passes through | Kuchukuliwa hapa / Kushushwa hapa / Kuchukuliwa au kushushwa hapa / Kuchukuliwa na kushushwa hapa / Njia inapita hapa |
| record.title | Pricing record | Kumbukumbu ya bei |
| record.why | Why this profile | Kwa nini wasifu huu |
| record.final | Final, cannot change | Imekamilika, haibadiliki |
| geofence.kind.DISTRICT / ZONE / PLACE | District / Zone / Place | Wilaya / Eneo / Mahali |
| geofence.overlap | {a} overlaps {b} with the same priority. | {a} inaingiliana na {b} kwa kipaumbele sawa. |
| simulate.title | Simulate a trip | Jaribu safari |

## 7. Tests
**Go, S1** (`go test ./...`; the DB tests need `NEXA_TEST_DATABASE_URL`):
- `profiles/resolve_test.go` (table): the country default wins alone; an area beats the default; INACTIVE, DRAFT, out-of-window and future-version profiles are skipped with the right outcome; the default's version is in the trace; a forced tie is broken by the older assignment and marked.
- `profiles/charges_test.go`:
  - FLAT, PER_KM and PERCENT with min and max;
  - a zero amount is skipped;
  - class and service filters;
  - a fixed fare with `on_fixed_routes=false` is skipped;
  - a night window wrapping midnight, evaluated in Africa/Dar_es_Salaam and in Africa/Nairobi;
  - a duplicate code from two sources is applied once.
- `profiles/terms_test.go`: Merge inherits nulls, the country-only fields come from the default, and validation produces each error.
- `money/split_test.go`:
  - each recipient's arithmetic;
  - a taxable PLATFORM charge extracts VAT;
  - the GROSS_FARE base excludes carved charges;
  - carved more than the fare gives an error;
  - a property test (10,000 random cases): the parts always sum to the total and none is negative.
- `money/ledger_test.go` (DB): cash and mobile-money trips with a THIRD_PARTY charge post balanced transactions with `CHARGES_PAYABLE` per payee, and `Verify` passes.
- `profiles/records_test.go` (DB):
  - PinTx copies the class slice;
  - FinalizeTx sets the hash once;
  - a second finalise, any `UPDATE` of `terms`, and `DELETE` are each refused by the trigger.
- `profiles/service_test.go` (DB):
  - publish writes a version, an event, an audit row and a config bump;
  - a non-default profile setting VAT gives 409;
  - make-default is atomic (two concurrent calls: one wins and exactly one COUNTRY assignment stays open);
  - deactivating or unassigning the default gives 409;
  - ties give 409;
  - assignment close-only (an update of `profile_id` is refused by the trigger).
- `trips` and `pricing` (DB), **the core audit test**:
  1. Quote and book in a region with its own profile (commission 0.15).
  2. Publish v2 (commission 0.20) and deactivate the profile.
  3. Finish the ride. The split uses 0.15, and the record shows v1 and the area scope.
  4. A new quote uses the default.
- Also: a trip in a second country is split with that country's default, which is the regression for the `feeSplitter` bug.
- `pricing/quote_test.go`:
  - the CHARGE line has a label and sits before ROUNDING;
  - the waiting cap excludes charges;
  - the quote makes no extra queries (counting pool).
- Migration test: 00035 up, down and up. Each `fee_policies` scope becomes one profile with equal version counts, and backfilled records match `trips.fees`.
- `receipts`: a CHARGE line uses its EN/SW label; the lines sum still holds.
- `httpx`: every new route is declared with security; `fees.edit`, `areas.edit` and `finance.view` are enforced; a region-scoped staff member gets 403 on profile writes; every error code is in both i18n files.

**Go, S2:**
- `geofences/geometry_test.go`: an open ring, too few points, bad coordinate ranges, a MultiPolygon with holes.
- `geofences/store_test.go` (DB): a bow-tie polygon gives SELF_INTERSECTION; 2,001 vertices gives TOO_MANY_VERTICES; size limits per kind; place radius bounds; another country gives 409; versions are appended.
- `geofences/index_test.go`:
  - a point inside the polygon but in a boundary cell is a hit;
  - a point in a candidate cell but outside is not;
  - a hole is excluded;
  - place haversine at radius ±1 m;
  - a route crossing a narrow zone (a 60 m wide strip) is detected;
  - a route ending before the zone is not.
- `profiles/resolve_test.go` (S2 cases): PLACE beats ZONE beats DISTRICT beats AREA; a priority within a kind; the drop-off winner contributes DROPOFF charges; BOTH needs the same profile; ROUTE_CROSSES from a crossed zone; no polyline means no crossing charge.
- Import: dry run reports per feature; apply is all-or-nothing.

**Go, S3:**
- `as_of_test.go` (DB):
  1. Build a history (assign at T1, publish v2 at T2, deactivate at T3, replace the default at T4).
  2. `as-of` at points between each change returns the profile and version that applied then.
  3. A changed area gives `approximate=true`.
- Simulate persists nothing (the quote count is unchanged). Preview of a DRAFT profile wins in the response only.
- The CSV has the header and a row per final record, and a range over 31 days gives 400.

**E2E** (each prints PASS lines like `e2e-fares.mjs`):
- `apps/tools/e2e-pricing-profiles.mjs` (S1):
  1. Create the payee "Test authority".
  2. Create profile "E2E region" for Dar es Salaam with a THIRD_PARTY `E2E_LEVY` 500 FLAT EITHER and a DRIVER `E2E_PARKING` 300 PICKUP. Publish and activate it.
  3. Quote: the CHARGE lines are labelled and the total includes 800.
  4. Book, ride and confirm cash. The record is FINAL with profile v1, the AREA scope and both charges.
  5. Ledger: `CHARGES_PAYABLE` +500 for the payee, and the driver's earnings include 300. `nexa verify-ledger` passes.
  6. Publish v2 (levy 900). The old record is unchanged and its ETag is stable.
  7. A `psql` `UPDATE trip_pricing_records` fails.
  8. Deactivate. A new quote has no charge lines.
  9. Deactivating the default gives 409. A tie gives 409.
  10. Clean up by deactivating with a reason.
- `apps/tools/e2e-geofences.mjs` (S2):
  1. Create a place "E2E airport" (radius 800 m) and a ZONE strip across the test route.
  2. Assign profiles with DROPOFF `E2E_ENTRY` and ROUTE_CROSSES `E2E_TOLL`.
  3. A quote to the place shows the entry charge, and a route through the strip shows the toll.
  4. Import 3 features in a dry run (one invalid) shows the per-feature outcomes.
  5. Overlaps with the same priority give 409.
- `apps/tools/e2e-pricing-history.mjs` (S3): `as-of` before and after the changes made above; simulate with a draft preview; CSV download.
- `e2e-admin.mjs`: extended to open `/pricing/fees`, a profile page, the trip "Pricing record", and `/pricing/geofences`.
- `shot-pricing-profiles.mjs`: screenshots of the list, the profile tabs, the trip record, the geofence editor and the simulator.

## 8. Acceptance
- [ ] 00035, 00036 and 00037 apply, roll back and re-apply. Every `fee_policies` scope is a profile with the same versions. `fee_policies_legacy` is read-only. Completed trips have backfilled FINAL records.
- [ ] Exactly one open default per country, enforced by the database. The default cannot be deactivated, windowed or unassigned, only replaced, and the replacement is atomic and audited.
- [ ] DRAFT, INACTIVE, out-of-window and not-yet-effective profiles never price a quote (tests prove each case).
- [ ] Precedence PLACE > ZONE > DISTRICT > AREA > COUNTRY, then priority. Ties are refused at save time.
- [ ] Charges: three amount types, four recipients, five conditions (S1: two), class and service filters, a fixed-fare flag and a time window. They show on the quote, the receipt (EN/SW) and the admin with their labels.
- [ ] One `Split()`. Its invariant includes the third-party amounts. THIRD_PARTY money sits in `CHARGES_PAYABLE` per payee, the ledger verifies, and Finance sees "Owed to third parties".
- [ ] **A trip is split on its pinned terms.** Publishing or deactivating after booking does not change it, and a second-country trip uses its own country's default (the `feeSplitter` bug is fixed).
- [ ] Every trip booked after deploy has a pricing record (profile, version, scope, default version, trace, charges, config). The trigger refuses any change after FINAL and any delete. The trip page shows it, with a JSON download.
- [ ] Profile changes write events and audit rows. The history shows who made each one, when, and why.
- [ ] Geofences: draw, edit (versioned), place circles, GeoJSON import with dry run, overlap report. Invalid shapes are refused with a reason.
- [ ] `as-of` answers "what priced point X at time T" from history. The simulator previews without persisting. The CSV export works.
- [ ] Quote resolution adds 0 Postgres queries. p95 and queries per request are recorded for `POST /quotes`, `POST /trips`, the AWAITING_PAYMENT and COMPLETED transitions, and the admin lists. EXPLAIN is recorded for as-of and the CSV.
- [ ] OpenAPI declares the routes with security, and both clients are regenerated. The `FareLine` enum is fixed. The i18n codes are in EN and SW.
- [ ] `gofmt`, `go vet`, `golangci-lint run ./...`, Biome, `tsc` and the brand grep pass. There is a `doc.go` in `profiles` and `geofences`. No literal colours, brand names or country names in code (the "LATRA levy" literal is removed from the rider app).
- [ ] D12 checks hold:
  - #15: one Split;
  - #17: rates in versioned profiles pinned per trip;
  - #6: GeoJSON lng/lat order;
  - #7: stable codes;
  - #13: routes declared;
  - #30: trips knows nothing about profiles (hooks only).
- [ ] `DOC/adr/0010-pricing-profiles.md` is written (profiles replace fee policies; districts are geofences). `DOC/04-pricing-modules.md` §1 is updated with the profile and charge layer. `DOC/06-taxi-roadmap.md` is updated.

## 9. Open questions for the owner (defaults in use)
1. **Toll when only passing through.** Default: charged when the **quoted** road route crosses a toll zone (S2), fixed at the quote like the fare. If the driver takes another road, nothing is re-priced. The alternative is a driver-entered toll with a photo, after the ride.
2. **Parking and tolls.** Default recipient is DRIVER: a pass-through with no commission, because the driver pays the attendant or the booth in cash.
3. **Levies and authority fees** (airport entry, city levy). Default recipient is THIRD_PARTY: owed to the named payee in `CHARGES_PAYABLE`, not subject to VAT, and paid by Finance outside the app.
4. **Commission on charges.** Default: none, except charges marked "Part of the fare" (for example a night surcharge), which are split like the fare.
5. **VAT on platform charges.** Default: taxable (VAT included in the amount).
6. **Fixed fares** (town–airport). Default: charges apply on fixed fares too, unless the charge is unticked. Do the current airport fixed fares already include the airport fee?
7. **Real values.** No charges or payees are seeded. Please give each toll, parking, airport and entry fee with its amount, its place, who receives it and the payee's reference (for example TIN).
8. **Who may activate a profile or make it the default.** Default: anyone with `fees.edit`, with a reason and an audit entry. Do you want a second approver (four-eyes) for activation and default changes?
9. **Districts.** They are modelled as geofences (drawn or imported boundaries), not as sub-areas of regions. Confirm, and say which district boundaries to import first.
10. **Drop-off charges when the ride ends elsewhere.** They are charged as quoted. Confirm.
11. **Record retention.** Pricing records are kept as long as trips, receipts and the ledger (at least 5 years, as tax records, per the compliance spec's retention default).
12. **Debt limits and cash-out rules per country.** Today they follow the home country for every driver. Should fixing that be scheduled next (separate small slice)?

## Factory log
- 2026-10-23: spec written.
  - Code findings recorded above. The main one: `feeSplitter` splits every trip on the home country's policy in force at that moment, so fees are not pinned per trip and area policies are ignored at settlement. S1 fixes it.
  - Migrations are numbered from 00035 because the branding build holds 00034.
  - The compliance spec's S3 and S4 must take the next free numbers.
  - Slices queued: S1 (profiles, charges, record) → S2 (geofences) → S3 (history, as-of, simulator).
- 2026-10-23: **S1 built** (migration 00035): `internal/profiles` (profiles, immutable versions, charges, payees, assignments, events, pure resolver, records), `money.Split` with carved charges and `CHARGES_PAYABLE`, quotes pin per-class terms (`quotes.pricing`), booking pins `trip_pricing_records`, the split reads only the pin (fallback pin marked FALLBACK), completion seals the record (SQL hash over the stored columns; `seal_valid` on read). Fees page is now a profile list + profile page (Rules, Extra charges, Where it applies, Versions with compare, Activity) and Payees; trip page has the Pricing record card; Finance has "Owed to third parties". Fee lines in apps carry labels; riders and drivers get a fee view without profiles, payees or platform net.
- 2026-10-23: **reviews on S1.** Security: cross-country publish through an area id (fixed: area must be in the country), payee update saved before the scope check (fixed), riders received the full split (fixed: `FeesView`), unreadable charges ignored (fixed), rounding below carved charges (fixed: never below booking fee + carved), DB guards widened (00036). Code: migration joined regions by name (fixed: scope mapping), version order, re-seed overwrote staff rules (fixed: seed only creates a missing default, with events), record names and payee names pinned, SQL seal, bounded snapshot, error mapping, timezone cache, region-limited staff reading records, admin publish reset and percent typing fixed. Left: some SQL still sits in service files; `createRegionProfile` (old endpoint shim) runs four transactions after checks; package name `profiles` vs `profile`.
- 2026-10-23: **S2 built** (migration 00036): `internal/geofences` (PostGIS validation, versions, in-memory H3 index for points and route crossing, GeoJSON import with dry run, overlaps), profiles resolve districts/zones/places at the pickup and drop-off and zones the quoted road crosses (DROPOFF, BOTH, ROUTE_CROSSES), tie checks for overlapping shapes, admin Geofences page (draw, place + radius, import, per-geofence versions) and profile assignment to geofences. Tests: `e2e-pricing-profiles.mjs` (rates changed mid-ride, the trip keeps its pin; seal verifies), `e2e-geofences.mjs`, Go unit and DB tests for resolution, charges, split invariant (10 000 cases), geometry, index, record guards.
