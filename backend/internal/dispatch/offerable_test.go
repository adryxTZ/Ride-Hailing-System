package dispatch

import (
	"testing"

	"github.com/kalen-limited/hauler-rider/backend/internal/presence"
	"github.com/kalen-limited/hauler-rider/backend/internal/trips"
)

func TestOfferableNeverOffersTheRidersOwnAccount(t *testing.T) {
	r := trips.OpenRequest{TripID: "t1", RiderID: "u1"}
	for name, c := range map[string]struct {
		d     presence.Driver
		asked map[string]bool
		dist  float64
		want  bool
	}{
		"idle nearby driver":      {presence.Driver{ID: "d1", State: presence.Idle}, nil, 500, true},
		"the rider's own account": {presence.Driver{ID: "u1", State: presence.Idle}, nil, 10, false},
		"already asked":           {presence.Driver{ID: "d1", State: presence.Idle}, map[string]bool{"d1": true}, 500, false},
		"on a trip":               {presence.Driver{ID: "d1", State: presence.OnTrip}, nil, 500, false},
		"too far":                 {presence.Driver{ID: "d1", State: presence.Idle}, nil, maxPickupKm*1000 + 1, false},
	} {
		if got := offerable(c.d, r, c.asked, c.dist); got != c.want {
			t.Errorf("%s: got %v", name, got)
		}
	}
}
