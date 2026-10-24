# Spec: trip queue (accept the next trip while finishing)

Owner request (Kalen, 2026-10-24): "feature to allow a driver to accept another trip when finalizing a trip".
This is Uber's forward dispatch / trip queue, listed in `DOC/02-dispatch-engine.md` (forward dispatch) and
built here in its simplest safe form. Migration `00045_trip_queue.sql`.

## 1. Behaviour
- **Finishing driver.** A driver whose current trip is in a *finishing status* (per country; default
  `ARRIVED_AT_DROPOFF` and `AWAITING_PAYMENT`, `IN_TRANSIT` may be added) and who has no next trip is a
  candidate for new requests. Distance and ETA are measured **from the current drop-off**, plus a finishing
  allowance (`finishingAllowanceS` = 120 s). In the batch they rank behind idle drivers that are about as
  close (`queuedRankPenaltyS` = 120 s). At most **one** queued trip per driver.
- **Offer.** The offer carries `queued: true`; the driver app shows "Next trip — after this one" and
  "Pickup · x km from this drop-off". Declining or letting it expire behaves like any offer.
- **Accept.** The trip becomes `ACCEPTED` with the driver set and `queued: true`. It has no
  `EN_ROUTE_TO_PICKUP` in `allowed_transitions`; forcing it answers `409 NEXT_TRIP_WAITING`. If the trip
  ahead ended between the offer and the answer, the accept simply makes it the current trip.
- **Promotion.** Completing the current trip (cash confirmation or gateway collection) or a staff cancel of
  it promotes the queued trip **in the same transaction** (`queued_after` cleared, `queue_promoted_at` set).
  Presence follows (ON_TRIP on the next trip), both apps get `trip.updated`, and the driver app moves to it.
- **Rider of the queued trip** sees "Your driver is finishing a trip nearby" with "Pickup in ~N min"
  (`queued_pickup_eta_s`, finishing time included, never below a minute). The trip is ACCEPTED, so no
  "no driver found" timer applies. Cancelling is free while queued, and the free-cancel window restarts at
  promotion.
- **Driver cancels the queued trip** (tap the banner): it goes back to matching like any driver cancel
  (`REQUESTED`, new window, that driver not asked again).
- **Driver goes offline** (switch, or swept as stale after 120 s without pings) with a queued trip: it is
  released back to matching (`trip.queue_released`). An open queued offer just expires.
- **Driver comes back online** while still on a trip: presence is restored to ON_TRIP on that trip (it used
  to reset to IDLE, which would have let them be offered a second current trip).

## 2. Data and contract
- `trips.queued_after` (the trip it waits for), `queued_pickup_by`, `queue_promoted_at`;
  `trip_offers.queued`.
- Unique partial indexes: `trips_one_queued_per_driver` (ACCEPTED and queued) and
  `trips_one_current_per_driver` (active statuses, not queued). Before this, a second current trip was
  only prevented by dispatch offering idle drivers.
- `dispatch_settings (brand_id, country_code, trip_queue_enabled, finishing_statuses, updated_by, updated_at)`;
  no row = defaults (on). Config scope `dispatch` (in-process cache, version check every 5 s).
- Presence hash: `finishing`, `drop_lat`, `drop_lng`, `queued` (compare-and-set Lua scripts), so matching
  never reads Postgres.
- API: `Offer.queued`; `Trip.queued`, `Trip.queued_pickup_eta_s`; `DriverPresence.finishing|dropoff|queued_trip_id`;
  `GET /driver/trips/next` (`NextTrip`: pickup, rider first name, distance from the drop-off, ETA);
  `GET /admin/dispatch/settings`, `PATCH /admin/dispatch/settings/{country}` (permission `dispatch.manage`,
  reason required, audited `dispatch.settings`). Errors: `NEXT_TRIP_WAITING`, `NEXT_TRIP_ALREADY_QUEUED`,
  `FINISHING_STATUSES_INVALID` (en, sw).
- `GET /driver/trips/active` returns the **current** trip only (never the queued one).

## 3. Apps and admin
- Driver: `QueuedOfferTag` on the offer card, `NextTripBanner` (Banner primitive; tap to cancel the next
  trip) during IN_TRANSIT / ARRIVED_AT_DROPOFF / AWAITING_PAYMENT, `useNextTrip`, `models/tripQueue`.
  Hook-ins: HomeScreen shows a queued offer over the trip panel; `useDriverTrips` ignores frames about the
  queued trip, never clears the current trip on another trip's frame, and re-reads the active trip after
  the current one ends (automatic switch).
- Rider: `models/tripQueue` + one branch in `TripBlocks` ("assigned" phase).
- Admin: Settings → Dispatch (`/settings/dispatch`), one card per country: switch "Offer the next trip while
  a driver is finishing", the finishing statuses, reason dialog for every change.

## 4. Safety rails
- Payment: the queued trip is a different row; cash confirmation and gateway completion of the current
  trip are unchanged and promote inside their transaction (`e2e-fares` re-run green).
- The switch is checked at match time; the finishing flag follows statuses only, so switching on reaches
  drivers already at a drop-off. Changes take up to 5 s to reach every dispatcher.
- Known limits: candidates are found by the driver's live position (GEOSEARCH), then measured from the
  drop-off; with `IN_TRANSIT` counted as finishing, a driver far from their drop-off is not found near
  pickups around it. Live `driver.location` frames go to the current rider only; the queued rider sees the
  driver's position on each read. The rider's "driver accepted" push is the same as for a normal accept.
  The driver's rating prompt for the finished trip is skipped when the next trip takes over the sheet.

## 5. Tests
- Go: `dispatch/queue_test.go` (finishing candidate rules incl. one queued at most, switch off, drop-off
  distance; idle preferred when about as close; finishing wins when clearly closer), `settings_test.go`,
  `trips/queue_test.go` (read model, cancel fee), `trips/queue_integration_test.go` (DB: queue, one-queued
  limit, start gate, promotion on completion and on staff cancel, release, driver cancel),
  `presence/queue_test.go` (Redis scripts, offline hook).
- E2E: `apps/tools/e2e-trip-queue.mjs` (PASS lines for each behaviour above); `e2e-dispatch.mjs` and
  `e2e-fares.mjs` now take `API`, `PG_DB`, `NEXA_DATABASE_URL`, `NEXA_REDIS_URL` for another stack.

## Factory log
- 2026-10-24: built on branch `worktree-agent-a69c7bf5830e38836` from `3dad0cd`, own stack (`nexa_chain`,
  API :8094, Redis db 4). Migration 00045 up, down, up verified.
- Verification: gofmt clean, `go vet`, `golangci-lint run ./...` 0 issues, `make brand-check` ok, `go test ./...`
  green with `NEXA_TEST_DATABASE_URL` (nexa_chain) and `NEXA_TEST_REDIS_URL` (db 12); driver and rider
  tsc + Biome + Jest green; admin tsc + Biome green; both TS clients regenerated.
- E2E on :8094: `e2e-trip-queue.mjs` 5 PASS lines; `e2e-dispatch.mjs` PASSED; `e2e-fares.mjs` PASSED.
- p95 (50 calls, local): `GET /driver/trips/next` 1.9 ms (1 query without a next trip, 3 with one, plus one
  Redis read); `GET /admin/dispatch/settings` 1.2 ms (1 query); matching reads settings from the cache.
- Found and fixed on the way: going online during a trip reset presence to IDLE (second current trip
  possible); the transition handler set IDLE after the trip listener (would undo a promotion); trip frames
  were published after dispatch reacted (the driver app could get the next trip before the end of the
  current one, now published first).
