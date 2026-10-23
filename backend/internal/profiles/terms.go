package profiles

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
)

// Terms are a version's money rules. On a non-default profile a nil trip rule
// means "inherit the default's", and country-only rules are always nil.
type Terms struct {
	CommissionRate    *decimal.Decimal `json:"commission_rate"`
	BookingFee        *decimal.Decimal `json:"booking_fee"`
	WaitingCapRate    *decimal.Decimal `json:"waiting_cap_rate"`
	FareRounding      *decimal.Decimal `json:"fare_rounding"`
	CancelFreeS       *int             `json:"cancel_free_s"`
	NoShowAfterS      *int             `json:"no_show_after_s"`
	CancelDriverShare *decimal.Decimal `json:"cancel_driver_share"`
	// Country-only (the default profile's).
	VATRate           *decimal.Decimal `json:"vat_rate"`
	VATBase           *string          `json:"vat_base"`
	LevyRate          *decimal.Decimal `json:"levy_rate"`
	LevyBorneBy       *string          `json:"levy_borne_by"`
	DebtSoftLimit     *decimal.Decimal `json:"debt_soft_limit"`
	DebtHardLimit     *decimal.Decimal `json:"debt_hard_limit"`
	DebtGraceDays     *int             `json:"debt_soft_grace_days"`
	PayinMinAmount    *decimal.Decimal `json:"payin_min_amount"`
	PayoutMinAmount   *decimal.Decimal `json:"payout_min_amount"`
	PayoutAutoApprove *bool            `json:"payout_auto_approve"`
}

// Effective is a complete set of rules (a profile's, merged over the default's).
type Effective struct {
	CommissionRate    decimal.Decimal `json:"commission_rate"`
	BookingFee        decimal.Decimal `json:"booking_fee"`
	WaitingCapRate    decimal.Decimal `json:"waiting_cap_rate"`
	FareRounding      decimal.Decimal `json:"fare_rounding"`
	CancelFreeS       int             `json:"cancel_free_s"`
	NoShowAfterS      int             `json:"no_show_after_s"`
	CancelDriverShare decimal.Decimal `json:"cancel_driver_share"`
	VATRate           decimal.Decimal `json:"vat_rate"`
	VATBase           string          `json:"vat_base"`
	LevyRate          decimal.Decimal `json:"levy_rate"`
	LevyBorneBy       string          `json:"levy_borne_by"`
	DebtSoftLimit     decimal.Decimal `json:"debt_soft_limit"`
	DebtHardLimit     decimal.Decimal `json:"debt_hard_limit"`
	DebtGraceDays     int             `json:"debt_soft_grace_days"`
	PayinMinAmount    decimal.Decimal `json:"payin_min_amount"`
	PayoutMinAmount   decimal.Decimal `json:"payout_min_amount"`
	PayoutAutoApprove bool            `json:"payout_auto_approve"`
}

// Merge fills own's unset trip rules from def; country-only rules always come from def.
func Merge(def, own Terms) Effective {
	e := effectiveOf(def)
	pickDec(&e.CommissionRate, own.CommissionRate)
	pickDec(&e.BookingFee, own.BookingFee)
	pickDec(&e.WaitingCapRate, own.WaitingCapRate)
	pickDec(&e.FareRounding, own.FareRounding)
	pickInt(&e.CancelFreeS, own.CancelFreeS)
	pickInt(&e.NoShowAfterS, own.NoShowAfterS)
	pickDec(&e.CancelDriverShare, own.CancelDriverShare)
	return e
}

// effectiveOf reads a complete (default) version; missing values are zero.
func effectiveOf(t Terms) Effective {
	var e Effective
	pickDec(&e.CommissionRate, t.CommissionRate)
	pickDec(&e.BookingFee, t.BookingFee)
	pickDec(&e.WaitingCapRate, t.WaitingCapRate)
	pickDec(&e.FareRounding, t.FareRounding)
	pickInt(&e.CancelFreeS, t.CancelFreeS)
	pickInt(&e.NoShowAfterS, t.NoShowAfterS)
	pickDec(&e.CancelDriverShare, t.CancelDriverShare)
	pickDec(&e.VATRate, t.VATRate)
	pickStr(&e.VATBase, t.VATBase)
	pickDec(&e.LevyRate, t.LevyRate)
	pickStr(&e.LevyBorneBy, t.LevyBorneBy)
	pickDec(&e.DebtSoftLimit, t.DebtSoftLimit)
	pickDec(&e.DebtHardLimit, t.DebtHardLimit)
	pickInt(&e.DebtGraceDays, t.DebtGraceDays)
	pickDec(&e.PayinMinAmount, t.PayinMinAmount)
	pickDec(&e.PayoutMinAmount, t.PayoutMinAmount)
	if t.PayoutAutoApprove != nil {
		e.PayoutAutoApprove = *t.PayoutAutoApprove
	}
	return e
}

func pickDec(dst *decimal.Decimal, v *decimal.Decimal) {
	if v != nil {
		*dst = *v
	}
}

func pickInt(dst *int, v *int) {
	if v != nil {
		*dst = *v
	}
}

func pickStr(dst *string, v *string) {
	if v != nil {
		*dst = *v
	}
}

