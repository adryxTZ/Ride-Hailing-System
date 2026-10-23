-- +goose Up
CREATE EXTENSION IF NOT EXISTS postgis;
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE brands (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  code            text UNIQUE NOT NULL CHECK (code ~ '^[a-z][a-z0-9_-]{1,31}$'),
  display_name    text NOT NULL,
  legal_entity    text,
  country         char(2) NOT NULL,
  currency        char(3) NOT NULL,
  timezone        text NOT NULL,
  default_locale  text NOT NULL,
  locales         text[] NOT NULL DEFAULT '{en}',
  domains         jsonb NOT NULL DEFAULT '{}',
  support_phone   text,
  support_email   text,
  providers       jsonb NOT NULL DEFAULT '{}',
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE theme_presets (
  key         text PRIMARY KEY,
  name        text NOT NULL,
  colours     text[] NOT NULL CHECK (array_length(colours, 1) = 4),
  roles       jsonb NOT NULL,
  source_url  text,
  tags        text[] NOT NULL DEFAULT '{}'
);

CREATE TYPE theme_status AS ENUM ('DRAFT', 'PUBLISHED', 'ARCHIVED');

CREATE TABLE brand_themes (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  brand_id      uuid NOT NULL REFERENCES brands(id),
  version       int NOT NULL,
  status        theme_status NOT NULL DEFAULT 'DRAFT',
  name          text,
  source        text NOT NULL,            -- PRESET:<key> | IMPORTED:<url> | CUSTOM
  palette       jsonb NOT NULL,
  dark          jsonb,
  derived       jsonb,
  typography    jsonb NOT NULL DEFAULT '{}',
  shape         jsonb NOT NULL DEFAULT '{}',
  assets        jsonb NOT NULL DEFAULT '{}',
  wcag_report   jsonb,
  change_reason text,
  created_by    uuid,
  created_at    timestamptz NOT NULL DEFAULT now(),
  published_at  timestamptz,
  published_by  uuid,
  UNIQUE (brand_id, version)
);
CREATE UNIQUE INDEX brand_themes_one_published ON brand_themes (brand_id) WHERE status = 'PUBLISHED';

-- +goose Down
DROP TABLE brand_themes;
DROP TYPE theme_status;
DROP TABLE theme_presets;
DROP TABLE brands;
