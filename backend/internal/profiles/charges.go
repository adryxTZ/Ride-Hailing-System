package profiles

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// ChargeRule is one extra charge a profile version adds to matching trips.
type ChargeRule struct {
	Code           string            `json:"code"`
	Label          map[string]string `json:"label"` // {en, sw}
	Category       string            `json:"category"`
	AmountType     string            `json:"amount_type"` // FLAT | PER_KM | PERCENT_OF_FARE
	Amount         decimal.Decimal   `json:"amount"`
	MinAmount      *decimal.Decimal  `json:"min_amount,omitempty"`
	MaxAmount      *decimal.Decimal  `json:"max_amount,omitempty"`
	Recipient      string            `json:"recipient"` // FARE | PLATFORM | DRIVER | THIRD_PARTY
	PayeeID        string            `json:"payee_id,omitempty"`
	Taxable        bool              `json:"taxable"`
	AppliesWhen    string            `json:"applies_when"`
	Services       []string          `json:"services"`
	VehicleClasses []string          `json:"vehicle_classes"`
	OnFixedRoutes  bool              `json:"on_fixed_routes"`
	Days           []int             `json:"days"`      // ISO weekdays 1–7; empty = every day
	FromTime       string            `json:"from_time"` // HH:MM local, with ToTime; may wrap midnight
	ToTime         string            `json:"to_time"`
}

// Allowed values.
var (
	ChargeCategories = []string{"TOLL", "PARKING", "LEVY", "AIRPORT_FEE", "ENTRY_FEE", "NIGHT", "OTHER"}
	AmountTypes      = []string{"FLAT", "PER_KM", "PERCENT_OF_FARE"}
	Recipients       = []string{"FARE", "PLATFORM", "DRIVER", "THIRD_PARTY"}
	Conditions       = []string{"PICKUP", "DROPOFF", "EITHER", "BOTH", "ROUTE_CROSSES"}
)

// Label bounds.
const labelMaxLen = 40

var (
	chargeCode = regexp.MustCompile(`^[A-Z][A-Z0-9_]{1,31}$`)
	clockTime  = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)
)

// chargeErrors checks one rule's shape (payees and catalogue keys are checked by the service).
func chargeErrors(c ChargeRule) []map[string]string {
	var bad []map[string]string
	flag := func(path, code string) { bad = append(bad, map[string]string{"path": path, "code": code}) }
	if !chargeCode.MatchString(c.Code) {
		flag("code", "FORMAT")
	}
	for _, lang := range []string{"en", "sw"} {
		if l := strings.TrimSpace(c.Label[lang]); l == "" || len([]rune(l)) > labelMaxLen {
			flag("label."+lang, "LENGTH")
		}
	}
	if !slices.Contains(ChargeCategories, c.Category) {
		flag("category", "NOT_ALLOWED")
	}
	if !slices.Contains(AmountTypes, c.AmountType) {
		flag("amount_type", "NOT_ALLOWED")
	}
	if c.Amount.IsNegative() || (c.AmountType == "PERCENT_OF_FARE" && c.Amount.GreaterThan(one)) {
		flag("amount", "OUT_OF_RANGE")
	}
	if (c.MinAmount != nil && c.MinAmount.IsNegative()) || (c.MaxAmount != nil && c.MaxAmount.IsNegative()) {
		flag("min_amount", "NEGATIVE")
	}
	if c.FromTime != "" && c.FromTime == c.ToTime {
		flag("to_time", "SAME_AS_FROM") // a whole day is no time window at all
	}
	if c.MinAmount != nil && c.MaxAmount != nil && c.MaxAmount.LessThan(*c.MinAmount) {
		flag("max_amount", "BELOW_MIN")
	}
	if !slices.Contains(Recipients, c.Recipient) {
		flag("recipient", "NOT_ALLOWED")
	}
	if (c.Recipient == "THIRD_PARTY") != (c.PayeeID != "") {
		flag("payee_id", "REQUIRED_FOR_THIRD_PARTY_ONLY")
	}
	if c.Taxable && c.Recipient != "PLATFORM" {
		flag("taxable", "PLATFORM_ONLY")
	}
	if !slices.Contains(Conditions, c.AppliesWhen) {
		flag("applies_when", "NOT_ALLOWED")
	}
	for _, d := range c.Days {
		if d < 1 || d > 7 {
			flag("days", "OUT_OF_RANGE")
		}
	}
	if (c.FromTime == "") != (c.ToTime == "") || (c.FromTime != "" && (!clockTime.MatchString(c.FromTime) || !clockTime.MatchString(c.ToTime))) {
		flag("from_time", "FORMAT")
	}
	return bad
}