// countryOnly lists the country-only fields a terms value sets.
func countryOnly(t Terms) []string {
	var out []string
	set := map[string]bool{"vat_rate": t.VATRate != nil, "vat_base": t.VATBase != nil, "levy_rate": t.LevyRate != nil,
		"levy_borne_by": t.LevyBorneBy != nil, "debt_soft_limit": t.DebtSoftLimit != nil, "debt_hard_limit": t.DebtHardLimit != nil,
		"debt_soft_grace_days": t.DebtGraceDays != nil, "payin_min_amount": t.PayinMinAmount != nil,
		"payout_min_amount": t.PayoutMinAmount != nil, "payout_auto_approve": t.PayoutAutoApprove != nil}
	for _, k := range countryOnlyFields {
		if set[k] {
			out = append(out, k)
		}
	}
	return out
}

var countryOnlyFields = []string{"vat_rate", "vat_base", "levy_rate", "levy_borne_by", "debt_soft_limit", "debt_hard_limit",
	"debt_soft_grace_days", "payin_min_amount", "payout_min_amount", "payout_auto_approve"}

// missing lists the fields a default profile must set but does not.
func missing(t Terms) []string {
	var out []string
	if t.CommissionRate == nil {
		out = append(out, "commission_rate")
	}
	if t.BookingFee == nil {
		out = append(out, "booking_fee")
	}
	if t.WaitingCapRate == nil {
		out = append(out, "waiting_cap_rate")
	}
	if t.FareRounding == nil {
		out = append(out, "fare_rounding")
	}
	if t.CancelFreeS == nil || t.NoShowAfterS == nil || t.CancelDriverShare == nil {
		out = append(out, "cancellation")
	}
	set := countryOnly(t)
	if len(set) < len(countryOnlyFields) {
		out = append(out, "country_rules")
	}
	return out
}

var one = decimal.NewFromInt(1)

// ValidateTerms checks ranges, and the default/non-default field rules.
func ValidateTerms(t Terms, isDefault bool) error {
	if isDefault {
		if m := missing(t); len(m) > 0 {
			return ErrIncomplete.WithDetails(map[string]any{"fields": m})
		}
	} else if f := countryOnly(t); len(f) > 0 {
		return ErrCountryOnly.WithDetails(map[string]any{"fields": f})
	}
	var bad []map[string]string
	flag := func(path, code string) { bad = append(bad, map[string]string{"path": path, "code": code}) }
	rate := func(path string, d *decimal.Decimal) {
		if d != nil && (d.IsNegative() || !d.LessThan(one)) {
			flag(path, "OUT_OF_RANGE")
		}
	}
	nonNeg := func(path string, d *decimal.Decimal) {
		if d != nil && d.IsNegative() {
			flag(path, "NEGATIVE")
		}
	}
	rate("commission_rate", t.CommissionRate)
	rate("waiting_cap_rate", t.WaitingCapRate)
	rate("vat_rate", t.VATRate)
	rate("levy_rate", t.LevyRate)
	nonNeg("booking_fee", t.BookingFee)
	nonNeg("fare_rounding", t.FareRounding)
	if t.CancelDriverShare != nil && (t.CancelDriverShare.IsNegative() || t.CancelDriverShare.GreaterThan(one)) {
		flag("cancel_driver_share", "OUT_OF_RANGE")
	}
	if (t.CancelFreeS != nil && *t.CancelFreeS < 0) || (t.NoShowAfterS != nil && *t.NoShowAfterS < 0) {
		flag("cancellation", "NEGATIVE")
	}
	if t.VATBase != nil && *t.VATBase != "COMMISSION" && *t.VATBase != "GROSS_FARE" {
		flag("vat_base", "NOT_ALLOWED")
	}
	if t.LevyBorneBy != nil && *t.LevyBorneBy != "PLATFORM" && *t.LevyBorneBy != "DRIVER" && *t.LevyBorneBy != "RIDER" {
		flag("levy_borne_by", "NOT_ALLOWED")
	}
	if isDefault {
		bad = append(bad, debtLimitErrors(t)...)
	}
	if len(bad) > 0 {
		return apperr.Validation.WithDetails(map[string]any{"fields": bad})
	}
	return nil
}

// debtLimitErrors mirrors the database checks on a default's debt and payout rules.
func debtLimitErrors(t Terms) []map[string]string {
	var bad []map[string]string
	if !t.PayinMinAmount.IsPositive() {
		bad = append(bad, map[string]string{"path": "payin_min_amount", "code": "MUST_BE_POSITIVE"})
	}
	if t.DebtSoftLimit.LessThan(*t.PayinMinAmount) {
		bad = append(bad, map[string]string{"path": "debt_soft_limit", "code": "BELOW_MIN_PAYIN"})
	}
	if t.DebtHardLimit.LessThan(*t.DebtSoftLimit) {
		bad = append(bad, map[string]string{"path": "debt_hard_limit", "code": "BELOW_SOFT_LIMIT"})
	}
	if *t.DebtGraceDays < 0 {
		bad = append(bad, map[string]string{"path": "debt_soft_grace_days", "code": "NEGATIVE"})
	}
	if !t.PayoutMinAmount.IsPositive() {
		bad = append(bad, map[string]string{"path": "payout_min_amount", "code": "MUST_BE_POSITIVE"})
	}
	return bad
}

// ContentHash is a stable digest of a version's terms and charges (struct field
// order makes the JSON canonical).
func ContentHash(t Terms, charges []ChargeRule) string {
	raw, _ := json.Marshal(struct {
		Terms   Terms        `json:"terms"`
		Charges []ChargeRule `json:"charges"`
	}{t, charges})
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
