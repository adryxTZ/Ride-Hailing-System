package pricing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/audit"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
	"github.com/kalen-limited/hauler-rider/backend/internal/pricing/engine"
	"github.com/kalen-limited/hauler-rider/backend/internal/profiles"
)

// TariffView is one ride type's price list as staff edit it.
type TariffView struct {
	Service           string                     `json:"service"`
	VehicleClass      string                     `json:"vehicle_class"`
	Country           string                     `json:"country"`
	AreaID            string                     `json:"area_id,omitempty"`
	Inherited         bool                       `json:"inherited"` // an area using the country price
	Currency          string                     `json:"currency"`
	Engine            string                     `json:"engine"`
	Params            map[string]decimal.Decimal `json:"params"`
	MinimumFare       decimal.Decimal            `json:"minimum_fare"`
	WaitingRatePerMin decimal.Decimal            `json:"waiting_rate_per_min"`
	FreeWaitingS      int                        `json:"free_waiting_s"`
	CancellationFee   decimal.Decimal            `json:"cancellation_fee"`
	SurgeEnabled      bool                       `json:"surge_enabled"`
}

// PolicyView is a fee-policy version as staff see it, for one scope.
type PolicyView struct {
	Country           string          `json:"country"`
	AreaID            string          `json:"area_id,omitempty"`
	Inherited         bool            `json:"inherited"` // an area showing its country's policy
	Version           int             `json:"version"`
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
	CancelDriverShare decimal.Decimal `json:"cancel_driver_share"`
	DebtSoftLimit     decimal.Decimal `json:"debt_soft_limit"`
	DebtHardLimit     decimal.Decimal `json:"debt_hard_limit"`
	DebtGraceDays     int             `json:"debt_soft_grace_days"`
	PayinMinAmount    decimal.Decimal `json:"payin_min_amount"`
	PayoutMinAmount   decimal.Decimal `json:"payout_min_amount"`
	PayoutAutoApprove bool            `json:"payout_auto_approve"`
	EffectiveFrom     time.Time       `json:"effective_from"`
}

// PricingAdmin is the pricing setup of one scope (a country, or an area in it) in one read.
type PricingAdmin struct {
	Country     string               `json:"country"`
	AreaID      string               `json:"area_id,omitempty"`
	Tariffs     []TariffView         `json:"tariffs"`
	Policy      *PolicyView          `json:"policy"`
	Policies    []PolicyView         `json:"policy_history"`
	FixedRoutes []FixedRoute         `json:"fixed_routes"`
	Engines     []engine.Description `json:"engines"`
}

// AdminPricing returns the tariffs in force for a scope (an area's own, else the
// country's, flagged inherited), its policy with history, the fixed fares of
// the country (and the area's), and the engine catalogue.
func (s *Service) AdminPricing(ctx context.Context, country, areaID string) (*PricingAdmin, error) {
	tf, _, err := s.cache.Get(ctx)
	if err != nil {
		return nil, err
	}
	if country == "" {
		country = s.store.homeCountry
	}
	out := &PricingAdmin{Country: country, AreaID: areaID, Tariffs: []TariffView{}, Engines: engine.Describe(), FixedRoutes: []FixedRoute{}}
	for _, r := range tf.Routes {
		if r.Country == country && (r.AreaID == "" || r.AreaID == areaID) {
			out.FixedRoutes = append(out.FixedRoutes, r)
		}
	}
	countryPrefix := scopeKey(country, "") + "/"
	for key, t := range tf.ByClass {
		if !strings.HasPrefix(key, countryPrefix) {
			continue
		}
		rest := strings.TrimPrefix(key, countryPrefix)
		svc, cls, _ := strings.Cut(rest, "/")
		view := tariffView(svc, cls, t, country, "", areaID != "")
		if areaID != "" {
			if own, ok := tf.ByClass[scopeKey(country, areaID)+"/"+rest]; ok {
				view = tariffView(svc, cls, own, country, areaID, false)
			}
		}
		out.Tariffs = append(out.Tariffs, view)
	}
	if out.Policies, out.Policy, err = s.policyViews(ctx, country, areaID); err != nil {
		return nil, err
	}
	return out, nil
}

