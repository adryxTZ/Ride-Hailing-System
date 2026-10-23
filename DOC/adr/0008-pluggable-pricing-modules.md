# ADR 0008 · Pricing is a registry of pluggable modules chosen per tariff

Date: 2026-10-14 · Status: Accepted

## Context
Kalen: "we should have multiple pricing modules that one can choose on the admin … according to market need / area need / trip type; some services just need flat rate; some routes are fixed price, and the same route can differ per direction." Songa had two or three hard-coded engines selected globally.

## Decision
- A pricing module implements one Go interface (`engine.Engine`) and registers itself; the registry is the only place the quote and billing paths look up engines.
- The operator chooses the module **per vehicle-class tariff** and fills its parameters; each module publishes a JSON Schema for them so the admin renders the form generically.
- Route-based fixed prices (direction-aware) and, next, zone tariffs sit above the metered module in a fixed precedence order; manual staff prices sit above everything.
- Add-ons (loaders, waiting, approach, booking fee) are shared helpers so switching modules never changes how extras are charged.

## Consequences
Adding a module is one file plus tests, no admin or app change. Golden tests pin the Songa port. Per-area selection arrives with pricing zones.
