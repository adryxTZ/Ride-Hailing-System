// Package dispatch matches waiting trips to available drivers (DOC/02-dispatch-engine.md).
//
//	model.go          tunables, the Offer sent to drivers, error codes
//	matcher.go        the assignment step: pure function, unit-tested
//	store.go          trip_offers persistence (no rules)
//	stats.go          a driver's offer answers (totals and per day) for the staff driver page
//	service.go        the loop (one leader per brand via a Redis lock), accept / decline,
//	                  and reactions to trip changes (close offers, free the driver)
//	queue.go          the trip queue: finishing drivers as candidates, handing a
//	                  driver over to their next trip, release on offline, the banner
//	settings.go       per-country dispatch settings (trip queue on/off, finishing
//	                  statuses), cached by config version, audited writes
//	settings_store.go dispatch_settings persistence (no rules)
//
// v0 behaviour: every TickInterval the waiting trips and the idle drivers around
// them are assigned together (greedy by pickup ETA, one trip per driver — a
// batched assignment; Hungarian/min-cost flow replaces the greedy step when
// volumes justify it). Each chosen driver gets one sequential offer for OfferTTL;
// a decline or timeout frees the trip for the next tick and that driver is not
// asked again for the same trip. Trips past their window end as NO_DRIVER_FOUND.
//
// Trip queue (forward dispatch): drivers whose trip is in a finishing status
// (per country, default ARRIVED_AT_DROPOFF and AWAITING_PAYMENT) and who have
// no next trip are candidates too, measured from their drop-off plus a
// finishing allowance and ranked behind idle drivers about as close. Accepting
// queues the trip behind the current one; it becomes current when that ends.
package dispatch
