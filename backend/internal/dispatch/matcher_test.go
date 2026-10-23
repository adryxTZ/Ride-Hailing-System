package dispatch

import "testing"

func TestAssignOneDriverPerTripCheapestFirst(t *testing.T) {
	// d1 is closest to both trips; the batch gives d1 to t2 (cheaper) and d2 to t1.
	got := assign([]string{"t1", "t2"}, []candidate{
		{TripID: "t1", DriverID: "d1", EtaS: 300},
		{TripID: "t1", DriverID: "d2", EtaS: 400},
		{TripID: "t2", DriverID: "d1", EtaS: 120},
	})
	want := map[string]string{"t2": "d1", "t1": "d2"}
	if len(got) != 2 {
		t.Fatalf("want 2 assignments, got %v", got)
	}
	for _, c := range got {
		if want[c.TripID] != c.DriverID {
			t.Fatalf("trip %s got %s, want %s", c.TripID, c.DriverID, want[c.TripID])
		}
	}
}

func TestAssignTieGoesToLongestWaiting(t *testing.T) {
	got := assign([]string{"old", "new"}, []candidate{
		{TripID: "new", DriverID: "d1", EtaS: 200},
		{TripID: "old", DriverID: "d1", EtaS: 200},
	})
	if len(got) != 1 || got[0].TripID != "old" {
		t.Fatalf("tie should go to the older trip, got %v", got)
	}
}

func TestEtaFloor(t *testing.T) {
	if etaSeconds(10) != minEtaSeconds {
		t.Fatal("very short pickups still take a minute")
	}
	if etaSeconds(3000) <= etaSeconds(1000) {
		t.Fatal("further is slower")
	}
}
