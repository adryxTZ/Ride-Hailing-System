package engine

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestRegistryListsEngines(t *testing.T) {
	keys := map[string]bool{}
	for _, d := range Describe() {
		keys[d.Key] = true
		if len(d.SettingsSchema) == 0 {
			t.Errorf("%s has no settings schema", d.Key)
		}
	}
	for _, k := range []string{"bracket_meter", "per_km_min", "flat_rate"} {
		if !keys[k] || Get(k) == nil {
			t.Errorf("engine %s missing", k)
		}
	}
}

func TestPerKmMin(t *testing.T) {
	tf := Tariff{Currency: "TZS", MinimumFare: d("3000"), SurgeEnabled: true, LoaderRate: d("0"),
		Params: map[string]decimal.Decimal{"base_fare": d("1500"), "per_km": d("700"), "per_min": d("100")}}
	total, _ := Get("per_km_min").Quote(tf, QuoteInput{DistanceM: d("10000"), DurationS: d("1200"), Surge: d("1")})
	// 1500 + 700×10 + 100×20 = 10500
	if total.StringFixed(2) != "10500.00" {
		t.Fatalf("got %s", total.StringFixed(2))
	}
	total, _ = Get("per_km_min").Quote(tf, QuoteInput{DistanceM: d("500"), DurationS: d("60"), Surge: d("1")})
	if total.StringFixed(2) != "3000.00" { // 1500+350+100 = 1950 → floor 3000
		t.Fatalf("floor: got %s", total.StringFixed(2))
	}
}

func TestFlatRate(t *testing.T) {
	tf := Tariff{Currency: "TZS", SurgeEnabled: false, Params: map[string]decimal.Decimal{"price": d("45000")}}
	total, lines := Get("flat_rate").Quote(tf, QuoteInput{DistanceM: d("99999"), DurationS: d("9"), Surge: d("2"), BookingFee: d("1000")})
	if total.StringFixed(2) != "46000.00" || len(lines) != 2 { // surge disabled on this tariff
		t.Fatalf("got %s %v", total.StringFixed(2), lines)
	}
}
