# ADR 0003 · Service abstraction: configuration first, small Go plugin second

Date: 2026-10-14 · Status: Accepted

## Context
The apps are service-first: a rider picks Taxi, Cargo hauling, Recovery, Water (clean or septic) and more later. Each service has its own request fields, vehicle classes, lifecycle phases, pricing and regulatory mapping. Songa hard-coded one cargo flow.

## Decision
- `Service` rows carry a versioned **RequestSchema** (JSON Schema + `x-ui` hints), a **LifecycleProfile** (which phases exist, who triggers them, timers), a **PricingPolicy** (engine + tariff set), a **ComplianceMapping** and a **DispatchPolicy**.
- `VehicleClass` rows per service; vehicles hold eligibilities to many classes.
- One generic trip state machine parameterised by the profile; durations measured server-side.
- A per-service Go **plugin** interface (`Validate`, `Filter`, `PricingInputs`, `ExtraLines`, `OnTransition`) for behaviour that must be code; default no-op.
- Apps render forms from the schema and buttons from `allowed_transitions[]`, so a new service needs no app release.

## Alternatives
- Pure config (no plugin): capacity validation and special rules end up as untyped expressions.
- Pure code per service: the Songa fork problem again.

## Consequences
"Add Water delivery" = seed rows + a ~120-line plugin + strings; water clean and septic are two services because their stop rules differ; the catalogue is per brand.
