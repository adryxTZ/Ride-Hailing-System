# 04 · Pricing modules

> How pricing is composed, which modules exist, how an operator picks one, and how a developer adds a new one. Code: `backend/internal/pricing` (quotes, tariffs, fixed routes) and `backend/internal/pricing/engine` (the modules). Decision record: `adr/0008-pluggable-pricing-modules.md`.

## 1. Layers, in precedence order

| Layer | What decides the price | Where it is configured |
|---|---|---|
| Manual price | Staff set a price on a trip (after booking; audited) | Admin, per trip (dispatch phase) |
| **Fixed route** | Origin area → destination area for a service and vehicle classes; separate prices per direction; optional validity window and priority | `fixed_routes` (admin CRUD) |
| Zone tariff | Area-specific tariff overriding the global one (pickup decides the zone) | `pricing_zones` + zone tariffs (next step) |
| **Metered tariff** | The vehicle class's tariff, priced by its **engine** | `tariffs` per vehicle class: `engine` + `params` (+ brackets) |

Add-ons are the same for every layer and engine: loaders (flat per loader), waiting beyond grace (billing only), pickup-approach fee (billing only), booking fee. The fee policy (commission, VAT, levy) is applied when the fare is split, never in pricing.

## 2. Modules shipped

| Key | Name | Parameters | Used for |
|---|---|---|---|
| `bracket_meter` | Bracket meter | distance and duration bracket tables; first distance bracket's price is the base fare; minimum fare | Cargo hauling (exact port of Songa V1; golden-tested) |
| `per_km_min` | Per km + per minute | `base_fare`, `per_km`, `per_min`; minimum fare | Taxi, bajaji |
| `flat_rate` | Flat rate per trip | `price` | Water tanker delivery, septic pump-out, any "one price per job" service |

Every module publishes a JSON Schema for its parameters (with `x-ui` labels in EN/SW). `GET /v1/pricing/engines` returns the list, so the admin's tariff editor shows a module selector and renders the right form without module-specific frontend code.

## 3. Choosing a module

Per **vehicle class** of a service (the tariff row). A service can mix modules: a water service might price small tankers per trip (`flat_rate`) and bulk tankers per km. Fixed routes sit above whichever module the class uses. When zones land, a zone can carry its own tariff (and therefore its own module) for an area.

## 4. Adding a module (developer)

1. Create `backend/internal/pricing/engine/<key>.go` with a type implementing `Engine`:
   `Key()`, `Name()`, `SettingsSchema()`, `Quote(Tariff, QuoteInput)`, `Bill(Tariff, BillInput)`.
   Read parameters with `param(t, "name", default)`; finish with `addOns(...)` so loaders and booking fee behave like every other module; use `baseAndSurge(...)` and `waitingCharge(...)` for the shared lines.
2. `func init() { Register(YourEngine{}) }` — nothing else to wire. The registry, the engines endpoint, the seed loader and the quote path pick it up.
3. Add a table test in `engines_test.go` with hand-computed expected totals.
4. If the module needs new tariff columns rather than `params`, add a migration; prefer `params`.
5. Seed or configure a tariff with `"engine": "<key>", "params": {...}`; the quote endpoint prices with it immediately after the pricing version bumps.

Candidates already named in the design: `dimension` (weight/volume tiers, the Songa V3 catalogue), `hourly` (vehicle hire by the hour), `zone_to_zone` (matrix of area pairs).
