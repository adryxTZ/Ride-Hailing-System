# ADR 0010: pricing profiles replace fee policies; districts are geofences

Date: 2026-10-23. Status: accepted (owner request, spec `DOC/specs/2026-10-23-pricing-profiles-geofences.md`).

## Context
Fee policies were versioned rows per country and region, but a trip was split on the home country's policy in force at the moment of the split, so a trip's rates were not pinned and region rules were ignored at settlement (a D12 #17 regression). The owner asked for profiles applied to regions, districts, places and drawn zones, with extra charges (toll, parking, levy …), active/inactive states, an auditable record of the profile every trip used, and history of which profile applied where and when.

## Decision
- Profiles (`pricing_profiles`) with immutable versions and charges; one open COUNTRY assignment is the default; other assignments are AREA, DISTRICT, ZONE or PLACE, closed (never edited or deleted) when they end. `fee_policies` became `fee_policies_legacy` (read-only).
- Precedence: place > zone > district > region > country default, then priority; same-kind overlaps at the same priority are refused when saved.
- Every quote stores the resolution and per-class terms; booking pins them in `trip_pricing_records`; the split reads only that pin; completion seals it (a database trigger refuses any later change).
- Districts are geofences (drawn or imported polygons), not nested operating areas: areas stay a flat H3 cover with no overlaps.
- Resolution never queries Postgres on the quote path: profiles and geofences are cached in memory and invalidated by `config_versions`.

## Consequences
Rates are now pinned per trip and auditable; extra charges go through one `Split()` and the ledger (`CHARGES_PAYABLE` per payee). Changing the default is an explicit, audited action. Debt-hold and payout rules still read the home country's default (owner question 12).