func tariffView(svc, cls string, t engine.Tariff, country, areaID string, inherited bool) TariffView {
	return TariffView{Service: svc, VehicleClass: cls, Country: country, AreaID: areaID, Inherited: inherited, Currency: t.Currency, Engine: t.Engine, Params: t.Params,
		MinimumFare: t.MinimumFare, WaitingRatePerMin: t.WaitingRatePerMin, FreeWaitingS: t.LoadingGraceS,
		CancellationFee: t.CancellationFee, SurgeEnabled: t.SurgeEnabled}
}

// TariffInput is what staff change on a ride type.
type TariffInput struct {
	Engine            string                     `json:"engine"`
	Params            map[string]decimal.Decimal `json:"params"`
	MinimumFare       decimal.Decimal            `json:"minimum_fare"`
	WaitingRatePerMin decimal.Decimal            `json:"waiting_rate_per_min"`
	FreeWaitingS      int                        `json:"free_waiting_s"`
	CancellationFee   decimal.Decimal            `json:"cancellation_fee"`
	SurgeEnabled      bool                       `json:"surge_enabled"`
	Reason            string                     `json:"reason"`
}

// UpdateTariff sets one ride type's prices for a scope: the country default, or
// an area's own prices (created on first save, so the area stops inheriting).
// Audited; quotes use the new prices within seconds.
func (s *Service) UpdateTariff(ctx context.Context, staffID, country, areaID, service, class string, in TariffInput) error {
	if err := validateTariff(in); err != nil {
		return err
	}
	if country == "" {
		country = s.store.homeCountry
	}
	params, _ := json.Marshal(in.Params)
	target := "tariff:" + country + "/" + areaID + "/" + service + "/" + class
	return s.adminWrite(ctx, staffID, "pricing.tariff", target, in.Reason, in, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			INSERT INTO tariffs (vehicle_class_id, country_code, area_id, currency, engine, params, minimum_fare, waiting_rate_per_min, loading_grace_s, cancellation_fee, surge_enabled)
			SELECT vc.id, $4, NULLIF($5,'')::uuid,
			  coalesce((SELECT currency FROM countries c WHERE c.brand_id=sv.brand_id AND c.code=$4), (SELECT currency FROM brands b WHERE b.id=sv.brand_id)),
			  $6, $7, $8, $9, $10, $11, $12
			FROM vehicle_classes vc JOIN services sv ON sv.id=vc.service_id WHERE sv.brand_id=$1 AND sv.key=$2 AND vc.key=$3
			ON CONFLICT ON CONSTRAINT tariffs_scope_uq DO UPDATE SET engine=EXCLUDED.engine, params=EXCLUDED.params, minimum_fare=EXCLUDED.minimum_fare,
			  waiting_rate_per_min=EXCLUDED.waiting_rate_per_min, loading_grace_s=EXCLUDED.loading_grace_s, cancellation_fee=EXCLUDED.cancellation_fee,
			  surge_enabled=EXCLUDED.surge_enabled, status='ACTIVE', updated_at=now()`,
			s.store.brandID, service, class, country, areaID, in.Engine, params, in.MinimumFare, in.WaitingRatePerMin, in.FreeWaitingS, in.CancellationFee, in.SurgeEnabled)
		if err == nil && tag.RowsAffected() == 0 {
			return apperr.NotFound
		}
		return err
	})
}

// DeleteTariffOverride removes an area's own prices so it inherits the country's again.
func (s *Service) DeleteTariffOverride(ctx context.Context, staffID, country, areaID, service, class, reason string) error {
	if areaID == "" {
		return ErrCountryDefaultRequired
	}
	return s.adminWrite(ctx, staffID, "pricing.tariff.delete", "tariff:"+country+"/"+areaID+"/"+service+"/"+class, reason, nil, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `DELETE FROM tariffs t USING vehicle_classes vc, services sv
			WHERE t.vehicle_class_id=vc.id AND vc.service_id=sv.id AND sv.brand_id=$1 AND sv.key=$2 AND vc.key=$3 AND t.country_code=$4 AND t.area_id=$5::uuid`,
			s.store.brandID, service, class, country, areaID)
		if err == nil && tag.RowsAffected() == 0 {
			return apperr.NotFound
		}
		return err
	})
}

// NewPolicy publishes rules for a scope through its pricing profile (the
// Pricing page; kept for the old policy endpoint): the country default's next
// version, or a region's own profile (created on first save). A region cannot
// change the country-only tax, debt and payout rules.
func (s *Service) NewPolicy(ctx context.Context, staffID string, p PolicyView, reason string) error {
	if err := validatePolicy(p); err != nil {
		return err
	}
	if p.Country == "" {
		p.Country = s.store.homeCountry
	}
	if s.profiles == nil {
		return ErrNoTariff
	}
	t := termsOf(p)
	if p.AreaID != "" {
		def, _, err := s.profiles.DefaultTerms(ctx, p.Country)
		if err != nil {
			return err
		}
		if fields := countryOnlyDiff(p, def); len(fields) > 0 {
			return ErrPolicyFieldCountryOnly.WithDetails(map[string]any{"fields": fields})
		}
		t = tripRulesOf(t)
	}
	return s.profiles.PublishForScope(ctx, staffID, p.Country, p.AreaID, t, reason)
}

// termsOf is a full policy view as profile terms.
func termsOf(p PolicyView) profiles.Terms {
	return profiles.Terms{CommissionRate: &p.CommissionRate, BookingFee: &p.BookingFee, WaitingCapRate: &p.WaitingCapRate, FareRounding: &p.FareRounding,
		CancelFreeS: &p.CancelFreeS, NoShowAfterS: &p.NoShowAfterS, CancelDriverShare: &p.CancelDriverShare, VATRate: &p.VATRate, VATBase: &p.VATBase,
		LevyRate: &p.LevyRate, LevyBorneBy: &p.LevyBorneBy, DebtSoftLimit: &p.DebtSoftLimit, DebtHardLimit: &p.DebtHardLimit, DebtGraceDays: &p.DebtGraceDays,
		PayinMinAmount: &p.PayinMinAmount, PayoutMinAmount: &p.PayoutMinAmount, PayoutAutoApprove: &p.PayoutAutoApprove}
}

// tripRulesOf keeps only the trip rules (a region's profile inherits the country-only ones).
func tripRulesOf(t profiles.Terms) profiles.Terms {
	return profiles.Terms{CommissionRate: t.CommissionRate, BookingFee: t.BookingFee, WaitingCapRate: t.WaitingCapRate, FareRounding: t.FareRounding,
		CancelFreeS: t.CancelFreeS, NoShowAfterS: t.NoShowAfterS, CancelDriverShare: t.CancelDriverShare}
}

// countryOnlyDiff lists the tax, debt and payout fields a region tried to change from the default.
func countryOnlyDiff(p PolicyView, d profiles.Effective) []string {
	var out []string
	if !p.VATRate.Equal(d.VATRate) || p.VATBase != d.VATBase {
		out = append(out, "vat_rate", "vat_base")
	}
	if !p.LevyRate.Equal(d.LevyRate) || p.LevyBorneBy != d.LevyBorneBy {
		out = append(out, "levy_rate", "levy_borne_by")
	}
	if !p.DebtSoftLimit.Equal(d.DebtSoftLimit) || !p.DebtHardLimit.Equal(d.DebtHardLimit) || p.DebtGraceDays != d.DebtGraceDays || !p.PayinMinAmount.Equal(d.PayinMinAmount) {
		out = append(out, "debt_soft_limit", "debt_hard_limit", "debt_soft_grace_days", "payin_min_amount")
	}
	if !p.PayoutMinAmount.Equal(d.PayoutMinAmount) || p.PayoutAutoApprove != d.PayoutAutoApprove {
		out = append(out, "payout_min_amount", "payout_auto_approve")
	}
	return out
}

// policyViews shows a scope's profile versions as policy views (merged over the
// default) and the one in force; a region without its own profile shows the default's, flagged inherited.
func (s *Service) policyViews(ctx context.Context, country, areaID string) ([]PolicyView, *PolicyView, error) {
	out := []PolicyView{}
	if s.profiles == nil {
		return out, nil, nil
	}
	sv, err := s.profiles.ForScope(ctx, country, areaID)
	if err != nil {
		return nil, nil, err
	}
	def, _, err := s.profiles.DefaultTerms(ctx, country)
	if err != nil {
		return out, nil, nil
	}
	var current *PolicyView
	now := s.now()
	for _, v := range sv.Versions {
		e := profiles.Merge(termsFromEffective(def), v.Terms)
		view := policyView(country, areaID, v.Version, v.EffectiveFrom, e, sv.Inherited)
		out = append(out, view)
		if current == nil && !v.EffectiveFrom.After(now) {
			c := view
			current = &c
		}
	}
	return out, current, nil
}

func policyView(country, areaID string, version int, from time.Time, e profiles.Effective, inherited bool) PolicyView {
	return PolicyView{Country: country, AreaID: areaID, Inherited: inherited, Version: version, CommissionRate: e.CommissionRate, VATRate: e.VATRate,
		VATBase: e.VATBase, LevyRate: e.LevyRate, LevyBorneBy: e.LevyBorneBy, BookingFee: e.BookingFee, WaitingCapRate: e.WaitingCapRate,
		FareRounding: e.FareRounding, CancelFreeS: e.CancelFreeS, NoShowAfterS: e.NoShowAfterS, CancelDriverShare: e.CancelDriverShare,
		DebtSoftLimit: e.DebtSoftLimit, DebtHardLimit: e.DebtHardLimit, DebtGraceDays: e.DebtGraceDays, PayinMinAmount: e.PayinMinAmount,
		PayoutMinAmount: e.PayoutMinAmount, PayoutAutoApprove: e.PayoutAutoApprove, EffectiveFrom: from}
}

// termsFromEffective turns complete rules back into terms (for merging).
func termsFromEffective(e profiles.Effective) profiles.Terms {
	return termsOf(policyView("", "", 0, time.Time{}, e, false))
}

// SaveFixedRoute creates (empty ID) or updates a fixed fare, audited.
func (s *Service) SaveFixedRoute(ctx context.Context, staffID string, r FixedRoute, reason string) error {
	if strings.TrimSpace(r.Name) == "" || !r.Price.IsPositive() || r.OriginRadiusM <= 0 || r.DestinationRadiusM <= 0 {
		return apperr.Validation
	}
	if r.VehicleClasses == nil {
		r.VehicleClasses = []string{}
	}
	pt := func(lat, lng float64) string { return fmt.Sprintf("SRID=4326;POINT(%f %f)", lng, lat) }
	return s.adminWrite(ctx, staffID, "pricing.fixed_route", "fixed_route:"+r.Name, reason, r, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO fixed_routes (id, brand_id, service_key, name, origin, origin_radius_m, destination, destination_radius_m, bidirectional,
			  vehicle_classes, price, reverse_price, currency, includes_loaders, surge_applies, priority)
			VALUES (COALESCE(NULLIF($1,'')::uuid, gen_random_uuid()), $2, $3, $4, ST_GeogFromText($5), $6, ST_GeogFromText($7), $8, $9, $10, $11, $12, $13, false, $14, $15)
			ON CONFLICT (id) DO UPDATE SET service_key=EXCLUDED.service_key, name=EXCLUDED.name, origin=EXCLUDED.origin,
			  origin_radius_m=EXCLUDED.origin_radius_m, destination=EXCLUDED.destination, destination_radius_m=EXCLUDED.destination_radius_m,
			  bidirectional=EXCLUDED.bidirectional, vehicle_classes=EXCLUDED.vehicle_classes, price=EXCLUDED.price,
			  reverse_price=EXCLUDED.reverse_price, currency=EXCLUDED.currency, surge_applies=EXCLUDED.surge_applies,
			  priority=EXCLUDED.priority, updated_at=now()`,
			r.ID, s.store.brandID, r.ServiceKey, r.Name, pt(r.Origin.Lat, r.Origin.Lng), r.OriginRadiusM, pt(r.Destination.Lat, r.Destination.Lng),
			r.DestinationRadiusM, r.Bidirectional, r.VehicleClasses, r.Price, r.ReversePrice, r.Currency, r.SurgeApplies, r.Priority)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrRouteNameTaken
		}
		return err
	})
}

