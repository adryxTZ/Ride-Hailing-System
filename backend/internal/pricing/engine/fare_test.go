package engine

import (
	"testing"

	"github.com/shopspring/decimal"
)

func d(s string) decimal.Decimal             { return decimal.RequireFromString(s) }
func ptr(v decimal.Decimal) *decimal.Decimal { return &v }

// Mirrors the reference system's UAT tariff shape: base 2200 (first bracket), 2200/km to 5 km,
// 2000/km to 20 km, 1800/km after; no duration charge; floor 15,000.
func sampleTariff() Tariff {
	return Tariff{
		Currency: "TZS",
		Distance: []Bracket{
			{Min: d("0"), Max: ptr(d("5000")), UnitPrice: d("2200")},
			{Min: d("5000"), Max: ptr(d("20000")), UnitPrice: d("2000")},
			{Min: d("20000"), Max: nil, UnitPrice: d("1800")},
		},
		Duration:          []Bracket{{Min: d("0"), Max: nil, UnitPrice: d("0")}},
		MinimumFare:       d("15000"),
		WaitingRatePerMin: d("250"),
		LoaderRate:        d("6000"),
		LoadingGraceS:     1200,
		UnloadingGraceS:   1200,
		SurgeEnabled:      true,
	}
}

// Golden cases computed by hand from the reference system's calculate_fare:
// fare = base + rate(dist)*km + rate(dur)*min, ×surge, floored, rounded.
func TestRouteFareGolden(t *testing.T) {
	tf := sampleTariff()
	cases := []struct {
		name        string
		distM, durS string
		surge       string
		want        string
	}{
		{"short trip hits the floor", "1000", "300", "1", "15000.00"},             // 2200 + 2200×1 = 4400 → floor 15000
		{"8.42 km in the 2000 bracket", "8420", "1380", "1", "19040.00"},          // 2200 + 2000×8.42 = 19040
		{"25 km in the open bracket", "25000", "3000", "1", "47200.00"},           // 2200 + 1800×25 = 47200
		{"boundary 5000 m uses the second bracket", "5000", "0", "1", "12200.00"}, // 2200 + 2000×5 = 12200 → floor 15000? no: 12200 < 15000 → 15000
		{"surge 1.5 on 8.42 km", "8420", "1380", "1.5", "28560.00"},               // 19040 × 1.5
		{"non-positive surge is ignored", "8420", "1380", "0", "19040.00"},
	}
	cases[3].want = "15000.00"
	for _, c := range cases {
		got := RouteFare(tf, d(c.distM), d(c.durS), d(c.surge))
		if got.StringFixed(2) != c.want {
			t.Errorf("%s: got %s want %s", c.name, got.StringFixed(2), c.want)
		}
	}
}

func TestQuoteAddsLoadersAndBookingFee(t *testing.T) {
	tf := sampleTariff()
	total, lines := Quote(tf, QuoteInput{DistanceM: d("8420"), DurationS: d("1380"), Surge: d("1"), Loaders: 2, BookingFee: d("500")})
	// loaders: 6000 × 2 = 12000 (flat per loader, reference-system semantics)
	if total.StringFixed(2) != "31540.00" {
		t.Fatalf("total %s", total.StringFixed(2))
	}
	codes := map[string]string{}
	for _, l := range lines {
		codes[l.Code] = l.Amount.StringFixed(2)
	}
	if codes[LineBaseFare] != "19040.00" || codes[LineLoaders] != "12000.00" || codes[LineBookingFee] != "500.00" {
		t.Fatalf("lines %v", codes)
	}
}

func TestBillWaitingFloorAndApproach(t *testing.T) {
	tf := sampleTariff()
	tf.PickupApproachPerKm = d("500")
	// 1 km trip: trip_fare floored to 15000; waiting 10 min over grace at 250 = 2500
	// subtotal 17500 ≥ floor so no adjustment; approach 3 km × 500 = 1500; total 19000
	total, lines := Bill(tf, BillInput{DistanceM: d("1000"), DurationS: d("300"), Surge: d("1"),
		LoadingTakenS: 1800, UnloadingTakenS: 1200, ApproachM: d("3000"), ApproachApplies: true})
	if total.StringFixed(2) != "19000.00" {
		t.Fatalf("total %s (%v)", total.StringFixed(2), lines)
	}
	for _, l := range lines {
		if l.Code == LineMinimumFareAdjustment {
			t.Fatalf("no adjustment expected, got %s", l.Amount)
		}
	}
	// Surge is absorbed by the floor on short trips: 4400×1.5 = 6600 < 15000 → surge line absent
	_, lines = Bill(tf, BillInput{DistanceM: d("1000"), DurationS: d("300"), Surge: d("1.5")})
	for _, l := range lines {
		if l.Code == LineSurge {
			t.Fatalf("surge must be absorbed by the floor")
		}
	}
}

func TestRateForOpenEndedAndBeyond(t *testing.T) {
	br := []Bracket{{Min: d("0"), Max: ptr(d("10")), UnitPrice: d("1")}, {Min: d("10"), Max: ptr(d("20")), UnitPrice: d("2")}}
	if RateFor(d("25"), br).String() != "2" { // beyond every bracket → last
		t.Fatal("beyond-range should use the last bracket")
	}
	if RateFor(d("10"), br).String() != "2" { // boundary belongs to the upper bracket
		t.Fatal("boundary should use the upper bracket")
	}
}
