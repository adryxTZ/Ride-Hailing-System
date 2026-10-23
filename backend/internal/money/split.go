package money

import (
	"fmt"

	"github.com/shopspring/decimal"
)

// Policy is the pricing profile version a trip is split under (pinned on the trip).
type Policy struct {
	Version          int
	ProfileID        string
	ProfileVersionID string
	DefaultVersionID string
	CommissionRate   decimal.Decimal // platform share of the ride fare, e.g. 0.10
	VATRate          decimal.Decimal // e.g. 0.18
	VATBase          string          // COMMISSION: VAT on platform revenue (default) | GROSS_FARE
	LevyRate         decimal.Decimal // regulator's levy as a share of the ride fare
	LevyBorneBy      string          // PLATFORM (default) | DRIVER | RIDER
}

// Breakdown is one trip's split. All amounts are inside the total the rider pays
// (VAT and levy are extracted, never added on top: the quoted price is the price).
type Breakdown struct {
	PolicyVersion  int             `json:"policy_version"`
	Currency       string          `json:"currency"`
	Total          decimal.Decimal `json:"total"`           // what the rider pays
	BookingFee     decimal.Decimal `json:"booking_fee"`     // platform revenue
	RideFare       decimal.Decimal `json:"ride_fare"`       // total − booking fee
	Commission     decimal.Decimal `json:"commission"`      // platform share of the ride fare (incl. its VAT)
	VAT            decimal.Decimal `json:"vat"`             // owed to the tax authority
	Levy           decimal.Decimal `json:"levy"`            // owed to the regulator
	LevyBorneBy    string          `json:"levy_borne_by"`   // who it comes out of
	DriverEarnings decimal.Decimal `json:"driver_earnings"` // what the driver keeps
	PlatformNet    decimal.Decimal `json:"platform_net"`    // what the platform keeps
	// The profile the split used, and the extra charges carved out of the total.
	ProfileID        string          `json:"profile_id,omitempty"`
	ProfileVersionID string          `json:"profile_version_id,omitempty"`
	DefaultVersionID string          `json:"default_version_id,omitempty"`
	Charges          []ChargeSplit   `json:"charges,omitempty"`
	ThirdParty       decimal.Decimal `json:"third_party"` // owed to payees (CHARGES_PAYABLE)
}

// Charge recipients.
const (
	ChargeToFare       = "FARE"
	ChargeToPlatform   = "PLATFORM"
	ChargeToDriver     = "DRIVER"
	ChargeToThirdParty = "THIRD_PARTY"
)

// ChargeShare is an extra charge inside the total, and who receives it.
type ChargeShare struct {
	Code      string
	Amount    decimal.Decimal
	Recipient string // FARE: split like the ride fare | PLATFORM | DRIVER | THIRD_PARTY
	PayeeID   string
	Taxable   bool // PLATFORM only: VAT is included in the amount
}

// ChargeSplit is how one charge was divided.
type ChargeSplit struct {
	Code      string          `json:"code"`
	Recipient string          `json:"recipient"`
	PayeeID   string          `json:"payee_id,omitempty"`
	Amount    decimal.Decimal `json:"amount"`
	VAT       decimal.Decimal `json:"vat"`
}

var hundredths = int32(2)

// Split divides total under policy p. Charges paid to the platform, the driver
// or a third party are carved out first; the rest (FARE charges included) is the
// ride fare split as before. The driver's earnings take the rounding remainder
// so the parts always add up to the total exactly.
func Split(total, bookingFee decimal.Decimal, currency string, p Policy, charges ...ChargeShare) (Breakdown, error) {
	carved := decimal.Zero
	for _, c := range charges {
		if c.Amount.IsNegative() {
			return Breakdown{}, fmt.Errorf("money: negative charge %s", c.Code)
		}
		if c.Recipient != ChargeToFare {
			carved = carved.Add(c.Amount.Round(hundredths))
		}
	}
	if total.IsNegative() || bookingFee.IsNegative() || bookingFee.Add(carved).GreaterThan(total) {
		return Breakdown{}, fmt.Errorf("money: invalid amounts total=%s booking_fee=%s charges=%s", total, bookingFee, carved)
	}
	b, err := splitFare(total.Sub(carved), bookingFee, currency, p)
	if err != nil {
		return Breakdown{}, err
	}
	b.Total = total.Round(hundredths)
	b.ProfileID, b.ProfileVersionID, b.DefaultVersionID = p.ProfileID, p.ProfileVersionID, p.DefaultVersionID
	for _, c := range charges {
		addCharge(&b, c, p.VATRate)
	}
	return b, checkSum(b)
}