// ChargeInput is what a charge is evaluated against, for one vehicle class.
type ChargeInput struct {
	Service    string
	Class      string
	FixedRoute bool
	DistanceM  int
	FareBase   decimal.Decimal // the option's fare after surge and minimum, before booking fee, charges and rounding
	At         time.Time       // quote time, already in the country's timezone
}

// metresPerKm converts route metres to kilometres for PER_KM charges.
var metresPerKm = decimal.NewFromInt(1000)

// ApplyCharges returns the rules that match, with amounts, in source order.
// A code applies at most once per trip: the first source wins.
func ApplyCharges(rules []SourcedRule, in ChargeInput) []AppliedCharge {
	out := []AppliedCharge{}
	seen := map[string]bool{}
	for _, r := range rules {
		if seen[r.Code] || !matches(r.ChargeRule, in) {
			continue
		}
		amount := amountOf(r.ChargeRule, in)
		if !amount.IsPositive() {
			continue
		}
		seen[r.Code] = true
		out = append(out, AppliedCharge{Code: r.Code, Label: r.Label, Category: r.Category, Recipient: r.Recipient, PayeeID: r.PayeeID,
			Taxable: r.Taxable, AppliesWhen: r.AppliesWhen, MatchedAt: r.MatchedAt, ProfileID: r.ProfileID, Version: r.Version, Amount: amount,
			PayeeName: r.PayeeName})
	}
	return out
}

// matches applies the service, class, fixed-fare, weekday and time-window filters.
func matches(r ChargeRule, in ChargeInput) bool {
	if len(r.Services) > 0 && !slices.Contains(r.Services, in.Service) {
		return false
	}
	if len(r.VehicleClasses) > 0 && !slices.Contains(r.VehicleClasses, in.Class) {
		return false
	}
	if in.FixedRoute && !r.OnFixedRoutes {
		return false
	}
	if len(r.Days) > 0 && !slices.Contains(r.Days, isoWeekday(in.At)) {
		return false
	}
	return inWindow(r.FromTime, r.ToTime, in.At)
}

func amountOf(r ChargeRule, in ChargeInput) decimal.Decimal {
	var a decimal.Decimal
	switch r.AmountType {
	case "PER_KM":
		a = r.Amount.Mul(decimal.NewFromInt(int64(in.DistanceM))).Div(metresPerKm)
	case "PERCENT_OF_FARE":
		a = r.Amount.Mul(in.FareBase)
	default:
		a = r.Amount
	}
	if r.MinAmount != nil && a.LessThan(*r.MinAmount) {
		a = *r.MinAmount
	}
	if r.MaxAmount != nil && a.GreaterThan(*r.MaxAmount) {
		a = *r.MaxAmount
	}
	return a.Round(2)
}

func isoWeekday(t time.Time) int {
	if d := int(t.Weekday()); d != 0 {
		return d
	}
	return 7 // Sunday
}

// inWindow reports whether t's local time is in [from, to); a window may wrap
// midnight, and its weekday (days) is the day the window starts on.
func inWindow(from, to string, t time.Time) bool {
	if from == "" {
		return true
	}
	f, e, now := minuteOf(from), minuteOf(to), t.Hour()*60+t.Minute()
	if f <= e {
		return now >= f && now < e
	}
	return now >= f || now < e
}

func minuteOf(hhmm string) int {
	h, _ := strconv.Atoi(hhmm[:2])
	m, _ := strconv.Atoi(hhmm[3:])
	return h*60 + m
}
