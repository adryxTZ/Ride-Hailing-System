package profiles

import (
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

var t0 = time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)

func dec(s string) *decimal.Decimal { d := decimal.RequireFromString(s); return &d }
func str(s string) *string          { return &s }
func num(i int) *int                { return &i }
func yes(b bool) *bool              { return &b }

// defaultTerms is a complete country default.
func defaultTerms(commission string) Terms {
	return Terms{CommissionRate: dec(commission), BookingFee: dec("0"), WaitingCapRate: dec("0"), FareRounding: dec("100"), CancelFreeS: num(120),
		NoShowAfterS: num(300), CancelDriverShare: dec("0.8"), VATRate: dec("0.18"), VATBase: str("COMMISSION"), LevyRate: dec("0"),
		LevyBorneBy: str("PLATFORM"), DebtSoftLimit: dec("1000"), DebtHardLimit: dec("5000"), DebtGraceDays: num(7), PayinMinAmount: dec("1000"),
		PayoutMinAmount: dec("5000"), PayoutAutoApprove: yes(false)}
}

// snapshot builds a country default (commission 0.10) and a region profile (0.15) in the given state.
func snapshot(regionStatus string, regionVersionAt time.Time) *Snapshot {
	s := &Snapshot{Profiles: map[string]SnapProfile{}, Versions: map[string][]Version{}, Open: map[string]Scope{}}
	s.Profiles["def"] = SnapProfile{ID: "def", Country: "ZZ", Name: "Default", Status: StatusActive}
	s.Versions["def"] = []Version{{ID: "dv1", Version: 1, EffectiveFrom: t0.Add(-time.Hour), Terms: defaultTerms("0.10")}}
	s.Open["COUNTRY/ZZ"] = Scope{ProfileID: "def", Kind: ScopeCountry, ID: "ZZ", ValidFrom: t0.Add(-2 * time.Hour)}
	s.Profiles["reg"] = SnapProfile{ID: "reg", Country: "ZZ", Name: "Region", Status: regionStatus}
	s.Versions["reg"] = []Version{{ID: "rv1", Version: 1, EffectiveFrom: regionVersionAt, Terms: Terms{CommissionRate: dec("0.15")},
		Charges: []ChargeRule{{Code: "TOLL", AppliesWhen: "PICKUP"}, {Code: "DROP", AppliesWhen: "DROPOFF"}}}}
	s.Open["AREA/a1"] = Scope{ProfileID: "reg", Kind: ScopeArea, ID: "a1", ValidFrom: t0.Add(-time.Hour)}
	return s
}

func TestResolveRegionBeatsDefault(t *testing.T) {
	res, err := Resolve(snapshot(StatusActive, t0.Add(-time.Minute)), Input{Country: "ZZ", AreaID: "a1", AreaName: "Region", At: t0})
	if err != nil {
		t.Fatal(err)
	}
	if res.Profile.ID != "reg" || res.Scope.Kind != ScopeArea {
		t.Fatalf("winner = %+v", res.Profile)
	}
	if !res.Terms.CommissionRate.Equal(decimal.RequireFromString("0.15")) || !res.Terms.VATRate.Equal(decimal.RequireFromString("0.18")) {
		t.Fatalf("terms not merged over the default: %+v", res.Terms)
	}
	if res.Default.Version != 1 || len(res.Charges) != 1 || res.Charges[0].Code != "TOLL" {
		t.Fatalf("default/charges = %+v %+v", res.Default, res.Charges)
	}
}

func TestResolveSkipsIneligible(t *testing.T) {
	cases := []struct {
		name    string
		status  string
		at      time.Time
		outcome string
	}{
		{"inactive", StatusInactive, t0.Add(-time.Minute), OutcomeInactive},
		{"draft", StatusDraft, t0.Add(-time.Minute), OutcomeDraft},
		{"future version", StatusActive, t0.Add(time.Hour), OutcomeNoVersion},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res, err := Resolve(snapshot(c.status, c.at), Input{Country: "ZZ", AreaID: "a1", At: t0})
			if err != nil {
				t.Fatal(err)
			}
			if res.Profile.ID != "def" || res.Trace.WinnerRule != winnerRuleDefault {
				t.Fatalf("an ineligible region priced the trip: %+v", res.Profile)
			}
			if got := outcomeOf(res.Trace, "reg"); got != c.outcome {
				t.Fatalf("region outcome = %s, want %s", got, c.outcome)
			}
		})
	}
}

func TestResolveOutOfWindow(t *testing.T) {
	s := snapshot(StatusActive, t0.Add(-time.Minute))
	until := t0
	p := s.Profiles["reg"]
	p.ActiveUntil = &until
	s.Profiles["reg"] = p
	res, _ := Resolve(s, Input{Country: "ZZ", AreaID: "a1", At: t0})
	if res.Profile.ID != "def" || outcomeOf(res.Trace, "reg") != OutcomeOutWindow {
		t.Fatalf("a profile past its window priced the trip")
	}
}

func TestResolveFutureVersionTakesEffectOnTime(t *testing.T) {
	s := snapshot(StatusActive, t0.Add(-time.Hour))
	s.Versions["reg"] = append([]Version{{ID: "rv2", Version: 2, EffectiveFrom: t0.Add(time.Hour), Terms: Terms{CommissionRate: dec("0.20")}}}, s.Versions["reg"]...)
	before, _ := Resolve(s, Input{Country: "ZZ", AreaID: "a1", At: t0})
	after, _ := Resolve(s, Input{Country: "ZZ", AreaID: "a1", At: t0.Add(2 * time.Hour)})
	if before.Profile.Version != 1 || after.Profile.Version != 2 {
		t.Fatalf("versions = %d then %d, want 1 then 2", before.Profile.Version, after.Profile.Version)
	}
}

func TestResolveNeedsDefault(t *testing.T) {
	if _, err := Resolve(snapshot(StatusActive, t0), Input{Country: "YY", At: t0}); !errors.Is(err, ErrNoDefault) {
		t.Fatalf("no default = %v", err)
	}
}

func outcomeOf(tr Trace, profileID string) string {
	for _, c := range tr.Candidates {
		if c.Profile.ID == profileID {
			return c.Outcome
		}
	}
	return ""
}
