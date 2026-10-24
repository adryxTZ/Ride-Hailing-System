package profiles

import (
	"context"
	"testing"
)

// TestChargeDefaults: tolls and parking go to the driver out of the box; a staff
// change wins; a charge saved without a recipient takes its kind's default.
func TestChargeDefaults(t *testing.T) {
	ctx := context.Background()
	pool, _, brand := liveRecord(t)
	s := New(pool, brand, nil, nil)
	defs, err := s.ChargeDefaults(ctx, "KE") // a country no test changes: built-ins only
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, d := range defs {
		got[d.Category] = d.Recipient
	}
	if got["TOLL"] != "DRIVER" || got["PARKING"] != "DRIVER" || len(defs) != len(ChargeCategories) {
		t.Fatalf("built-in defaults: %v", got)
	}
	if _, err := s.SetChargeDefault(ctx, "", "KE", "NIGHT", "PLATFORM", "", "test"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM charge_category_defaults WHERE country_code='KE' AND category='NIGHT'`)
	})
	filled, err := s.withDefaultRecipients(ctx, "KE", []ChargeRule{{Category: "NIGHT"}, {Category: "TOLL"}, {Category: "PARKING", Recipient: "FARE"}})
	if err != nil {
		t.Fatal(err)
	}
	if filled[0].Recipient != "PLATFORM" || filled[1].Recipient != "DRIVER" || filled[2].Recipient != "FARE" {
		t.Fatalf("filled: %+v", filled)
	}
	if _, err := s.SetChargeDefault(ctx, "", "KE", "LEVY", "THIRD_PARTY", "", "test"); err == nil {
		t.Fatal("a payee is required for THIRD_PARTY")
	}
}
