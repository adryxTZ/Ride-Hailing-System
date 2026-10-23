-- +goose Up
-- Operating areas: countries (from the brand file) divided into areas staff mark on
-- the map as sets of H3 res-7 cells. Prices, policies, routes, quotes, trips and
-- drivers are scoped to a country and, where set, an area.
CREATE TABLE countries (
  brand_id        uuid NOT NULL REFERENCES brands(id),
  code            char(2) NOT NULL,
  name            jsonb NOT NULL DEFAULT '{}',
  currency        char(3) NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
  timezone        text NOT NULL,
  area_term       jsonb NOT NULL DEFAULT '{}',
  regulator       text NOT NULL DEFAULT 'none',
  payment_gateway text NOT NULL DEFAULT '',
  emergency_phone text NOT NULL DEFAULT '',
  map_lat         numeric(9,6) NOT NULL DEFAULT 0,
  map_lng         numeric(9,6) NOT NULL DEFAULT 0,
  map_zoom        smallint NOT NULL DEFAULT 11,
  status          text NOT NULL DEFAULT 'DRAFT' CHECK (status IN ('ACTIVE','DRAFT')),
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (brand_id, code)
);

CREATE TABLE areas (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  brand_id      uuid NOT NULL REFERENCES brands(id),
  country_code  char(2) NOT NULL,
  code          text NOT NULL,
  name          text NOT NULL,
  status        text NOT NULL DEFAULT 'DRAFT' CHECK (status IN ('ACTIVE','DRAFT','ARCHIVED')),
  surge_enabled boolean NOT NULL DEFAULT true,
  surge_max     numeric(4,2) CHECK (surge_max IS NULL OR surge_max BETWEEN 1 AND 2),
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  FOREIGN KEY (brand_id, country_code) REFERENCES countries (brand_id, code),
  UNIQUE (brand_id, country_code, code)
);

-- One row per res-7 cell; the primary key is what makes overlapping areas impossible.
CREATE TABLE area_cells (
  brand_id uuid NOT NULL,
  cell     bigint NOT NULL,
  area_id  uuid NOT NULL REFERENCES areas(id) ON DELETE CASCADE,
  PRIMARY KEY (brand_id, cell)
);
CREATE INDEX area_cells_area_idx ON area_cells (area_id);

CREATE TABLE driver_areas (
  driver_id  uuid NOT NULL REFERENCES driver_profiles(user_id) ON DELETE CASCADE,
  area_id    uuid NOT NULL REFERENCES areas(id),
  is_primary boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (driver_id, area_id)
);
CREATE UNIQUE INDEX driver_areas_primary_uq ON driver_areas (driver_id) WHERE is_primary;
CREATE INDEX driver_areas_area_idx ON driver_areas (area_id);

-- Every brand's home country exists before anything is scoped to it.
INSERT INTO countries (brand_id, code, currency, timezone, status)
SELECT id, country, currency, timezone, 'ACTIVE' FROM brands
ON CONFLICT DO NOTHING;

ALTER TABLE driver_profiles ADD COLUMN country_code char(2);
UPDATE driver_profiles d SET country_code = b.country FROM brands b WHERE b.id = d.brand_id;
ALTER TABLE driver_profiles ALTER COLUMN country_code SET NOT NULL;
CREATE INDEX driver_profiles_country_idx ON driver_profiles (brand_id, country_code, status);

ALTER TABLE tariffs ADD COLUMN country_code char(2), ADD COLUMN area_id uuid REFERENCES areas(id);
UPDATE tariffs t SET country_code = b.country
  FROM vehicle_classes vc JOIN services s ON s.id = vc.service_id JOIN brands b ON b.id = s.brand_id
  WHERE vc.id = t.vehicle_class_id;
ALTER TABLE tariffs ALTER COLUMN country_code SET NOT NULL;
ALTER TABLE tariffs DROP CONSTRAINT tariffs_vehicle_class_id_key;
ALTER TABLE tariffs ADD CONSTRAINT tariffs_scope_uq UNIQUE NULLS NOT DISTINCT (vehicle_class_id, country_code, area_id);

ALTER TABLE fee_policies ADD COLUMN country_code char(2), ADD COLUMN area_id uuid REFERENCES areas(id);
UPDATE fee_policies p SET country_code = b.country FROM brands b WHERE b.id = p.brand_id;
ALTER TABLE fee_policies ALTER COLUMN country_code SET NOT NULL;
ALTER TABLE fee_policies DROP CONSTRAINT fee_policies_brand_id_version_key;
ALTER TABLE fee_policies ADD CONSTRAINT fee_policies_scope_uq UNIQUE NULLS NOT DISTINCT (brand_id, country_code, area_id, version);
CREATE INDEX fee_policies_scope_idx ON fee_policies (brand_id, country_code, area_id, version DESC);

ALTER TABLE fixed_routes ADD COLUMN country_code char(2), ADD COLUMN area_id uuid REFERENCES areas(id);
UPDATE fixed_routes r SET country_code = b.country FROM brands b WHERE b.id = r.brand_id;
ALTER TABLE fixed_routes ALTER COLUMN country_code SET NOT NULL;

ALTER TABLE quotes ADD COLUMN country_code char(2), ADD COLUMN area_id uuid;
ALTER TABLE trips ADD COLUMN country_code char(2), ADD COLUMN area_id uuid REFERENCES areas(id);
UPDATE trips t SET country_code = b.country FROM brands b WHERE b.id = t.brand_id;
ALTER TABLE trips ALTER COLUMN country_code SET NOT NULL;
CREATE INDEX trips_country_requested_idx ON trips (brand_id, country_code, requested_at DESC);
CREATE INDEX trips_area_requested_idx ON trips (area_id, requested_at DESC);

INSERT INTO config_versions (scope) VALUES ('areas') ON CONFLICT DO NOTHING;

-- +goose Down
DROP INDEX IF EXISTS trips_area_requested_idx;
DROP INDEX IF EXISTS trips_country_requested_idx;
ALTER TABLE trips DROP COLUMN IF EXISTS area_id, DROP COLUMN IF EXISTS country_code;
ALTER TABLE quotes DROP COLUMN IF EXISTS area_id, DROP COLUMN IF EXISTS country_code;
ALTER TABLE fixed_routes DROP COLUMN IF EXISTS area_id, DROP COLUMN IF EXISTS country_code;
ALTER TABLE fee_policies DROP CONSTRAINT IF EXISTS fee_policies_scope_uq;
DROP INDEX IF EXISTS fee_policies_scope_idx;
ALTER TABLE fee_policies ADD CONSTRAINT fee_policies_brand_id_version_key UNIQUE (brand_id, version);
ALTER TABLE fee_policies DROP COLUMN IF EXISTS area_id, DROP COLUMN IF EXISTS country_code;
ALTER TABLE tariffs DROP CONSTRAINT IF EXISTS tariffs_scope_uq;
ALTER TABLE tariffs ADD CONSTRAINT tariffs_vehicle_class_id_key UNIQUE (vehicle_class_id);
ALTER TABLE tariffs DROP COLUMN IF EXISTS area_id, DROP COLUMN IF EXISTS country_code;
DROP INDEX IF EXISTS driver_profiles_country_idx;
ALTER TABLE driver_profiles DROP COLUMN IF EXISTS country_code;
DROP TABLE IF EXISTS driver_areas;
DROP TABLE IF EXISTS area_cells;
DROP TABLE IF EXISTS areas;
DROP TABLE IF EXISTS countries;
DELETE FROM config_versions WHERE scope = 'areas';
