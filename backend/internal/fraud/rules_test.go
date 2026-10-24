package fraud

import (
	"testing"
	"time"
)

func TestCheckPing(t *testing.T) {
	t0 := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	base := Ping{Lat: -6.80, Lng: 39.28, At: t0}
	// ~5.5 km north
	far := Ping{Lat: -6.75, Lng: 39.28}
	for name, c := range map[string]struct {
		prev *Ping
		cur  Ping
		hit  bool
		mock bool
	}{
		"first ping":                    {nil, base, false, false},
		"jitter: tiny move":             {&base, Ping{Lat: -6.8001, Lng: 39.28, At: t0.Add(time.Second)}, false, false},
		"too close in time to judge":    {&base, Ping{Lat: far.Lat, Lng: far.Lng, At: t0.Add(time.Second)}, false, false},
		"5 km in 2 s":                   {&base, Ping{Lat: far.Lat, Lng: far.Lng, At: t0.Add(2 * time.Second)}, true, false},
		"5 km in 10 minutes is a drive": {&base, Ping{Lat: far.Lat, Lng: far.Lng, At: t0.Add(10 * time.Minute)}, false, false},
		"the phone says mocked":         {&base, Ping{Lat: -6.8, Lng: 39.28, At: t0.Add(time.Minute), Mocked: true}, true, true},
		"mocked on the first ping":      {nil, Ping{Lat: -6.8, Lng: 39.28, At: t0, Mocked: true}, true, true},
	} {
		got := CheckPing(c.prev, c.cur)
		if got.Hit != c.hit || got.Mocked != c.mock {
			t.Errorf("%s: got %+v", name, got)
		}
	}
}

func TestSelfBookingSignals(t *testing.T) {
	for name, c := range map[string]struct {
		p     TripParties
		codes []string
		worst string
	}{
		"clean trip":          {TripParties{DriverPhone: "+255700000001", PickupPhone: "+255700000002"}, nil, SeverityLow},
		"shared install":      {TripParties{DriverPhone: "+255700000001", SharedInstalls: []string{"i"}}, []string{SignalSharedInstall}, SeverityHigh},
		"passenger is driver": {TripParties{DriverPhone: "+255700000001", PickupPhone: "+255700000001"}, []string{SignalPassengerIsDriver}, SeverityMedium},
		"receiver is driver":  {TripParties{DriverPhone: "+255700000001", ReceiverPhone: "+255700000001"}, []string{SignalPassengerIsDriver}, SeverityMedium},
		"no contacts at all":  {TripParties{DriverPhone: "+255700000001"}, nil, SeverityLow},
		"both":                {TripParties{DriverPhone: "x", PickupPhone: "x", SharedInstalls: []string{"i"}}, []string{SignalSharedInstall, SignalPassengerIsDriver}, SeverityHigh},
	} {
		got := SelfBookingSignals(c.p)
		if len(got) != len(c.codes) {
			t.Fatalf("%s: got %+v", name, got)
		}
		for i := range got {
			if got[i].Code != c.codes[i] {
				t.Errorf("%s: signal %d is %s", name, i, got[i].Code)
			}
		}
		if worstSeverity(got) != c.worst {
			t.Errorf("%s: worst %s", name, worstSeverity(got))
		}
	}
}

func TestCancelFindings(t *testing.T) {
	rules := func(c CancelCounts) (out []string) {
		for _, f := range CancelFindings(c) {
			out = append(out, f.Rule)
		}
		return out
	}
	d := func(acc, cancels, arr, noShows int) CancelCounts {
		return CancelCounts{SubjectKind: SubjectDriver, Accepted: acc, Cancels: cancels, Arrivals: arr, NoShows: noShows}
	}
	if got := rules(d(10, 3, 0, 0)); len(got) != 1 || got[0] != RuleDriverCancelRate {
		t.Fatalf("3 of 10 cancels must flag, got %v", got)
	}
	if got := rules(d(10, 2, 0, 0)); len(got) != 0 {
		t.Fatalf("2 of 10 must not flag, got %v", got)
	}
	if got := rules(d(9, 9, 0, 0)); len(got) != 0 {
		t.Fatalf("below the sample size must not flag, got %v", got)
	}
	if got := rules(d(20, 0, 10, 3)); len(got) != 1 || got[0] != RuleDriverNoShowRate {
		t.Fatalf("3 no-shows in 10 arrivals must flag, got %v", got)
	}
	r := CancelCounts{SubjectKind: SubjectRider, Accepted: 10, Cancels: 4}
	if got := rules(r); len(got) != 1 || got[0] != RuleRiderCancelRate {
		t.Fatalf("4 of 10 rider cancels must flag, got %v", got)
	}
	r.Cancels = 3
	if got := rules(r); len(got) != 0 {
		t.Fatalf("3 of 10 rider cancels must not flag, got %v", got)
	}
}
