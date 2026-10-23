-- +goose Up
-- Pricing profiles (spec 2026-10-23-pricing-profiles-geofences.md, S1).
-- A country has one default profile; other profiles are assigned to a region
-- (later: district, zone, place). A profile has immutable versions with the
-- trip money rules and extra charges (toll, parking, levy …) paid to the fare,
-- the platform, the driver or a third party. Every trip pins the profile
-- version it was quoted under in an immutable pricing record. fee_policies is
-- migrated into profiles and kept read-only as fee_policies_legacy.

-- Third parties a charge can be owed to (an airport, a city council, a regulator).
ALTER TYPE account_kind ADD VALUE IF NOT EXISTS 'CHARGES_PAYABLE';

CREATE TABLE charge_payees (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  brand_id     uuid NOT NULL REFERENCES brands(id),
  country_code char(2) NOT NULL,
  name         text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
  kind         text NOT NULL CHECK (kind IN ('TAX_AUTHORITY','REGULATOR','AIRPORT','MUNICIPALITY','PARKING_OPERATOR','OTHER')),
  reference    text CHECK (char_length(reference) <= 80),
  status       text NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','INACTIVE')),
  created_by   uuid REFERENCES users(id),
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  UNIQUE (brand_id, country_code, name)
);

CREATE TABLE pricing_profiles (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  brand_id     uuid NOT NULL REFERENCES brands(id),
  country_code char(2) NOT NULL,
  name         text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
  description  text CHECK (char_length(description) <= 500),
  status       text NOT NULL DEFAULT 'DRAFT' CHECK (status IN ('DRAFT','ACTIVE','INACTIVE')),
  active_from  timestamptz,
  active_until timestamptz,
  created_by   uuid REFERENCES users(id),
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  CHECK (active_until IS NULL OR active_from IS NULL OR active_until > active_from),
  UNIQUE (brand_id, country_code, name)
);
CREATE INDEX pricing_profiles_scope_idx ON pricing_profiles (brand_id, country_code, status);

-- Trip-rule columns are NULL on a non-default profile when it inherits the
-- default's value; country-only columns are NULL on every non-default profile.
CREATE TABLE pricing_profile_versions (
  id                   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  profile_id           uuid NOT NULL REFERENCES pricing_profiles(id),
  version              int NOT NULL,
  effective_from       timestamptz NOT NULL DEFAULT now(),
  commission_rate      numeric(6,4) CHECK (commission_rate >= 0 AND commission_rate < 1),
  booking_fee          numeric(14,2) CHECK (booking_fee >= 0),
  waiting_cap_rate     numeric(6,4) CHECK (waiting_cap_rate >= 0 AND waiting_cap_rate < 1),
  fare_rounding        numeric(14,2) CHECK (fare_rounding >= 0),
  cancel_free_s        int CHECK (cancel_free_s >= 0),
  no_show_after_s      int CHECK (no_show_after_s >= 0),
  cancel_driver_share  numeric(6,4) CHECK (cancel_driver_share >= 0 AND cancel_driver_share <= 1),
  vat_rate             numeric(6,4) CHECK (vat_rate >= 0 AND vat_rate < 1),
  vat_base             text CHECK (vat_base IN ('COMMISSION','GROSS_FARE')),
  levy_rate            numeric(6,4) CHECK (levy_rate >= 0 AND levy_rate < 1),
  levy_borne_by        text CHECK (levy_borne_by IN ('PLATFORM','DRIVER','RIDER')),
  debt_soft_limit      numeric(14,2),
  debt_hard_limit      numeric(14,2),
  debt_soft_grace_days int CHECK (debt_soft_grace_days >= 0),
  payin_min_amount     numeric(14,2) CHECK (payin_min_amount > 0),
  payout_min_amount    numeric(14,2) CHECK (payout_min_amount > 0),
  payout_auto_approve  boolean,
  content_hash         text NOT NULL,
  published_by         uuid REFERENCES users(id),
  reason               text NOT NULL,
  legacy_fee_policy_id uuid,
  created_at           timestamptz NOT NULL DEFAULT now(),
  CHECK (debt_soft_limit IS NULL OR payin_min_amount IS NULL OR debt_soft_limit >= payin_min_amount),
  CHECK (debt_hard_limit IS NULL OR debt_soft_limit IS NULL OR debt_hard_limit >= debt_soft_limit),
  UNIQUE (profile_id, version)
);
CREATE INDEX pricing_profile_versions_eff_idx ON pricing_profile_versions (profile_id, effective_from DESC);

