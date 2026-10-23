package trips

import "testing"

func TestRideLifecycle(t *testing.T) {
	lc := Lifecycle{Profile: "pickup_dropoff", PaymentAtEnd: true}
	path := []Status{Accepted, EnRouteToPickup, ArrivedAtPickup, InTransit, ArrivedAtDropoff, AwaitingPayment, Completed}
	for i := 0; i+1 < len(path); i++ {
		if !CanTransition(path[i], path[i+1], lc) {
			t.Fatalf("%s → %s should be allowed", path[i], path[i+1])
		}
	}
	if CanTransition(InTransit, CancelledByDriver, lc) {
		t.Fatal("a driver cannot cancel a ride in progress")
	}
}

func TestRideWithoutPaymentStep(t *testing.T) {
	lc := Lifecycle{Profile: "pickup_dropoff"}
	if !CanTransition(ArrivedAtDropoff, Completed, lc) || CanTransition(ArrivedAtDropoff, AwaitingPayment, lc) {
		t.Fatal("without payment at the end the ride completes on arrival")
	}
}

func TestRiderCancelWindow(t *testing.T) {
	if !RiderMayCancel(EnRouteToPickup) || RiderMayCancel(InTransit) {
		t.Fatal("rider may cancel until the ride starts")
	}
}
