package profiles

import (
	"errors"
	"testing"
)

func TestMergeInheritsNulls(t *testing.T) {
	e := Merge(defaultTerms("0.10"), Terms{BookingFee: dec("200")})
	if !e.CommissionRate.Equal(*dec("0.10")) || !e.BookingFee.Equal(*dec("200")) || e.VATBase != "COMMISSION" {
		t.Fatalf("merge = %+v", e)
	}
}

func TestValidateTerms(t *testing.T) {
	if err := ValidateTerms(defaultTerms("0.10"), true); err != nil {
		t.Fatalf("complete default refused: %v", err)
	}
	if err := ValidateTerms(Terms{CommissionRate: dec("0.10")}, true); !errors.Is(err, ErrIncomplete) {
		t.Fatalf("incomplete default = %v", err)
	}
	if err := ValidateTerms(Terms{VATRate: dec("0.2")}, false); !errors.Is(err, ErrCountryOnly) {
		t.Fatalf("region with VAT = %v", err)
	}
	if err := ValidateTerms(Terms{CommissionRate: dec("1.2")}, false); err == nil {
		t.Fatal("commission above 1 accepted")
	}
	bad := defaultTerms("0.10")
	bad.DebtHardLimit = dec("10")
	if err := ValidateTerms(bad, true); err == nil {
		t.Fatal("hard limit below soft limit accepted")
	}
}

func TestContentHashIsStable(t *testing.T) {
	a, b := ContentHash(defaultTerms("0.10"), nil), ContentHash(defaultTerms("0.10"), nil)
	if a != b || a == ContentHash(defaultTerms("0.11"), nil) {
		t.Fatal("hash must depend on content only")
	}
}
