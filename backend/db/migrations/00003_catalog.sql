-- +goose Up
-- Service catalogue: everything the rider app's service picker and request
-- forms need. Rows are per brand so one deployment can carry several brands.
CREATE TYPE service_category AS ENUM ('GOODS', 'PASSENGER', 'RECOVERY', 'LIQUID');
CREATE TYPE service_status AS ENUM ('ACTIVE', 'HIDDEN', 'DISABLED');

CREATE TABLE services (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  brand_id        uuid NOT NULL REFERENCES brands(id),
  key             text NOT NULL,
  category        service_category NOT NULL,
  name            jsonb NOT NULL,                 -- {"en": "...", "sw": "..."}
  description     jsonb NOT NULL DEFAULT '{}',
  status          service_status NOT NULL DEFAULT 'ACTIVE',
  sort_order      int NOT NULL DEFAULT 0,
  icon            text,
  lifecycle       jsonb NOT NULL,                 -- {profile, has_loading, has_unloading, payment_at_end, ...}
  request_schema  jsonb NOT NULL DEFAULT '{}',    -- JSON Schema (2020-12 subset) + x-ui hints
  dispatch        jsonb NOT NULL DEFAULT '{}',    -- per-service dispatch knobs (overrides matching config)
  compliance      jsonb NOT NULL DEFAULT '{}',    -- regulator mapping
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  UNIQUE (brand_id, key)
);

CREATE TABLE vehicle_classes (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  service_id      uuid NOT NULL REFERENCES services(id) ON DELETE CASCADE,
  key             text NOT NULL,
  name            jsonb NOT NULL,
  description     jsonb NOT NULL DEFAULT '{}',
  icon            text,
  sort_order      int NOT NULL DEFAULT 0,
  seats           int,
  capacity_kg_min int,
  capacity_kg_max int,
  volume_l_max    int,
  regulator_type  text,                           -- e.g. LATRA CAR | MOTORCYCLE | TRICYCLE
  status          service_status NOT NULL DEFAULT 'ACTIVE',
  UNIQUE (service_id, key)
);

CREATE TABLE cargo_types (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  service_id      uuid NOT NULL REFERENCES services(id) ON DELETE CASCADE,
  key             text NOT NULL,
  name            jsonb NOT NULL,
  description     jsonb NOT NULL DEFAULT '{}',
  icon            text,
  sort_order      int NOT NULL DEFAULT 0,
  allowed_classes text[] NOT NULL DEFAULT '{}',   -- vehicle_classes.key values
  status          service_status NOT NULL DEFAULT 'ACTIVE',
  UNIQUE (service_id, key)
);

-- Bumped on every catalogue write; the in-process cache keys on it.
CREATE TABLE config_versions (
  scope      text PRIMARY KEY,
  version    bigint NOT NULL DEFAULT 1,
  updated_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO config_versions (scope) VALUES ('catalog');

-- +goose Down
DROP TABLE config_versions;
DROP TABLE cargo_types;
DROP TABLE vehicle_classes;
DROP TABLE services;
DROP TYPE service_status;
DROP TYPE service_category;
