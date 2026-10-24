package trips

import (
	"encoding/json"
	"slices"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func queuedTrip(now time.Time, pickupIn time.Duration) *Trip {
	ahead, by, accepted := "ahead", now.Add(pickupIn), now.Add(-10*time.Minute)
	t := &Trip{Status: Accepted, QueuedAfter: &ahead, queuedPickupBy: &by,
		Timestamps: map[string]*time.Time{"accepted_at": &accepted}}
	t.AllowedNext = AllowedNext(t.Status, t.Lifecycle)
	return t
}

func TestQueuedTripCannotBeStartedFromTheApp(t *testing.T) {
	now := time.Now()
	tr := queuedTrip(now, 5*time.Minute)
	applyQueue(tr, now)
	if !tr.Queued || slices.Contains(tr.AllowedNext, EnRouteToPickup) || !slices.Contains(tr.AllowedNext, CancelledByDriver) {
		t.Fatalf("queued trip: queued=%v next=%v", tr.Queued, tr.AllowedNext)
	}
	if tr.QueuedPickupEtaS == nil || *tr.QueuedPickupEtaS < 290 || *tr.QueuedPickupEtaS > 300 {
		t.Fatalf("pickup eta: %v", tr.QueuedPickupEtaS)
	}
}

func TestQueuedEtaNeverReadsZero(t *testing.T) {
	now := time.Now()
	tr := queuedTrip(now, -3*time.Minute) // the promise passed: the driver is late finishing
	applyQueue(tr, now)
	if tr.QueuedPickupEtaS == nil || *tr.QueuedPickupEtaS != minQueuedEtaS {
		t.Fatalf("eta floor: %v", tr.QueuedPickupEtaS)
	}
}

func TestCurrentTripIsNotQueued(t *testing.T) {
	tr := &Trip{Status: Accepted, Timestamps: map[string]*time.Time{}}
	tr.AllowedNext = AllowedNext(tr.Status, tr.Lifecycle)
	applyQueue(tr, time.Now())
	if tr.Queued || !slices.Contains(tr.AllowedNext, EnRouteToPickup) {
		t.Fatalf("a current trip keeps its start button: %v", tr.AllowedNext)
	}
}

func TestQueuedTripIsFreeToCancelUntilItBecomesCurrent(t *testing.T) {
	now := time.Now()
	option, _ := json.Marshal(map[string]any{"cancellation_fee": "2000", "cancel_free_s": 120})
	tr := queuedTrip(now, 5*time.Minute) // accepted 10 minutes ago, well past the free window
	tr.QuoteOption = option
	if fee := cancelFeeAt(tr, now); !fee.IsZero() {
		t.Fatalf("queued: fee %s", fee)
	}
	// Promoted a minute ago: the free window counts from then, not from the accept.
	promoted := now.Add(-time.Minute)
	tr.QueuedAfter, tr.queuePromotedAt = nil, &promoted
	if fee := cancelFeeAt(tr, now); !fee.IsZero() {
		t.Fatalf("just promoted: fee %s", fee)
	}
	if fee := cancelFeeAt(tr, now.Add(2*time.Minute)); !fee.Equal(decimal.NewFromInt(2000)) {
		t.Fatalf("after the window: fee %s", fee)
	}
}
