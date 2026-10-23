-- +goose Up
-- Tariffs: the fare curve per vehicle class (port of Songa's PricingBracket +
-- CarType pricing fields), versioned through config_versions('pricing').
CREATE TYPE bracket_kind AS ENUM ('DISTANCE', 'DURATION');

CREATE TABLE tariffs (
  id                     uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  vehicle_class_id       uuid NOT NULL REFERENCES vehicle_classes(id) ON DELETE CASCADE,
  currency               char(3) NOT NULL,
  minimum_fare           numeric(14,2) NOT NULL DEFAULT 0,        -- floor after surge; 0 = use first distance bracket's unit price
  waiting_rate_per_min   numeric(14,2) NOT NULL DEFAULT 0,
  pickup_approach_per_km numeric(14,2) NOT NULL DEFAULT 0,
  loader_rate            numeric(14,2) NOT NULL DEFAULT 0,        -- flat fee per loader
  loading_grace_s        int NOT NULL DEFAULT 900,
  unloading_grace_s      int NOT NULL DEFAULT 900,
  surge_enabled          boolean NOT NULL DEFAULT true,
  status                 service_status NOT NULL DEFAULT 'ACTIVE',
  created_at             timestamptz NOT NULL DEFAULT now(),
  updated_at             timestamptz NOT NULL DEFAULT now(),
  UNIQUE (vehicle_class_id)
);

CREATE TABLE tariff_brackets (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tariff_id   uuid NOT NULL REFERENCES tariffs(id) ON DELETE CASCADE,
  kind        bracket_kind NOT NULL,
  min_value   numeric(14,2) NOT NULL,        -- metres for DISTANCE, seconds for DURATION
  max_value   numeric(14,2),                 -- NULL = open ended
  unit_price  numeric(14,2) NOT NULL,        -- per km / per minute; the FIRST distance bracket's price is also the base fare
  sort_order  int NOT NULL DEFAULT 0
);
CREATE INDEX tariff_brackets_tariff_idx ON tariff_brackets (tariff_id, kind, min_value);

-- Platform-wide money policy, versioned; trips pin the row they were settled under.
CREATE TABLE fee_policies (
  id                     uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  brand_id               uuid NOT NULL REFERENCES brands(id),
  version                int NOT NULL,
  commission_rate        numeric(6,4) NOT NULL DEFAULT 0.1000,
  vat_rate               numeric(6,4) NOT NULL DEFAULT 0.1800,
  vat_base               text NOT NULL DEFAULT 'COMMISSION' CHECK (vat_base IN ('COMMISSION','GROSS_FARE')),
  levy_rate              numeric(6,4) NOT NULL DEFAULT 0,
  levy_borne_by          text NOT NULL DEFAULT 'PLATFORM' CHECK (levy_borne_by IN ('PLATFORM','DRIVER','RIDER')),
  booking_fee            numeric(14,2) NOT NULL DEFAULT 0,
  effective_from         timestamptz NOT NULL DEFAULT now(),
  created_at             timestamptz NOT NULL DEFAULT now(),
  UNIQUE (brand_id, version)
);

-- Quotes: what the rider was offered, kept so a booking can lock exactly that price.
CREATE TABLE quotes (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  brand_id        uuid NOT NULL REFERENCES brands(id),
  user_id         uuid REFERENCES users(id),
  service_key     text NOT NULL,
  pickup          geography(Point, 4326) NOT NULL,
  dropoff         geography(Point, 4326),
  attributes      jsonb NOT NULL DEFAULT '{}',
  route           jsonb NOT NULL,             -- {distance_m, duration_s, provider, polyline?}
  surge           numeric(6,3) NOT NULL DEFAULT 1,
  options         jsonb NOT NULL,             -- per vehicle class: fare, lines, availability
  pricing_version bigint NOT NULL,
  expires_at      timestamptz NOT NULL,
  created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX quotes_user_idx ON quotes (user_id, created_at DESC);

INSERT INTO config_versions (scope) VALUES ('pricing') ON CONFLICT DO NOTHING;

-- +goose Down
DROP TABLE quotes;
DROP TABLE fee_policies;
DROP TABLE tariff_brackets;
DROP TABLE tariffs;
DROP TYPE bracket_kind;
DELETE FROM config_versions WHERE scope='pricing';