// splitFare divides the ride fare and booking fee (no carved charges) under p.
func splitFare(total, bookingFee decimal.Decimal, currency string, p Policy) (Breakdown, error) {
	b := Breakdown{PolicyVersion: p.Version, Currency: currency, Total: total.Round(hundredths), BookingFee: bookingFee.Round(hundredths), LevyBorneBy: orDefault(p.LevyBorneBy, "PLATFORM")}
	b.RideFare = b.Total.Sub(b.BookingFee)
	b.Levy = b.RideFare.Mul(p.LevyRate).Round(hundredths)
	base := b.RideFare
	if b.LevyBorneBy == "RIDER" { // included in the price, so it comes off before the split
		base = base.Sub(b.Levy)
	}
	b.Commission = base.Mul(p.CommissionRate).Round(hundredths)
	b.VAT = extract(vatBase(b, p.VATBase), p.VATRate)
	b.PlatformNet = b.Commission.Add(b.BookingFee).Sub(b.VAT)
	if b.LevyBorneBy == "PLATFORM" {
		b.PlatformNet = b.PlatformNet.Sub(b.Levy)
	}
	b.DriverEarnings = b.Total.Sub(b.PlatformNet).Sub(b.VAT).Sub(b.Levy)
	if b.PlatformNet.IsNegative() || b.DriverEarnings.IsNegative() {
		return Breakdown{}, fmt.Errorf("money: policy makes a share negative (platform %s, driver %s)", b.PlatformNet, b.DriverEarnings)
	}
	return b, nil
}

// addCharge credits one charge to its recipient (FARE charges are already inside the ride fare).
func addCharge(b *Breakdown, c ChargeShare, vatRate decimal.Decimal) {
	amount := c.Amount.Round(hundredths)
	cs := ChargeSplit{Code: c.Code, Recipient: c.Recipient, PayeeID: c.PayeeID, Amount: amount, VAT: decimal.Zero}
	switch c.Recipient {
	case ChargeToPlatform:
		if c.Taxable {
			cs.VAT = extract(amount, vatRate)
		}
		b.PlatformNet = b.PlatformNet.Add(amount.Sub(cs.VAT))
		b.VAT = b.VAT.Add(cs.VAT)
	case ChargeToDriver:
		b.DriverEarnings = b.DriverEarnings.Add(amount)
	case ChargeToThirdParty:
		b.ThirdParty = b.ThirdParty.Add(amount)
	}
	b.Charges = append(b.Charges, cs)
}

// checkSum asserts the invariant on every split: the parts are the total, none negative.
func checkSum(b Breakdown) error {
	if b.PlatformNet.IsNegative() || b.DriverEarnings.IsNegative() || b.ThirdParty.IsNegative() {
		return fmt.Errorf("money: a share is negative for total %s", b.Total)
	}
	if !b.DriverEarnings.Add(b.PlatformNet).Add(b.VAT).Add(b.Levy).Add(b.ThirdParty).Equal(b.Total) {
		return fmt.Errorf("money: split does not add up for total %s", b.Total)
	}
	return nil
}

// vatBase is what VAT is charged on: the platform's revenue (agent model) or the whole fare.
func vatBase(b Breakdown, base string) decimal.Decimal {
	if base == "GROSS_FARE" {
		return b.Total
	}
	return b.Commission.Add(b.BookingFee)
}

// extract returns the VAT contained in a VAT-inclusive amount: amount × r / (1 + r).
func extract(amount, rate decimal.Decimal) decimal.Decimal {
	if rate.IsZero() {
		return decimal.Zero
	}
	return amount.Mul(rate).Div(decimal.NewFromInt(1).Add(rate)).Round(hundredths)
}

func orDefault(v, d string) string {
	if v == "" {
		return d
	}
	return v
}
