-- +goose Up
CREATE EXTENSION IF NOT EXISTS citext;
CREATE TYPE user_status AS ENUM ('ACTIVE', 'DISABLED', 'DELETED');
CREATE TYPE user_role AS ENUM ('RIDER', 'DRIVER', 'FLEET_OWNER', 'STAFF');
CREATE TYPE app_kind AS ENUM ('RIDER', 'DRIVER', 'ADMIN');

CREATE TABLE users (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  brand_id       uuid NOT NULL REFERENCES brands(id),
  phone          text NOT NULL,                       -- E.164
  email          citext,
  full_name      text,
  locale         text,
  status         user_status NOT NULL DEFAULT 'ACTIVE',
  pdpa_consent_version int,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  deleted_at     timestamptz,
  UNIQUE (brand_id, phone)
);

CREATE TABLE user_roles (
  user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role       user_role NOT NULL,
  granted_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, role)
);

CREATE TABLE staff_roles (
  user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role       text NOT NULL,                           -- Super Admin, Operations Manager, KYC Officer, Finance Officer, ...
  granted_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, role)
);

CREATE TABLE otp_codes (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  brand_id    uuid NOT NULL REFERENCES brands(id),
  phone       text NOT NULL,
  app         app_kind NOT NULL,
  code_hash   text NOT NULL,
  attempts    int NOT NULL DEFAULT 0,
  expires_at  timestamptz NOT NULL,
  consumed_at timestamptz,
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX otp_codes_phone_idx ON otp_codes (brand_id, phone, created_at DESC);

CREATE TABLE refresh_tokens (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  family_id   uuid NOT NULL,
  token_hash  text NOT NULL UNIQUE,
  device_id   uuid,
  expires_at  timestamptz NOT NULL,
  revoked_at  timestamptz,
  created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE devices (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id      uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  app          app_kind NOT NULL,
  platform     text NOT NULL CHECK (platform IN ('ANDROID', 'IOS', 'WEB')),
  push_token   text,
  app_version  text,
  last_seen_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (push_token)
);

-- +goose Down
DROP TABLE devices;
DROP TABLE refresh_tokens;
DROP TABLE otp_codes;
DROP TABLE staff_roles;
DROP TABLE user_roles;
DROP TABLE users;
DROP TYPE app_kind;
DROP TYPE user_role;
DROP TYPE user_status;