// ErrRouteNameTaken: fixed fares are named uniquely per service.
var ErrRouteNameTaken = apperr.New("ROUTE_NAME_TAKEN", http.StatusConflict, "")

// DeleteFixedRoute retires a fixed fare (past trips keep their price), audited.
func (s *Service) DeleteFixedRoute(ctx context.Context, staffID, id, reason string) error {
	return s.adminWrite(ctx, staffID, "pricing.fixed_route.delete", "fixed_route:"+id, reason, nil, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE fixed_routes SET status='DISABLED', updated_at=now() WHERE id=$1 AND brand_id=$2`, id, s.store.brandID)
		if err == nil && tag.RowsAffected() == 0 {
			return apperr.NotFound
		}
		return err
	})
}

// adminWrite runs a pricing change with its audit entry and bumps the pricing
// version so every process reloads prices within seconds.
func (s *Service) adminWrite(ctx context.Context, staffID, action, target, reason string, data any, fn func(pgx.Tx) error) error {
	if strings.TrimSpace(reason) == "" {
		return apperr.Validation.WithDetails(map[string]any{"fields": []map[string]string{{"path": "reason", "code": "REQUIRED"}}})
	}
	return pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		if err := fn(tx); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE config_versions SET version = version + 1, updated_at = now() WHERE scope='pricing'`); err != nil {
			return err
		}
		return audit.Record(ctx, tx, s.store.brandID, staffID, action, target, reason, data)
	})
}

