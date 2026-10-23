# ADR 0005 · Generic platform: brand and system name are data

Date: 2026-10-14 · Status: Accepted

## Context
Kalen: "the design should be generic, system name should be a variable." The vision documents also describe a tenant-based telematics SaaS. Songa hard-codes its name in SMS, receipts, the admin and the apps.

## Decision
- A `brands` table (one row at launch) holds name, legal entity, logos, colours, domains, store ids, locale, currency, timezone, country. `brand_id` exists on tenant-scoped tables from day one.
- All user-facing strings are templated with brand variables; no literal product name under `internal/` (CI grep).
- Country adapters behind interfaces: `Regulator`, `PaymentGateway`, `SMSProvider`, `RoutingProvider`, `ObjectStore`.
- Service catalogue per brand; apps are white-label builds from brand JSON; admin reads brand from the API.
- Repo and binary use a neutral codename.

## Alternatives
- Single-brand code with a rename later: every past rename of this kind has leaked into schemas, SMS templates and app ids.

## Consequences
Slightly more config plumbing in phase 0; a second brand or country later is configuration plus adapters, not a fork.
