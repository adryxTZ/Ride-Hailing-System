-- +goose Up
-- Geofences (spec 2026-10-23-pricing-profiles-geofences.md, S2): districts and
-- zones drawn as polygons, and named places as a point with a radius. Shapes
-- are versioned (every save appends a version; a pricing record names the
-- version it matched). Profiles are assigned to them like regions.

CREATE TABLE geofences (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  brand_id     uuid NOT NULL REFERENCES brands(id),
  country_code char(2) NOT NULL,
  kind         text NOT NULL CHECK (kind IN ('DISTRICT','ZONE','PLACE')),
  code         text NOT NULL CHECK (code ~ '^[a-z0-9][a-z0-9-]{1,47}$'),
  name         text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
  status       text NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','INACTIVE','ARCHIVED')),
  version      int NOT NULL DEFAULT 1,
  geom         geography(MultiPolygon, 4326),
  center       geography(Point, 4326),
  radius_m     int CHECK (radius_m BETWEEN 50 AND 5000),
  area_m2      double precision,
  vertex_count int,
  created_by   uuid REFERENCES users(id),
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  CHECK ((kind = 'PLACE') = (center IS NOT NULL AND radius_m IS NOT NULL AND geom IS NULL)),
  CHECK (kind = 'PLACE' OR geom IS NOT NULL),
  UNIQUE (brand_id, country_code, code)
);
CREATE INDEX geofences_geom_idx ON geofences USING gist (geom);
CREATE INDEX geofences_center_idx ON geofences USING gist (center);
CREATE INDEX geofences_scope_idx ON geofences (brand_id, country_code, kind, status);

CREATE TABLE geofence_versions (
  geofence_id uuid NOT NULL REFERENCES geofences(id),
  version     int NOT NULL,
  geom        geography(MultiPolygon, 4326),
  center      geography(Point, 4326),
  radius_m    int,
  created_by  uuid REFERENCES users(id),
  reason      text NOT NULL,
  created_at  timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (geofence_id, version)
);
CREATE INDEX geofence_versions_time_idx ON geofence_versions (geofence_id, created_at);

CREATE TRIGGER geofence_versions_append_only BEFORE UPDATE OR DELETE ON geofence_versions FOR EACH ROW EXECUTE FUNCTION pricing_refuse_change();

INSERT INTO config_versions (scope) VALUES ('geofences') ON CONFLICT DO NOTHING;

-- Hardening of the S1 guards (security review 2026-10-23):
-- charges can only be added to a version in the transaction that created it;
-- a record's identity columns are pinned too; an assignment's brand cannot
-- change and it cannot be closed in the past.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION pricing_charges_with_version_only() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF (SELECT created_at FROM pricing_profile_versions WHERE id = NEW.version_id) <> now() THEN
    RAISE EXCEPTION 'charges belong to the version they were published with' USING ERRCODE = 'check_violation';
  END IF;
  RETURN NEW;
END $$;

CREATE OR REPLACE FUNCTION trip_pricing_records_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP = 'DELETE' THEN
    RAISE EXCEPTION 'pricing records are never deleted' USING ERRCODE = 'check_violation';
  END IF;
  IF OLD.finalized_at IS NOT NULL OR NEW.stage <> 'FINAL' OR NEW.finalized_at IS NULL OR NEW.record_hash IS NULL
     OR NEW.trip_id IS DISTINCT FROM OLD.trip_id OR NEW.brand_id IS DISTINCT FROM OLD.brand_id
     OR NEW.country_code IS DISTINCT FROM OLD.country_code OR NEW.created_at IS DISTINCT FROM OLD.created_at
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

CREATE OR REPLACE FUNCTION pricing_assignments_close_only() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP = 'DELETE' THEN
    RAISE EXCEPTION 'pricing assignments are closed, never deleted' USING ERRCODE = 'check_violation';
  END IF;
  IF OLD.valid_to IS NOT NULL OR NEW.valid_to IS NULL OR NEW.valid_to < now() - interval '1 minute'
     OR NEW.brand_id <> OLD.brand_id OR NEW.profile_id <> OLD.profile_id OR NEW.scope_kind <> OLD.scope_kind OR NEW.scope_id <> OLD.scope_id
     OR NEW.priority <> OLD.priority OR NEW.valid_from <> OLD.valid_from OR NEW.country_code <> OLD.country_code THEN
    RAISE EXCEPTION 'a pricing assignment can only be closed, now' USING ERRCODE = 'check_violation';
  END IF;
  RETURN NEW;
END $$;
-- +goose StatementEnd

CREATE TRIGGER pricing_charges_with_version_only BEFORE INSERT ON pricing_profile_charges FOR EACH ROW EXECUTE FUNCTION pricing_charges_with_version_only();

-- +goose Down
DROP TRIGGER IF EXISTS pricing_charges_with_version_only ON pricing_profile_charges;
DROP FUNCTION IF EXISTS pricing_charges_with_version_only();
DELETE FROM config_versions WHERE scope = 'geofences';
DROP TABLE IF EXISTS geofence_versions;
DROP TABLE IF EXISTS geofences;
