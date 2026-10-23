package money

import (
	"testing"

	"github.com/shopspring/decimal"
)

var d = decimal.RequireFromString

func sum(b Breakdown) decimal.Decimal {
	return b.DriverEarnings.Add(b.PlatformNet).Add(b.VAT).Add(b.Levy)
}

func TestDefaultPolicyOnATenThousandFare(t *testing.T) {
	// 10% commission, 18% VAT on platform revenue, no levy, no booking fee.
	b, err := Split(d("10000"), d("0"), "TZS", Policy{CommissionRate: d("0.10"), VATRate: d("0.18"), VATBase: "COMMISSION", LevyBorneBy: "PLATFORM"})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"commission": "1000", "vat": "152.54", "platform": "847.46", "driver": "9000"}
	got := map[string]string{"commission": b.Commission.String(), "vat": b.VAT.String(), "platform": b.PlatformNet.String(), "driver": b.DriverEarnings.String()}
	for k := range want {
		if want[k] != got[k] {
			t.Fatalf("%s: want %s got %s (%+v)", k, want[k], got[k], b)
		}
	}
	if !sum(b).Equal(b.Total) {
		t.Fatal("parts must add up to the total")
	}
}

func TestBookingFeeAndLevyBorneByPlatform(t *testing.T) {
	b, err := Split(d("4283.77"), d("500"), "TZS", Policy{CommissionRate: d("0.10"), VATRate: d("0.18"), LevyRate: d("0.01"), LevyBorneBy: "PLATFORM"})
	if err != nil {
		t.Fatal(err)
	}
	if !b.RideFare.Equal(d("3783.77")) || !b.Levy.Equal(d("37.84")) {
		t.Fatalf("ride fare / levy wrong: %+v", b)
	}
	// The driver is not touched by the levy or the booking fee: fare − commission.
	if !b.DriverEarnings.Equal(b.RideFare.Sub(b.Commission)) {
		t.Fatalf("driver earnings %s, want %s", b.DriverEarnings, b.RideFare.Sub(b.Commission))
	}
	if !sum(b).Equal(b.Total) {
		t.Fatal("parts must add up to the total")
	}
}

func TestLevyBorneByDriver(t *testing.T) {
	b, err := Split(d("10000"), d("0"), "TZS", Policy{CommissionRate: d("0.10"), VATRate: d("0.18"), LevyRate: d("0.02"), LevyBorneBy: "DRIVER"})
	if err != nil {
		t.Fatal(err)
	}
	if !b.DriverEarnings.Equal(d("8800")) {
		t.Fatalf("driver pays the levy: want 8800, got %s", b.DriverEarnings)
	}
	if !sum(b).Equal(b.Total) {
		t.Fatal("parts must add up to the total")
	}
}

func TestRejectsAPolicyThatMakesThePlatformLoseMoney(t *testing.T) {
	// VAT on the whole fare at 18% is more than a 10% commission: refused.
	if _, err := Split(d("10000"), d("0"), "TZS", Policy{CommissionRate: d("0.10"), VATRate: d("0.18"), VATBase: "GROSS_FARE"}); err == nil {
		t.Fatal("expected an error")
	}
}

func TestOddAmountsAlwaysAddUp(t *testing.T) {
	p := Policy{CommissionRate: d("0.125"), VATRate: d("0.18"), LevyRate: d("0.015"), LevyBorneBy: "PLATFORM"}
	for _, total := range []string{"0", "1", "999.99", "3185.17", "6000.13", "8244.4", "1234567.89"} {
		b, err := Split(d(total), d("0"), "TZS", p)
		if err != nil {
			t.Fatalf("%s: %v", total, err)
		}
		if !sum(b).Equal(b.Total) {
			t.Fatalf("%s: parts %s != total %s", total, sum(b), b.Total)
		}
	}
}

func TestSplitWithCharges(t *testing.T) {
	p := Policy{CommissionRate: decimal.RequireFromString("0.10"), VATRate: decimal.RequireFromString("0.18"), VATBase: "COMMISSION"}
	charges := []ChargeShare{
		{Code: "LEVY", Amount: decimal.NewFromInt(500), Recipient: ChargeToThirdParty, PayeeID: "payee"},
		{Code: "PARKING", Amount: decimal.NewFromInt(300), Recipient: ChargeToDriver},
		{Code: "SERVICE", Amount: decimal.NewFromInt(118), Recipient: ChargeToPlatform, Taxable: true},
		{Code: "NIGHT", Amount: decimal.NewFromInt(1000), Recipient: ChargeToFare},
	}
	b, err := Split(decimal.NewFromInt(10918), decimal.Zero, "TZS", p, charges...)
	if err != nil {
		t.Fatal(err)
	}
	if !b.RideFare.Equal(decimal.NewFromInt(10000)) || !b.ThirdParty.Equal(decimal.NewFromInt(500)) {
		t.Fatalf("ride fare %s, third party %s", b.RideFare, b.ThirdParty)
	}
	if !b.Commission.Equal(decimal.NewFromInt(1000)) { // FARE charges are split like the fare
		t.Fatalf("commission = %s", b.Commission)
	}
	if b.Charges[2].VAT.String() != "18" {
		t.Fatalf("platform charge VAT = %s", b.Charges[2].VAT)
	}
	if _, err := Split(decimal.NewFromInt(500), decimal.Zero, "TZS", p, charges...); err == nil {
		t.Fatal("charges larger than the total must be refused")
	}
}

func TestSplitAlwaysAddsUp(t *testing.T) {
	p := Policy{CommissionRate: decimal.RequireFromString("0.13"), VATRate: decimal.RequireFromString("0.18"), VATBase: "COMMISSION",
		LevyRate: decimal.RequireFromString("0.01"), LevyBorneBy: "RIDER"}
	for i := int64(0); i < 10_000; i++ {
		total := decimal.NewFromInt(2000 + i*7).Add(decimal.New(i%100, -2))
		charges := []ChargeShare{
			{Code: "A", Amount: decimal.NewFromInt(i % 400), Recipient: ChargeToThirdParty, PayeeID: "p"},
			{Code: "B", Amount: decimal.NewFromInt(i % 250), Recipient: ChargeToDriver},
			{Code: "C", Amount: decimal.NewFromInt(i % 90), Recipient: ChargeToPlatform, Taxable: i%2 == 0},
		}
		b, err := Split(total, decimal.NewFromInt(i%50), "TZS", p, charges...)
		if err != nil {
			t.Fatalf("total %s: %v", total, err)
		}
		if !b.DriverEarnings.Add(b.PlatformNet).Add(b.VAT).Add(b.Levy).Add(b.ThirdParty).Equal(b.Total) {
			t.Fatalf("total %s does not add up", total)
		}
	}
}
