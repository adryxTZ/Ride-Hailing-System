# ADR 0009 · The Platform is a taxi app (like Bolt and Uber), nothing else

Date: 2026-10-20 · Status: Accepted · Supersedes the multi-service scope of ADR 0003 and the "hauling ships first" phasing

## Context
Kalen, 2026-10-20: "I have made up my mind, we are only doing TAXI like Bolt and Uber. Remove all, leave only taxi, finalize all apps driver and rider."
By then the Platform also carried delivery, cargo hauling, recovery, water, construction materials, food, groceries and mechanic services, with their own lifecycles (loading/unloading, single-site jobs), a truck-distance pricing module, cargo types and per-service request forms.

## Decision
- One service: **taxi**, with vehicle classes as the ride types (Bajaji, Standard, XL; more are catalogue rows).
- One lifecycle: ACCEPTED → EN_ROUTE_TO_PICKUP → ARRIVED_AT_PICKUP → IN_TRANSIT → ARRIVED_AT_DROPOFF → AWAITING_PAYMENT → COMPLETED. LOADING/UNLOADING are gone from the code (the database enum keeps the values so old rows stay readable).
- The catalogue keeps its shape (a service with classes, a request schema and a lifecycle) because the white-label brand, tariffs and dispatch read it; seeds hold only `taxi.json`, and the seeder **disables** any service whose seed file is gone instead of deleting it.
- Removed: cargo types (table dropped), vehicle capacity/volume columns, the site pricing module, the nearest-truck supply lookup, receiver contacts, service-specific request forms, the rider's service picker and service screens, the driver's on-device service chooser.
- Kept because taxis need them: stops, fixed-route fares (e.g. town – airport), booking for someone else ("Who is riding?"), chat, ratings, push, driver registration and review, the ledger.
- The rider app opens on the taxi map; the driver app serves the service and class of the driver's approved vehicle.
- The multi-service version is archived outside the repository: `ADRIAN/RIDER-HAULER-multiservice-snapshot-2026-10-20.tar.gz`.

## Amendment 2026-10-20: Boda package delivery
Kalen, the same day: "also add boda package delivery". A second service `parcel` ("Package") exists, like Bolt Send: same ride lifecycle, Boda class only, the receiver's name and phone are required, the sender is asked when the rider is not at the pickup, and an optional "what are you sending" note goes to the driver. A Boda driver registers once (taxi · boda) and serves every service offering the `boda` class, so they get ride and package offers. No other delivery types (cars, trucks, cargo) come back.

## Consequences
- Less code and fewer states to test; every screen is a taxi screen.
- Adding another service later means new catalogue rows and, if its lifecycle differs, new states in `trips/machine.go`. ADR 0003's mechanism allows it; it is not planned.
