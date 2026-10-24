package dispatch

import (
	"testing"

	"github.com/kalen-limited/hauler-rider/backend/internal/presence"
	"github.com/kalen-limited/hauler-rider/backend/internal/providers/routing"
	"github.com/kalen-limited/hauler-rider/backend/internal/trips"
)

// Points in Dar es Salaam: a pickup, a drop-off 500 m away, a spot 8 km away.
var (
	pickup  = routing.Point{Lat: -6.7764, Lng: 39.2290}
	nearby  = routing.Point{Lat: -6.7764, Lng: 39.2335}
	farAway = routing.Point{Lat: -6.8500, Lng: 39.2290}
)

func finishing(id string, drop routing.Point) presence.Driver {
	return presence.Driver{ID: id, State: presence.OnTrip, Finishing: true, Location: drop, Dropoff: &drop, TripID: "current"}
}

func TestFinishingDriverCandidateRules(t *testing.T) {
	r := trips.OpenRequest{TripID: "t1", RiderID: "u1", Pickup: pickup}
	queuedAlready := finishing("d1", nearby)
	queuedAlready.QueuedTripID = "t0"
	notFinishing := finishing("d1", nearby)
	notFinishing.Finishing = false
	// Where the driver is now does not matter: the next pickup is measured from the drop-off.
	farNowNearDrop := finishing("d1", nearby)
	farNowNearDrop.Location = farAway
	for name, c := range map[string]struct {
		d       presence.Driver
		queueOn bool
		asked   map[string]bool
		want    bool
	}{
		"finishing near the pickup":              {finishing("d1", nearby), true, nil, true},
		"trip queue turned off for the country":  {finishing("d1", nearby), false, nil, false},
		"already has a next trip (one at most)":  {queuedAlready, true, nil, false},
		"on a trip that is not finishing":        {notFinishing, true, nil, false},
		"drop-off too far from the pickup":       {finishing("d1", farAway), true, nil, false},
		"asked before for this trip":             {finishing("d1", nearby), true, map[string]bool{"d1": true}, false},
		"the rider's own driver account":         {finishing("u1", nearby), true, nil, false},
		"far now, drop-off near: measured there": {farNowNearDrop, true, nil, true},
	} {
		got, ok := candidateOf(c.d, r, c.asked, c.queueOn)
		if ok != c.want {
			t.Errorf("%s: got %v", name, ok)
		}
		if ok && (!got.Queued || got.EtaS < finishingAllowanceS) {
			t.Errorf("%s: a finishing candidate is queued and its ETA includes the allowance, got %+v", name, got)
		}
	}
}

func TestIdleDriverIsNotQueued(t *testing.T) {
	r := trips.OpenRequest{TripID: "t1", RiderID: "u1", Pickup: pickup}
	c, ok := candidateOf(presence.Driver{ID: "d1", State: presence.Idle, Location: nearby}, r, nil, true)
	if !ok || c.Queued || c.EtaS != etaSeconds(c.DistanceM) {
		t.Fatalf("idle driver: %+v %v", c, ok)
	}
}

func TestIdleDriverPreferredWhenAboutAsClose(t *testing.T) {
	// The finishing driver's ETA (allowance included) is a little better, but
	// within the penalty: the idle driver gets the trip.
	got := assign([]string{"t1"}, []candidate{
		{TripID: "t1", DriverID: "finishing", EtaS: 300, Queued: true},
		{TripID: "t1", DriverID: "idle", EtaS: 360},
	})
	if len(got) != 1 || got[0].DriverID != "idle" {
		t.Fatalf("idle driver about as close should win, got %v", got)
	}
}

func TestFinishingDriverWinsWhenClearlyCloser(t *testing.T) {
	got := assign([]string{"t1"}, []candidate{
		{TripID: "t1", DriverID: "finishing", EtaS: 200, Queued: true},
		{TripID: "t1", DriverID: "idle", EtaS: 900},
	})
	if len(got) != 1 || got[0].DriverID != "finishing" || !got[0].Queued {
		t.Fatalf("a much closer finishing driver should win, got %v", got)
	}
}

func TestOneTripPerFinishingDriverPerBatch(t *testing.T) {
	got := assign([]string{"t1", "t2"}, []candidate{
		{TripID: "t1", DriverID: "f", EtaS: 200, Queued: true},
		{TripID: "t2", DriverID: "f", EtaS: 210, Queued: true},
	})
	if len(got) != 1 {
		t.Fatalf("a finishing driver gets at most one next trip, got %v", got)
	}
}

func TestQueuedOriginFallsBackToLocation(t *testing.T) {
	d := presence.Driver{Location: farAway}
	if queuedOrigin(d) != farAway {
		t.Fatal("without a drop-off the driver's location is used")
	}
	d.Dropoff = &nearby
	if queuedOrigin(d) != nearby {
		t.Fatal("the drop-off is used when known")
	}
}

func TestFirstName(t *testing.T) {
	full, blank := "Asha Juma Said", "  "
	if got := firstName(&full); got == nil || *got != "Asha" {
		t.Fatalf("got %v", got)
	}
	if firstName(&blank) != nil || firstName(nil) != nil {
		t.Fatal("blank or missing names give nil")
	}
}
