package cets

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/providers/regulator"
)

var update = flag.Bool("update", false, "rewrite testdata golden files")

const tzDar = "Africa/Dar_es_Salaam"

func sampleDriver() regulator.DriverRecord {
	return regulator.DriverRecord{
		DriverID: "drv-1", Name: " Juma Ally ", Phone: "+255659829811", Email: "juma@example.com",
		LicenceNumber: "4000123454", LicenceClass: "d", Country: "TZ", Timezone: tzDar,
	}
}

func sampleVehicle() regulator.VehicleRecord {
	registered := time.Date(2026, 7, 20, 22, 30, 0, 0, time.UTC) // 01:30 on the 21st in Dar
	return regulator.VehicleRecord{
		VehicleID: "veh-1", DriverID: "drv-1", Plate: "t 921 abc", Make: "Toyota", Model: "IST",
		VehicleType: "CAR", RegisteredOn: &registered, Country: "TZ", Timezone: tzDar, DriverLicence: "4000123454",
	}
}

func sampleTrip() regulator.TripRecord {
	return regulator.TripRecord{
		TripID: "e4d1f0aa-0000-4000-8000-000000000001", Code: "TR-1",
		Driver: sampleDriver(), Vehicle: sampleVehicle(),
		PassengerName: "Asha Musa", PassengerPhone: "+255715000000",
		Pickup: regulator.Point{Lat: -6.7924, Lng: 39.2083}, Dropoff: regulator.Point{Lat: -6.81, Lng: 39.28},
		PickupAddress: "Hazina", DropoffAddress: "Makulu",
		StartedAt: time.Date(2026, 7, 21, 8, 0, 0, 0, time.UTC), EndedAt: time.Date(2026, 7, 21, 8, 45, 30, 0, time.UTC),
		DistanceM: 12450, Fare: decimal.RequireFromString("25000.00"), DriverEarning: decimal.RequireFromString("22500.40"),
		Rating: 4, Currency: "TZS", Timezone: tzDar,
	}
}

func sampleReport() regulator.DailyReport {
	loc, _ := time.LoadLocation(tzDar)
	return regulator.DailyReport{
		Country: "TZ", Date: time.Date(2026, 7, 21, 0, 0, 0, 0, loc), Timezone: tzDar,
		Trips: 42, Paid: decimal.RequireFromString("1050000.60"), Currency: "TZS",
	}
}

// TestGoldenPayloads sends each kind to a stub and compares the exact body.
func TestGoldenPayloads(t *testing.T) {
	s := &stub{status: 200, body: `{"statusCode":5000,"data":{}}`}
	c := newClient(t, s.server(t).URL)
	ctx := context.Background()
	kinds := map[string]func() error{
		"driver.json":  func() error { _, e := c.RegisterDriver(ctx, sampleDriver()); return e },
		"vehicle.json": func() error { _, e := c.RegisterVehicle(ctx, sampleVehicle()); return e },
		"trip.json":    func() error { _, e := c.SubmitTrip(ctx, sampleTrip()); return e },
		"report.json":  func() error { _, e := c.SubmitDailyReport(ctx, sampleReport()); return e },
	}
	for file, send := range kinds {
		if err := send(); err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		compareGolden(t, file, []byte(s.sent))
	}
}

// compareGolden indents got and compares it with testdata/name.
func compareGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	var buf bytes.Buffer
	if err := json.Indent(&buf, got, "", "  "); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	buf.WriteByte('\n')
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(want, buf.Bytes()) {
		t.Errorf("%s differs:\nwant %s\ngot  %s", name, want, buf.Bytes())
	}
}

func TestTripDataGapsAreNull(t *testing.T) {
	tr := sampleTrip()
	tr.Pickup, tr.StartedAt, tr.Fare, tr.Vehicle.Plate, tr.Rating = regulator.Point{}, time.Time{}, decimal.Zero, " ", 9
	body, err := tripPayload(tr)
	if err != nil {
		t.Fatal(err)
	}
	if body.OriginCoordinates != nil || body.StartTime != nil || body.TotalFareAmount != nil ||
		body.VehicleRegistrationNumber != nil || body.Rating != maxRating {
		t.Fatalf("gaps not null or rating not clamped: %+v", body)
	}
}