var one = decimal.NewFromInt(1)

func validateTariff(in TariffInput) error {
	if engine.Get(in.Engine) == nil {
		return apperr.Validation.WithDetails(map[string]any{"fields": []map[string]string{{"path": "engine", "code": "UNKNOWN"}}})
	}
	for k, v := range in.Params {
		if v.IsNegative() {
			return apperr.Validation.WithDetails(map[string]any{"fields": []map[string]string{{"path": "params." + k, "code": "NEGATIVE"}}})
		}
	}
	if in.MinimumFare.IsNegative() || in.WaitingRatePerMin.IsNegative() || in.CancellationFee.IsNegative() || in.FreeWaitingS < 0 {
		return apperr.Validation
	}
	return nil
}

func validatePolicy(p PolicyView) error {
	rate := func(d decimal.Decimal) bool { return !d.IsNegative() && d.LessThan(one) }
	if !rate(p.CommissionRate) || !rate(p.VATRate) || !rate(p.LevyRate) || !rate(p.WaitingCapRate) || p.CancelDriverShare.IsNegative() || p.CancelDriverShare.GreaterThan(one) {
		return apperr.Validation.WithDetails(map[string]any{"reason": "rates must be between 0 and 1"})
	}
	if (p.VATBase != "COMMISSION" && p.VATBase != "GROSS_FARE") || (p.LevyBorneBy != "PLATFORM" && p.LevyBorneBy != "DRIVER" && p.LevyBorneBy != "RIDER") {
		return apperr.Validation
	}
	if p.BookingFee.IsNegative() || p.FareRounding.IsNegative() || p.CancelFreeS < 0 || p.NoShowAfterS < 0 {
		return apperr.Validation
	}
	return validateDebtLimits(p)
}

// validateDebtLimits mirrors the fee_policies_debt_limits_chk constraint so staff get field errors, not a 500.
func validateDebtLimits(p PolicyView) error {
	var fields []map[string]string
	if !p.PayinMinAmount.IsPositive() {
		fields = append(fields, map[string]string{"path": "payin_min_amount", "code": "MUST_BE_POSITIVE"})
	}
	if p.DebtSoftLimit.LessThan(p.PayinMinAmount) {
		fields = append(fields, map[string]string{"path": "debt_soft_limit", "code": "BELOW_MIN_PAYIN"})
	}
	if p.DebtHardLimit.LessThan(p.DebtSoftLimit) {
		fields = append(fields, map[string]string{"path": "debt_hard_limit", "code": "BELOW_SOFT_LIMIT"})
	}
	if p.DebtGraceDays < 0 {
		fields = append(fields, map[string]string{"path": "debt_soft_grace_days", "code": "NEGATIVE"})
	}
	if !p.PayoutMinAmount.IsPositive() {
		fields = append(fields, map[string]string{"path": "payout_min_amount", "code": "MUST_BE_POSITIVE"})
	}
	if len(fields) > 0 {
		return apperr.Validation.WithDetails(map[string]any{"fields": fields})
	}
	return nil
}
