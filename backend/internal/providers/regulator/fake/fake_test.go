package fake

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/kalen-limited/hauler-rider/backend/internal/providers/regulator"
)

var ctx = context.Background()

func TestModes(t *testing.T) {
	ack, err := New("ok").RegisterDriver(ctx, regulator.DriverRecord{LicenceNumber: "4000123454"})
	if err != nil || !strings.HasPrefix(ack.Reference, "FAKE-") || len(ack.Reference) != len("FAKE-")+8 || ack.Code != CodeSuccess {
		t.Fatalf("ok mode: %+v %v", ack, err)
	}
	_, err = New("reject").SubmitDailyReport(ctx, regulator.DailyReport{})
	if r, ok := regulator.AsRejection(err); !ok || r.Code != "5001" || r.Class != regulator.ClassRetry {
		t.Fatalf("reject mode: %v", err)
	}
	if _, err = New("down").SubmitTrip(ctx, regulator.TripRecord{}); !errors.Is(err, regulator.ErrUnavailable) {
		t.Fatalf("down mode: %v", err)
	}
	if f := New(""); f.Mode() != ModeOK || f.Name() != "latra" {
		t.Fatalf("defaults: mode %q name %q", f.Mode(), f.Name())
	}
}

func TestReferencesDiffer(t *testing.T) {
	f := New(ModeOK)
	a, _ := f.RegisterVehicle(ctx, regulator.VehicleRecord{Plate: "T123ABC"})
	b, _ := f.RegisterVehicle(ctx, regulator.VehicleRecord{Plate: "T123ABC"})
	if a.Reference == b.Reference {
		t.Fatalf("references repeat: %s", a.Reference)
	}
}

func TestTriggers(t *testing.T) {
	f := New(ModeOK)
	cases := []struct {
		name    string
		call    func() error
		reject  bool
		unavail bool
	}{
		{"plate TFAIL vehicle", func() error { _, e := f.RegisterVehicle(ctx, regulator.VehicleRecord{Plate: "tfail 123"}); return e }, true, false},
		{"plate TDOWN trip", func() error {
			_, e := f.SubmitTrip(ctx, regulator.TripRecord{Vehicle: regulator.VehicleRecord{Plate: "TDOWN1"}})
			return e
		}, false, true},
		{"licence driver", func() error {
			_, e := f.RegisterDriver(ctx, regulator.DriverRecord{LicenceNumber: TriggerFailLicence})
			return e
		}, true, false},
		{"licence trip", func() error {
			_, e := f.SubmitTrip(ctx, regulator.TripRecord{Driver: regulator.DriverRecord{LicenceNumber: TriggerFailLicence}})
			return e
		}, true, false},
		{"plain plate", func() error { _, e := f.RegisterVehicle(ctx, regulator.VehicleRecord{Plate: "T123ABC"}); return e }, false, false},
	}
	for _, c := range cases {
		err := c.call()
		r, isRej := regulator.AsRejection(err)
		if isRej != c.reject || errors.Is(err, regulator.ErrUnavailable) != c.unavail || (isRej && r.Code != CodeRejected) {
			t.Errorf("%s: got %v", c.name, err)
		}
	}
}

func TestScriptQueuesPerKind(t *testing.T) {
	f := New(ModeDown)
	rej := &regulator.Rejection{Code: "5018", Class: regulator.ClassPrerequisite}
	f.Script(KindTrip, nil, rej)
	if _, err := f.SubmitTrip(ctx, regulator.TripRecord{}); err != nil {
		t.Fatalf("first scripted outcome should be accepted, got %v", err)
	}
	if _, err := f.RegisterDriver(ctx, regulator.DriverRecord{}); !errors.Is(err, regulator.ErrUnavailable) {
		t.Fatalf("driver kind is unscripted: want mode answer, got %v", err)
	}
	if _, err := f.SubmitTrip(ctx, regulator.TripRecord{}); !errors.Is(err, rej) {
		t.Fatalf("second scripted outcome: %v", err)
	}
	if _, err := f.SubmitTrip(ctx, regulator.TripRecord{}); !errors.Is(err, regulator.ErrUnavailable) || f.Pending(KindTrip) != 0 {
		t.Fatalf("queue drained: want mode answer, got %v", err)
	}
}

func TestScriptBeatsTrigger(t *testing.T) {
	f := New(ModeOK)
	f.Script(KindVehicle, nil)
	if _, err := f.RegisterVehicle(ctx, regulator.VehicleRecord{Plate: "TFAIL1"}); err != nil {
		t.Fatalf("script should win over trigger: %v", err)
	}
}

func TestScriptConcurrent(t *testing.T) {
	f := New(ModeOK)
	const n = 50
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(2)
		go func() { defer wg.Done(); f.Script(KindDailyReport, regulator.ErrUnavailable) }()
		go func() { defer wg.Done(); _, _ = f.SubmitDailyReport(ctx, regulator.DailyReport{}) }()
	}
	wg.Wait()
	if p := f.Pending(KindDailyReport); p < 0 || p > n {
		t.Fatalf("pending out of range: %d", p)
	}
}
