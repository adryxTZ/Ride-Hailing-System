package dispatch

import (
	"context"
	"errors"
	"testing"

	"github.com/kalen-limited/hauler-rider/backend/internal/trips"
)

func TestDefaultsAreOnWithDropoffAndPayment(t *testing.T) {
	q := DefaultQueueSettings("TZ")
	if !q.Enabled || !q.IsFinishing(trips.ArrivedAtDropoff) || !q.IsFinishing(trips.AwaitingPayment) || q.IsFinishing(trips.InTransit) {
		t.Fatalf("defaults: %+v", q)
	}
	var none *Settings // not wired: defaults, never a failure
	if !none.For(context.Background(), "KE").Enabled {
		t.Fatal("unwired settings must default to on")
	}
}

func TestFinishingIgnoresTheSwitch(t *testing.T) {
	// Matching checks the switch; marking drivers finishing does not, so turning
	// the queue on reaches drivers already at a drop-off.
	q := DefaultQueueSettings("TZ")
	q.Enabled = false
	if !q.IsFinishing(trips.AwaitingPayment) {
		t.Fatal("finishing follows the statuses only")
	}
	r := trips.OpenRequest{TripID: "t1", RiderID: "u1", Pickup: pickup}
	if _, ok := candidateOf(finishing("d1", nearby), r, nil, q.Enabled); ok {
		t.Fatal("with the queue off a finishing driver is not a candidate")
	}
}

func TestApplyValidatesFinishingStatuses(t *testing.T) {
	off := false
	for name, c := range map[string]struct {
		in   QueueSettingsInput
		ok   bool
		want []trips.Status
	}{
		"toggle only keeps statuses": {QueueSettingsInput{Enabled: &off}, true, defaultFinishing},
		"in transit allowed":         {QueueSettingsInput{FinishingStatuses: []trips.Status{trips.InTransit, trips.AwaitingPayment, trips.InTransit}}, true, []trips.Status{trips.InTransit, trips.AwaitingPayment}},
		"empty list refused":         {QueueSettingsInput{FinishingStatuses: []trips.Status{}}, false, nil},
		"pickup status refused":      {QueueSettingsInput{FinishingStatuses: []trips.Status{trips.ArrivedAtPickup}}, false, nil},
	} {
		q := DefaultQueueSettings("TZ")
		err := apply(&q, c.in)
		if (err == nil) != c.ok {
			t.Errorf("%s: err %v", name, err)
			continue
		}
		if !c.ok {
			if !errors.Is(err, ErrFinishingStatuses) {
				t.Errorf("%s: want FINISHING_STATUSES_INVALID, got %v", name, err)
			}
			continue
		}
		if len(q.FinishingStatuses) != len(c.want) {
			t.Errorf("%s: got %v want %v", name, q.FinishingStatuses, c.want)
			continue
		}
		for i := range c.want {
			if q.FinishingStatuses[i] != c.want[i] {
				t.Errorf("%s: got %v want %v", name, q.FinishingStatuses, c.want)
			}
		}
	}
}
