-- +goose Up
-- Route-based fixed prices: "Kariakoo → Goba 60,000", "Dar ↔ Iringa 850,000".
-- A route matches when pickup is within origin_radius_m of origin and dropoff
-- within destination_radius_m of destination (either direction if bidirectional).
-- Precedence: manual price > fixed route > metered tariff (zones later sit between).
CREATE TABLE fixed_routes (
  id                    uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  brand_id              uuid NOT NULL REFERENCES brands(id),
  service_key           text NOT NULL,
  name                  text NOT NULL,
  origin                geography(Point, 4326) NOT NULL,
  origin_radius_m       int NOT NULL DEFAULT 1500,
  destination           geography(Point, 4326) NOT NULL,
  destination_radius_m  int NOT NULL DEFAULT 1500,
  bidirectional         boolean NOT NULL DEFAULT true,
  vehicle_classes       text[] NOT NULL DEFAULT '{}',   -- empty = every class of the service
  price                 numeric(14,2) NOT NULL,
  currency              char(3) NOT NULL,
  includes_loaders      boolean NOT NULL DEFAULT false,
  surge_applies         boolean NOT NULL DEFAULT false,
  priority              int NOT NULL DEFAULT 0,         -- higher wins when several match
  status                service_status NOT NULL DEFAULT 'ACTIVE',
  valid_from            timestamptz,
  valid_to              timestamptz,
  created_at            timestamptz NOT NULL DEFAULT now(),
  updated_at            timestamptz NOT NULL DEFAULT now(),
  UNIQUE (brand_id, service_key, name)
);

-- +goose Down
DROP TABLE fixed_routes;
