package trips

// AllowedNext lists the statuses a ride may move to from its current status.
// Driver-side transitions are listed so the driver app renders its buttons
// from the server. The ride is paid at the end unless the lifecycle says not.
//
//	(SCHEDULED →) REQUESTED → OFFERED → ACCEPTED → EN_ROUTE_TO_PICKUP → ARRIVED_AT_PICKUP → IN_TRANSIT →
//	ARRIVED_AT_DROPOFF → AWAITING_PAYMENT → COMPLETED
func AllowedNext(status Status, lc Lifecycle) []Status {
	switch status {
	case Scheduled, Requested, Offered:
		return []Status{CancelledByRider} // acceptance happens through dispatch
	case Accepted:
		return []Status{EnRouteToPickup, CancelledByRider, CancelledByDriver}
	case EnRouteToPickup:
		return []Status{ArrivedAtPickup, CancelledByRider, CancelledByDriver}
	case ArrivedAtPickup:
		return []Status{InTransit, CancelledByRider, CancelledByDriver}
	case InTransit:
		return []Status{ArrivedAtDropoff}
	case ArrivedAtDropoff:
		if lc.PaymentAtEnd {
			return []Status{AwaitingPayment}
		}
		return []Status{Completed}
	case AwaitingPayment:
		return []Status{Completed}
	default:
		return nil
	}
}

// CanTransition reports whether `to` is reachable from `from` for this lifecycle.
func CanTransition(from, to Status, lc Lifecycle) bool {
	for _, s := range AllowedNext(from, lc) {
		if s == to {
			return true
		}
	}
	return false
}

// RiderMayCancel is true until the ride has started.
func RiderMayCancel(status Status) bool {
	switch status {
	case Scheduled, Requested, Offered, Accepted, EnRouteToPickup, ArrivedAtPickup:
		return true
	}
	return false
}

// IsTerminal reports whether no further transitions are possible.
func IsTerminal(status Status) bool {
	switch status {
	case Completed, CancelledByRider, CancelledByDriver, CancelledByStaff, NoDriverFound, Expired:
		return true
	}
	return false
}
