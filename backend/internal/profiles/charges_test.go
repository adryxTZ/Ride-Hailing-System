package profiles

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func rule(code, kind, amount string) ChargeRule {
	return ChargeRule{Code: code, AmountType: kind, Amount: decimal.RequireFromString(amount), Recipient: "DRIVER", AppliesWhen: "PICKUP", OnFixedRoutes: true}
}

func apply(r ChargeRule, in ChargeInput) []AppliedCharge {
	return ApplyCharges([]SourcedRule{{ChargeRule: r, ProfileID: "p", Version: 1, MatchedAt: "PICKUP"}}, in)
}

func TestChargeAmounts(t *testing.T) {
	in := ChargeInput{DistanceM: 12500, FareBase: decimal.NewFromInt(8000), At: t0}
	if a := apply(rule("F", "FLAT", "500"), in); a[0].Amount.String() != "500" {
		t.Fatalf("flat = %s", a[0].Amount)
	}
	if a := apply(rule("K", "PER_KM", "40"), in); a[0].Amount.String() != "500" {
		t.Fatalf("per km = %s", a[0].Amount)
	}
	pct := rule("P", "PERCENT_OF_FARE", "0.05")
	max := decimal.NewFromInt(300)
	pct.MaxAmount = &max
	if a := apply(pct, in); a[0].Amount.String() != "300" {
		t.Fatalf("percent capped = %s", a[0].Amount)
	}
	min := decimal.NewFromInt(1000)
	flat := rule("M", "FLAT", "200")
	flat.MinAmount = &min
	if a := apply(flat, in); a[0].Amount.String() != "1000" {
		t.Fatalf("min = %s", a[0].Amount)
	}
	if a := apply(rule("Z", "FLAT", "0"), in); len(a) != 0 {
		t.Fatal("a zero charge must be skipped")
	}
}

func TestChargeFilters(t *testing.T) {
	r := rule("C", "FLAT", "100")
	r.VehicleClasses, r.Services = []string{"boda"}, []string{"taxi"}
	if len(apply(r, ChargeInput{Service: "taxi", Class: "standard", At: t0})) != 0 {
		t.Fatal("class filter ignored")
	}
	if len(apply(r, ChargeInput{Service: "parcel", Class: "boda", At: t0})) != 0 {
		t.Fatal("service filter ignored")
	}
	r.VehicleClasses, r.Services, r.OnFixedRoutes = nil, nil, false
	if len(apply(r, ChargeInput{FixedRoute: true, At: t0})) != 0 {
		t.Fatal("fixed-fare flag ignored")
	}
}

func TestNightWindowWrapsMidnight(t *testing.T) {
	r := rule("N", "FLAT", "100")
	r.FromTime, r.ToTime = "22:00", "05:00"
	for _, zone := range []string{"Africa/Dar_es_Salaam", "Africa/Nairobi"} {
		loc, _ := time.LoadLocation(zone)
		at := func(h, m int) ChargeInput { return ChargeInput{At: time.Date(2026, 10, 23, h, m, 0, 0, loc)} }
		if len(apply(r, at(23, 30))) != 1 || len(apply(r, at(4, 59))) != 1 {
			t.Fatalf("%s: inside the night window must match", zone)
		}
		if len(apply(r, at(5, 0))) != 0 || len(apply(r, at(12, 0))) != 0 {
			t.Fatalf("%s: outside the night window must not match", zone)
		}
	}
	r.Days = []int{7}
	if len(apply(r, ChargeInput{At: time.Date(2026, 10, 25, 23, 0, 0, 0, time.UTC)})) != 1 { // a Sunday
		t.Fatal("Sunday must match day 7")
	}
}

func TestChargeAppliedOnce(t *testing.T) {
	both := []SourcedRule{
		{ChargeRule: rule("X", "FLAT", "1"), ProfileID: "p", MatchedAt: "PICKUP"},
		{ChargeRule: rule("X", "FLAT", "2"), ProfileID: "q", MatchedAt: "DROPOFF"},
	}
	got := ApplyCharges(both, ChargeInput{At: t0})
	if len(got) != 1 || got[0].ProfileID != "p" {
		t.Fatal("the same code must apply once per trip, from the first source")
	}
}

func TestChargeValidation(t *testing.T) {
	good := ChargeRule{Code: "AIRPORT", Label: map[string]string{"en": "Airport fee", "sw": "Ada ya uwanja"}, Category: "AIRPORT_FEE",
		AmountType: "FLAT", Amount: decimal.NewFromInt(1000), Recipient: "THIRD_PARTY", PayeeID: "x", AppliesWhen: "PICKUP"}
	if bad := chargeErrors(good); len(bad) > 0 {
		t.Fatalf("good charge refused: %v", bad)
	}
	bad := good
	bad.PayeeID, bad.Taxable, bad.Label = "", true, map[string]string{"en": "x"}
	if len(chargeErrors(bad)) < 3 {
		t.Fatalf("missing payee, taxable third party and missing sw label must each fail: %v", chargeErrors(bad))
	}
}