CREATE TABLE pricing_profile_charges (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  version_id      uuid NOT NULL REFERENCES pricing_profile_versions(id),
  code            text NOT NULL CHECK (code ~ '^[A-Z][A-Z0-9_]{1,31}$'),
  label           jsonb NOT NULL CHECK (label ? 'en' AND label ? 'sw'),
  category        text NOT NULL CHECK (category IN ('TOLL','PARKING','LEVY','AIRPORT_FEE','ENTRY_FEE','NIGHT','OTHER')),
  amount_type     text NOT NULL CHECK (amount_type IN ('FLAT','PER_KM','PERCENT_OF_FARE')),
  amount          numeric(14,4) NOT NULL CHECK (amount >= 0),
  min_amount      numeric(14,2) CHECK (min_amount >= 0),
  max_amount      numeric(14,2) CHECK (max_amount >= 0),
  recipient       text NOT NULL CHECK (recipient IN ('FARE','PLATFORM','DRIVER','THIRD_PARTY')),
  payee_id        uuid REFERENCES charge_payees(id),
  taxable         boolean NOT NULL DEFAULT false,
  applies_when    text NOT NULL CHECK (applies_when IN ('PICKUP','DROPOFF','EITHER','BOTH','ROUTE_CROSSES')),
  services        text[] NOT NULL DEFAULT '{}',
  vehicle_classes text[] NOT NULL DEFAULT '{}',
  on_fixed_routes boolean NOT NULL DEFAULT true,
  days            smallint[] NOT NULL DEFAULT '{}',
  from_minute     smallint CHECK (from_minute BETWEEN 0 AND 1439),
  to_minute       smallint CHECK (to_minute BETWEEN 0 AND 1439),
  sort_order      int NOT NULL DEFAULT 0,
  CHECK (max_amount IS NULL OR min_amount IS NULL OR max_amount >= min_amount),
  CHECK ((recipient = 'THIRD_PARTY') = (payee_id IS NOT NULL)),
  CHECK (NOT taxable OR recipient = 'PLATFORM'),
  CHECK (amount_type <> 'PERCENT_OF_FARE' OR amount <= 1),
  CHECK ((from_minute IS NULL) = (to_minute IS NULL)),
  UNIQUE (version_id, code)
);

-- Which profile applies where, and from when to when. One open assignment per
-- scope, so exactly one open COUNTRY default per country. Rows are closed, never edited or deleted.
CREATE TABLE pricing_profile_assignments (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  brand_id     uuid NOT NULL REFERENCES brands(id),
  country_code char(2) NOT NULL,
  profile_id   uuid NOT NULL REFERENCES pricing_profiles(id),
  scope_kind   text NOT NULL CHECK (scope_kind IN ('COUNTRY','AREA','DISTRICT','ZONE','PLACE')),
  scope_id     text NOT NULL,
  priority     int NOT NULL DEFAULT 0 CHECK (priority BETWEEN -1000 AND 1000),
  valid_from   timestamptz NOT NULL DEFAULT now(),
  valid_to     timestamptz,
  assigned_by  uuid REFERENCES users(id),
  reason       text NOT NULL,
  closed_by    uuid REFERENCES users(id),
  close_reason text,
  CHECK (valid_to IS NULL OR valid_to >= valid_from)
);
CREATE UNIQUE INDEX pricing_assign_open_uq ON pricing_profile_assignments (brand_id, scope_kind, scope_id) WHERE valid_to IS NULL;
CREATE INDEX pricing_assign_profile_idx ON pricing_profile_assignments (profile_id, valid_from DESC);
CREATE INDEX pricing_assign_asof_idx ON pricing_profile_assignments (brand_id, country_code, valid_from, valid_to);

