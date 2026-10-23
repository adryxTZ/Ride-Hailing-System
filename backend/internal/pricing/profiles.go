package pricing

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/pricing/engine"
	"github.com/kalen-limited/hauler-rider/backend/internal/profiles"
)

// Profiles is the pricing-profile module as quotes and the Pricing page use it.
type Profiles interface {
	Resolve(ctx context.Context, in profiles.Input) (*profiles.Resolution, error)
	DefaultTerms(ctx context.Context, country string) (profiles.Effective, profiles.Ref, error)
	ForScope(ctx context.Context, country, areaID string) (*profiles.ScopeVersions, error)
	PublishForScope(ctx context.Context, staffID, country, areaID string, t profiles.Terms, reason string) error
	SeedDefault(ctx context.Context, tx pgx.Tx, country string, t profiles.Terms) error
}

// LineCharge is the quote and receipt line of an extra charge (toll, parking, levy …).
const LineCharge = "CHARGE"

// SetProfiles connects pricing profiles and the country timezones charges are timed in.
func (s *Service) SetProfiles(p Profiles, tz func(country string) *time.Location) {
	s.profiles, s.tz = p, tz
}

// feePolicyOf presents merged profile rules in the shape quotes and old callers use.
func feePolicyOf(country, areaID string, e profiles.Effective, version int) FeePolicy {
	return FeePolicy{Country: country, AreaID: areaID, Version: version, CommissionRate: e.CommissionRate, VATRate: e.VATRate, VATBase: e.VATBase,
		LevyRate: e.LevyRate, LevyBorneBy: e.LevyBorneBy, BookingFee: e.BookingFee, WaitingCapRate: e.WaitingCapRate, FareRounding: e.FareRounding,
		CancelFreeS: e.CancelFreeS, NoShowAfterS: e.NoShowAfterS, CancelDriverShare: e.CancelDriverShare, DebtSoftLimit: e.DebtSoftLimit,
		DebtHardLimit: e.DebtHardLimit, DebtGraceDays: e.DebtGraceDays, PayinMinAmount: e.PayinMinAmount, PayoutMinAmount: e.PayoutMinAmount,
		PayoutAutoApprove: e.PayoutAutoApprove}
}

// localTime is the quote time in the country's timezone (charges' day and time windows).
// Locations are cached by the wiring (time.LoadLocation reads the zone database).
func (s *Service) localTime(country string) time.Time {
	now := s.now()
	if s.tz != nil {
		if loc := s.tz(country); loc != nil {
			return now.In(loc)
		}
	}
	return now
}

// withCharges adds the profile's matching charges to an option as labelled
// CHARGE lines (before rounding) and returns what they add.
func withCharges(o Option, res *profiles.Resolution, service string, distanceM int, at time.Time) (Option, []profiles.AppliedCharge, decimal.Decimal) {
	applied := []profiles.AppliedCharge{}
	if !o.Available || res == nil || len(res.Charges) == 0 {
		return o, applied, decimal.Zero
	}
	in := profiles.ChargeInput{Service: service, Class: o.VehicleClass, FixedRoute: o.Source == "FIXED_ROUTE", DistanceM: distanceM,
		FareBase: o.Fare.Sub(res.Terms.BookingFee), At: at}
	applied = profiles.ApplyCharges(res.Charges, in)
	added := decimal.Zero
	for _, c := range applied {
		o.Lines = append(o.Lines, engine.Line{Code: LineCharge, Amount: c.Amount, Label: c.Label,
			Meta: map[string]any{"charge": c.Code, "category": c.Category, "recipient": c.Recipient}})
		added = added.Add(c.Amount)
	}
	o.Fare = o.Fare.Add(added).Round(2)
	return o, applied, added
}

// carvedOf is what charges take out of the split (all but those that are part of the fare).
func carvedOf(applied []profiles.AppliedCharge) decimal.Decimal {
	sum := decimal.Zero
	for _, c := range applied {
		if c.Recipient != "FARE" {
			sum = sum.Add(c.Amount)
		}
	}
	return sum
}

// quotePricing is what quotes.pricing stores: the resolution and each class's pinned terms and charges.
func quotePricing(res *profiles.Resolution, perClass map[string]profiles.QuoteClassPinned) profiles.QuotePricing {
	return profiles.QuotePricing{Profile: res.Profile, Scope: res.Scope, Default: res.Default, Trace: res.Trace, Config: res.Config, PerClass: perClass}
}

// seedPolicyFile mirrors seed/pricing/fee_policy.json.
type seedPolicyFile struct {
	CommissionRate    decimal.Decimal `json:"commission_rate"`
	VATRate           decimal.Decimal `json:"vat_rate"`
	VATBase           string          `json:"vat_base"`
	LevyRate          decimal.Decimal `json:"levy_rate"`
	LevyBorneBy       string          `json:"levy_borne_by"`
	BookingFee        decimal.Decimal `json:"booking_fee"`
	WaitingCapRate    decimal.Decimal `json:"waiting_cap_rate"`
	FareRounding      decimal.Decimal `json:"fare_rounding"`
	CancelFreeS       int             `json:"cancel_free_s"`
	NoShowAfterS      int             `json:"no_show_after_s"`
	CancelShare       decimal.Decimal `json:"cancel_driver_share"`
	DebtSoft          decimal.Decimal `json:"debt_soft_limit"`
	DebtHard          decimal.Decimal `json:"debt_hard_limit"`
	DebtGraceDays     int             `json:"debt_soft_grace_days"`
	PayinMin          decimal.Decimal `json:"payin_min_amount"`
	PayoutMin         decimal.Decimal `json:"payout_min_amount"`
	PayoutAutoApprove bool            `json:"payout_auto_approve"`
}

// defaultPayoutMin is used when an older seed file has no payout minimum.
var defaultPayoutMin = decimal.NewFromInt(5000)

// seedPolicy publishes the brand's fee_policy.json as its home country's default profile when it changed.
func (s *Service) seedPolicy(ctx context.Context, tx pgx.Tx, raw []byte) error {
	if s.profiles == nil {
		return nil
	}
	var p seedPolicyFile
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("fee_policy.json: %w", err)
	}
	if !p.PayoutMin.IsPositive() {
		p.PayoutMin = defaultPayoutMin
	}
	t := profiles.Terms{CommissionRate: &p.CommissionRate, BookingFee: &p.BookingFee, WaitingCapRate: &p.WaitingCapRate, FareRounding: &p.FareRounding,
		CancelFreeS: &p.CancelFreeS, NoShowAfterS: &p.NoShowAfterS, CancelDriverShare: &p.CancelShare, VATRate: &p.VATRate, VATBase: &p.VATBase,
		LevyRate: &p.LevyRate, LevyBorneBy: &p.LevyBorneBy, DebtSoftLimit: &p.DebtSoft, DebtHardLimit: &p.DebtHard, DebtGraceDays: &p.DebtGraceDays,
		PayinMinAmount: &p.PayinMin, PayoutMinAmount: &p.PayoutMin, PayoutAutoApprove: &p.PayoutAutoApprove}
	return s.profiles.SeedDefault(ctx, tx, s.store.homeCountry, t)
}
