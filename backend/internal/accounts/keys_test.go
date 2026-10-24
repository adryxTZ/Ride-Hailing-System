package accounts

import (
	"errors"
	"testing"
)

func TestKeysRoundTrip(t *testing.T) {
	payee := "6b425043-f04b-4a83-8a4e-c769168ea86a"
	for _, c := range []struct{ kind, payee string }{{"VAT_PAYABLE", ""}, {"CHARGES_PAYABLE", payee}, {"DRIVER_WALLET", ""}} {
		kind, p, err := ParseKey(KeyOf(c.kind, c.payee))
		if err != nil || kind != c.kind || p != c.payee {
			t.Errorf("%s/%s: got %s %s %v", c.kind, c.payee, kind, p, err)
		}
	}
}

func TestParseKeyRefusesUnknownAndMalformed(t *testing.T) {
	for _, key := range []string{"", "NOPE", "CHARGES_PAYABLE", "CHARGES_PAYABLE:not-a-uuid", "VAT_PAYABLE:6b425043-f04b-4a83-8a4e-c769168ea86a"} {
		if _, _, err := ParseKey(key); !errors.Is(err, ErrAccountNotFound) {
			t.Errorf("%q: got %v", key, err)
		}
	}
}

func TestOnlyPayablesAreSettleable(t *testing.T) {
	want := map[string]bool{"VAT_PAYABLE": true, "LEVY_PAYABLE": true, "CHARGES_PAYABLE": true, "RIDER_REFUNDS": true}
	for kind, k := range kinds {
		if k.settleable != want[kind] {
			t.Errorf("%s settleable = %v", kind, k.settleable)
		}
		if k.settleable && k.position != PositionWeOwe {
			t.Errorf("%s is settleable but not something we owe", kind)
		}
	}
}