CREATE TABLE pricing_profile_events (
  id            bigserial PRIMARY KEY,
  brand_id      uuid NOT NULL REFERENCES brands(id),
  profile_id    uuid NOT NULL REFERENCES pricing_profiles(id),
  kind          text NOT NULL,
  version       int,
  assignment_id uuid,
  actor_id      uuid REFERENCES users(id),
  reason        text,
  data          jsonb NOT NULL DEFAULT '{}',
  at            timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX pricing_profile_events_idx ON pricing_profile_events (profile_id, at DESC);

-- The per-class pinned terms of a quote, so booking copies exactly what was offered.
ALTER TABLE quotes ADD COLUMN IF NOT EXISTS pricing jsonb;

-- One immutable record per trip of the profile it was priced under.
CREATE TABLE trip_pricing_records (
  trip_id            uuid PRIMARY KEY REFERENCES trips(id) ON DELETE RESTRICT,
  brand_id           uuid NOT NULL REFERENCES brands(id),
  quote_id           uuid,
  country_code       char(2),
  profile_id         uuid NOT NULL,
  profile_version_id uuid NOT NULL,
  profile_version    int NOT NULL,
  default_profile_id uuid,
  default_version_id uuid,
  scope_kind         text NOT NULL,
  scope_id           text NOT NULL,
  scope_name         text,
  terms              jsonb NOT NULL,
  charges            jsonb NOT NULL DEFAULT '[]',
  trace              jsonb NOT NULL,
  config             jsonb NOT NULL DEFAULT '{}',
  source             text NOT NULL CHECK (source IN ('LIVE','BACKFILL','FALLBACK')),
  stage              text NOT NULL DEFAULT 'QUOTED' CHECK (stage IN ('QUOTED','FINAL')),
  final              jsonb,
  record_hash        text,
  created_at         timestamptz NOT NULL DEFAULT now(),
  finalized_at       timestamptz
);
CREATE INDEX trip_pricing_records_profile_idx ON trip_pricing_records (profile_id, created_at DESC);
CREATE INDEX trip_pricing_records_final_idx ON trip_pricing_records (brand_id, country_code, finalized_at DESC) WHERE stage = 'FINAL';

-- Data: every fee_policies scope becomes a profile with the same versions.
-- A mapping keyed by scope (never by name: two regions may share a name).
CREATE TEMP TABLE profile_map ON COMMIT DROP AS
SELECT gen_random_uuid() AS profile_id, g.brand_id, g.country_code, g.area_id, g.first_at,
  CASE WHEN g.area_id IS NULL THEN 'Country default'
       WHEN count(*) OVER (PARTITION BY g.brand_id, g.country_code, a.name) > 1 OR a.name = 'Country default'
         THEN coalesce(a.name, 'Region') || ' (' || coalesce(a.code, left(g.area_id::text, 8)) || ')'
       ELSE coalesce(a.name, 'Region') END AS name
FROM (SELECT brand_id, country_code, area_id, min(created_at) AS first_at FROM fee_policies GROUP BY brand_id, country_code, area_id) g
LEFT JOIN areas a ON a.id = g.area_id;

INSERT INTO pricing_profiles (id, brand_id, country_code, name, status, created_at)
SELECT profile_id, brand_id, country_code, name, 'ACTIVE', first_at FROM profile_map;

-- Area profiles keep their trip rules (so they price exactly as before) and drop the copied country-only fields.
INSERT INTO pricing_profile_versions (profile_id, version, effective_from, commission_rate, booking_fee, waiting_cap_rate, fare_rounding,
  cancel_free_s, no_show_after_s, cancel_driver_share, vat_rate, vat_base, levy_rate, levy_borne_by, debt_soft_limit, debt_hard_limit,
  debt_soft_grace_days, payin_min_amount, payout_min_amount, payout_auto_approve, content_hash, reason, legacy_fee_policy_id, created_at)
SELECT m.profile_id, f.version, f.effective_from, f.commission_rate, f.booking_fee, f.waiting_cap_rate, f.fare_rounding,
  f.cancel_free_s, f.no_show_after_s, f.cancel_driver_share,
  CASE WHEN f.area_id IS NULL THEN f.vat_rate END, CASE WHEN f.area_id IS NULL THEN f.vat_base END,
  CASE WHEN f.area_id IS NULL THEN f.levy_rate END, CASE WHEN f.area_id IS NULL THEN f.levy_borne_by END,
  CASE WHEN f.area_id IS NULL THEN f.debt_soft_limit END, CASE WHEN f.area_id IS NULL THEN f.debt_hard_limit END,
  CASE WHEN f.area_id IS NULL THEN f.debt_soft_grace_days END, CASE WHEN f.area_id IS NULL THEN f.payin_min_amount END,
  CASE WHEN f.area_id IS NULL THEN f.payout_min_amount END, CASE WHEN f.area_id IS NULL THEN f.payout_auto_approve END,
  'legacy:' || encode(sha256(convert_to(row_to_json(f)::text, 'UTF8')), 'hex'), 'migrated from fee_policies', f.id, f.created_at
FROM fee_policies f
JOIN profile_map m ON m.brand_id = f.brand_id AND m.country_code = f.country_code AND m.area_id IS NOT DISTINCT FROM f.area_id;

INSERT INTO pricing_profile_assignments (brand_id, country_code, profile_id, scope_kind, scope_id, valid_from, reason)
SELECT brand_id, country_code, profile_id, CASE WHEN area_id IS NULL THEN 'COUNTRY' ELSE 'AREA' END,
  coalesce(area_id::text, country_code), first_at, 'migrated from fee_policies'
FROM profile_map;

INSERT INTO pricing_profile_events (brand_id, profile_id, kind, reason)
SELECT brand_id, id, 'MIGRATED', 'migrated from fee_policies' FROM pricing_profiles;

-- Backfill: until now every trip was split on the home country's policy in force at the time.
INSERT INTO trip_pricing_records (trip_id, brand_id, quote_id, country_code, profile_id, profile_version_id, profile_version,
  default_profile_id, default_version_id, scope_kind, scope_id, scope_name, terms, charges, trace, config, source, stage, final, record_hash, created_at, finalized_at)
SELECT t.id, t.brand_id, t.quote_id, t.country_code, d.id, v.id, v.version, d.id, v.id, 'COUNTRY', d.country_code, d.name,
  to_jsonb(v) - 'id' - 'profile_id' - 'content_hash' - 'published_by' - 'reason' - 'legacy_fee_policy_id' - 'created_at' - 'effective_from' - 'version',
  '[]', '{"backfilled": true, "note": "split used the home-country policy"}', '{}', 'BACKFILL',
  CASE WHEN t.status = 'COMPLETED' THEN 'FINAL' ELSE 'QUOTED' END,
  CASE WHEN t.status = 'COMPLETED' THEN jsonb_build_object('breakdown', t.fees, 'completed_at', t.completed_at) END,
  CASE WHEN t.status = 'COMPLETED' THEN encode(sha256(convert_to(t.id::text || t.fees::text, 'UTF8')), 'hex') END,
  t.requested_at, CASE WHEN t.status = 'COMPLETED' THEN coalesce(t.completed_at, now()) END
FROM trips t
JOIN brands b ON b.id = t.brand_id
JOIN profile_map m ON m.brand_id = t.brand_id AND m.country_code = b.country AND m.area_id IS NULL
JOIN pricing_profiles d ON d.id = m.profile_id
JOIN pricing_profile_versions v ON v.profile_id = d.id AND v.version = coalesce((t.fees->>'policy_version')::int, 1)
WHERE t.fees IS NOT NULL
ON CONFLICT (trip_id) DO NOTHING;

ALTER TABLE fee_policies RENAME TO fee_policies_legacy;

-- Immutability guards.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION pricing_refuse_change() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  RAISE EXCEPTION '% rows are immutable', TG_TABLE_NAME USING ERRCODE = 'check_violation';
END $$;

CREATE OR REPLACE FUNCTION pricing_assignments_close_only() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP = 'DELETE' THEN
    RAISE EXCEPTION 'pricing assignments are closed, never deleted' USING ERRCODE = 'check_violation';
  END IF;
  IF OLD.valid_to IS NOT NULL OR NEW.valid_to IS NULL
     OR NEW.profile_id <> OLD.profile_id OR NEW.scope_kind <> OLD.scope_kind OR NEW.scope_id <> OLD.scope_id
     OR NEW.priority <> OLD.priority OR NEW.valid_from <> OLD.valid_from OR NEW.country_code <> OLD.country_code THEN
    RAISE EXCEPTION 'a pricing assignment can only be closed' USING ERRCODE = 'check_violation';
  END IF;
  RETURN NEW;
END $$;

CREATE OR REPLACE FUNCTION trip_pricing_records_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP = 'DELETE' THEN
    RAISE EXCEPTION 'pricing records are never deleted' USING ERRCODE = 'check_violation';
  END IF;
  IF OLD.finalized_at IS NOT NULL OR NEW.stage <> 'FINAL' OR NEW.finalized_at IS NULL OR NEW.record_hash IS NULL
     OR NEW.profile_id IS DISTINCT FROM OLD.profile_id OR NEW.profile_version_id IS DISTINCT FROM OLD.profile_version_id
     OR NEW.profile_version IS DISTINCT FROM OLD.profile_version OR NEW.default_profile_id IS DISTINCT FROM OLD.default_profile_id
     OR NEW.default_version_id IS DISTINCT FROM OLD.default_version_id OR NEW.scope_kind IS DISTINCT FROM OLD.scope_kind
     OR NEW.scope_id IS DISTINCT FROM OLD.scope_id OR NEW.scope_name IS DISTINCT FROM OLD.scope_name
     OR NEW.terms IS DISTINCT FROM OLD.terms OR NEW.charges IS DISTINCT FROM OLD.charges OR NEW.trace IS DISTINCT FROM OLD.trace
     OR NEW.config IS DISTINCT FROM OLD.config OR NEW.quote_id IS DISTINCT FROM OLD.quote_id OR NEW.source IS DISTINCT FROM OLD.source THEN
    RAISE EXCEPTION 'a pricing record can only be finalised once, without changing what was pinned' USING ERRCODE = 'check_violation';
  END IF;
  RETURN NEW;
END $$;
-- +goose StatementEnd

CREATE TRIGGER pricing_profile_versions_immutable BEFORE UPDATE OR DELETE ON pricing_profile_versions FOR EACH ROW EXECUTE FUNCTION pricing_refuse_change();
CREATE TRIGGER pricing_profile_charges_immutable BEFORE UPDATE OR DELETE ON pricing_profile_charges FOR EACH ROW EXECUTE FUNCTION pricing_refuse_change();
CREATE TRIGGER pricing_profile_events_append_only BEFORE UPDATE OR DELETE ON pricing_profile_events FOR EACH ROW EXECUTE FUNCTION pricing_refuse_change();
CREATE TRIGGER fee_policies_legacy_read_only BEFORE INSERT OR UPDATE OR DELETE ON fee_policies_legacy FOR EACH ROW EXECUTE FUNCTION pricing_refuse_change();
CREATE TRIGGER pricing_assignments_close_only BEFORE UPDATE OR DELETE ON pricing_profile_assignments FOR EACH ROW EXECUTE FUNCTION pricing_assignments_close_only();
CREATE TRIGGER trip_pricing_records_guard BEFORE UPDATE OR DELETE ON trip_pricing_records FOR EACH ROW EXECUTE FUNCTION trip_pricing_records_guard();

-- +goose Down
DROP TRIGGER IF EXISTS fee_policies_legacy_read_only ON fee_policies_legacy;
ALTER TABLE fee_policies_legacy RENAME TO fee_policies;
DROP TABLE IF EXISTS trip_pricing_records;
ALTER TABLE quotes DROP COLUMN IF EXISTS pricing;
DROP TABLE IF EXISTS pricing_profile_events;
DROP TABLE IF EXISTS pricing_profile_assignments;
DROP TABLE IF EXISTS pricing_profile_charges;
DROP TABLE IF EXISTS pricing_profile_versions;
DROP TABLE IF EXISTS pricing_profiles;
DROP TABLE IF EXISTS charge_payees;
DROP FUNCTION IF EXISTS trip_pricing_records_guard();
DROP FUNCTION IF EXISTS pricing_assignments_close_only();
DROP FUNCTION IF EXISTS pricing_refuse_change();
