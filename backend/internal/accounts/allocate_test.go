package accounts

import (
	"errors"
	"testing"

	"github.com/shopspring/decimal"
)

func d(s string) decimal.Decimal { return decimal.RequireFromString(s) }

// owedFixture: rider A is owed 300 then 200, rider B 500 (oldest first).
var owedFixture = []OwedRefund{
	{ID: "r1", RiderID: "A", Amount: d("300")},
	{ID: "r2", RiderID: "B", Amount: d("500")},
	{ID: "r3", RiderID: "A", Amount: d("200")},
}

func TestOldestFirstPaysWholeRefunds(t *testing.T) {
	a, err := allocateOldest(owedFixture, d("800"))
	if err != nil {
		t.Fatal(err)
	}
	if len(a.refundIDs) != 2 || !a.perRider["A"].Equal(d("300")) || !a.perRider["B"].Equal(d("500")) {
		t.Fatalf("got %+v", a)
	}
	legs := a.payables()
	if len(legs) != 2 || legs[0].OwnerID != "A" {
		t.Fatalf("legs must be ordered by rider: %+v", legs)
	}
}

func TestOldestFirstRefusesPartialAndOverpay(t *testing.T) {
	if _, err := allocateOldest(owedFixture, d("400")); !errors.Is(err, ErrAmountMismatch) {
		t.Errorf("a partial refund must be refused, got %v", err)
	}
	if _, err := allocateOldest(owedFixture, d("1000.01")); !errors.Is(err, ErrOverpay) {
		t.Errorf("more than owed must be refused, got %v", err)
	}
}

func TestChosenMustMatchAndBeOwed(t *testing.T) {
	chosen := []OwedRefund{owedFixture[2]}
	if a, err := allocateChosen(chosen, []string{"r3"}, d("200")); err != nil || len(a.refundIDs) != 1 {
		t.Fatalf("got %+v %v", a, err)
	}
	if _, err := allocateChosen(chosen, []string{"r3"}, d("150")); !errors.Is(err, ErrAmountMismatch) {
		t.Errorf("a different amount must be refused, got %v", err)
	}
	if _, err := allocateChosen(chosen, []string{"r3", "gone"}, d("200")); !errors.Is(err, ErrRefundNotOwed) {
		t.Errorf("a paid or unknown refund must be refused, got %v", err)
	}
}

func TestRiderBalanceMarksCoveredRefunds(t *testing.T) {
	mine := []OwedRefund{owedFixture[0], owedFixture[2]}
	a, err := allocateRider(mine, "A", d("1500"), d("400")) // 300 covered in full, 200 not
	if err != nil {
		t.Fatal(err)
	}
	if len(a.refundIDs) != 1 || a.refundIDs[0] != "r1" || !a.perRider["A"].Equal(d("400")) {
		t.Fatalf("got %+v", a)
	}
	if _, err := allocateRider(mine, "A", d("1500"), d("1500.01")); !errors.Is(err, ErrOverpay) {
		t.Errorf("more than the rider is owed must be refused, got %v", err)
	}
}
