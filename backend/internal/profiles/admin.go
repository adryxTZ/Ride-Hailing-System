package profiles

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/kalen-limited/hauler-rider/backend/internal/audit"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
)

// CreateInput names a new profile.
type CreateInput struct {
	Country     string `json:"country"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Reason      string `json:"reason"`
}

// UpdateInput renames a profile or changes its activation window.
type UpdateInput struct {
	Name        *string    `json:"name"`
	Description *string    `json:"description"`
	ActiveFrom  *time.Time `json:"active_from"`
	ActiveUntil *time.Time `json:"active_until"`
	ClearWindow bool       `json:"clear_window"`
	Reason      string     `json:"reason"`
}

// VersionInput publishes a new version.
type VersionInput struct {
	Terms         Terms        `json:"terms"`
	Charges       []ChargeRule `json:"charges"`
	EffectiveFrom *time.Time   `json:"effective_from"`
	Reason        string       `json:"reason"`
}

// write runs one staff change under the country's pricing lock, with the
// profile's event, the audit entry and the config bump in the same transaction.
func (s *Service) write(ctx context.Context, country, reason string, fn func(tx pgx.Tx) error) error {
	if strings.TrimSpace(reason) == "" {
		return ErrReasonRequired
	}
	return pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1 || '/' || $2 || '/pricing'))`, s.store.brandID, country); err != nil {
			return err
		}
		if err := fn(tx); err != nil {
			return err
		}
		return bump(ctx, tx)
	})
}

// record audits a change to a profile and marks the profile as changed.
func (s *Service) record(ctx context.Context, tx pgx.Tx, staffID, action, profileID, reason string, data any) error {
	if _, err := tx.Exec(ctx, `UPDATE pricing_profiles SET updated_at=now() WHERE id=$1`, profileID); err != nil {
		return err
	}
	return audit.Record(ctx, tx, s.store.brandID, staffID, action, "pricing_profile:"+profileID, reason, data)
}

// Create adds a DRAFT profile (no version yet) to a country.
func (s *Service) Create(ctx context.Context, staffID string, in CreateInput) (*Profile, error) {
	in.Country, in.Name = strings.ToUpper(strings.TrimSpace(in.Country)), strings.TrimSpace(in.Name)
	if err := validateNaming(in.Name, in.Description); err != nil {
		return nil, err
	}
	if len(in.Country) != 2 {
		return nil, apperr.Validation.WithDetails(map[string]any{"fields": []map[string]string{{"path": "country", "code": "FORMAT"}}})
	}
	var id string
	err := s.write(ctx, in.Country, in.Reason, func(tx pgx.Tx) error {
		var n int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM pricing_profiles WHERE brand_id=$1 AND country_code=$2`, s.store.brandID, in.Country).Scan(&n); err != nil {
			return err
		}
		if n >= MaxProfilesPerCountry {
			return ErrLimitReached.WithDetails(map[string]any{"max": MaxProfilesPerCountry})
		}
		err := tx.QueryRow(ctx, `INSERT INTO pricing_profiles (brand_id, country_code, name, description, created_by)
			VALUES ($1,$2,$3,NULLIF($4,''),NULLIF($5,'')::uuid) RETURNING id`, s.store.brandID, in.Country, in.Name, in.Description, staffID).Scan(&id)
		if isUnique(err) {
			return ErrNameTaken
		}
		if err != nil {
			return err
		}
		if err := s.store.event(ctx, tx, id, EventCreated, staffID, in.Reason, nil, nil, map[string]any{"name": in.Name}); err != nil {
			return err
		}
		return s.record(ctx, tx, staffID, "pricing.profile.create", id, in.Reason, in)
	})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, id)
}

// Update renames a profile or changes its activation window (never the default's window).
func (s *Service) Update(ctx context.Context, staffID, id string, in UpdateInput) (*Profile, error) {
	p, err := s.store.profile(ctx, s.store.pool, id, false)
	if err != nil {
		return nil, err
	}
	err = s.write(ctx, p.Country, in.Reason, func(tx pgx.Tx) error {
		cur, err := s.store.profile(ctx, tx, id, true)
		if err != nil {
			return err
		}
		if err := s.applyUpdate(ctx, tx, staffID, cur, in); err != nil {
			return err
		}
		return s.record(ctx, tx, staffID, "pricing.profile.update", id, in.Reason, in)
	})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, id)
}

func (s *Service) applyUpdate(ctx context.Context, tx pgx.Tx, staffID string, cur Profile, in UpdateInput) error {
	name, desc := cur.Name, cur.Description
	if in.Name != nil {
		name = strings.TrimSpace(*in.Name)
	}
	if in.Description != nil {
		desc = *in.Description
	}
	if err := validateNaming(name, desc); err != nil {
		return err
	}
	from, until := cur.ActiveFrom, cur.ActiveUntil
	windowChanged := in.ClearWindow || in.ActiveFrom != nil || in.ActiveUntil != nil
	if in.ClearWindow {
		from, until = nil, nil
	}
	if in.ActiveFrom != nil {
		from = in.ActiveFrom
	}
	if in.ActiveUntil != nil {
		until = in.ActiveUntil
	}
	if windowChanged {
		if def, err := s.isDefault(ctx, tx, cur.ID); err != nil {
			return err
		} else if def && (from != nil || until != nil) {
			return ErrIsDefault
		}
		if until != nil && ((from != nil && !until.After(*from)) || until.Before(s.now())) {
			return ErrWindowInvalid
		}
	}
	_, err := tx.Exec(ctx, `UPDATE pricing_profiles SET name=$2, description=NULLIF($3,''), active_from=$4, active_until=$5, updated_at=now() WHERE id=$1`,
		cur.ID, name, desc, from, until)
	if isUnique(err) {
		return ErrNameTaken
	}
	if err != nil {
		return err
	}
	if name != cur.Name {
		if err := s.store.event(ctx, tx, cur.ID, EventRenamed, staffID, in.Reason, nil, nil, map[string]any{"from": cur.Name, "to": name}); err != nil {
			return err
		}
	}
	if windowChanged {
		return s.store.event(ctx, tx, cur.ID, EventWindowChanged, staffID, in.Reason, nil, nil,
			map[string]any{"old": map[string]any{"from": cur.ActiveFrom, "until": cur.ActiveUntil}, "new": map[string]any{"from": from, "until": until}})
	}
	return nil
}

// Publish adds the next immutable version (rules and charges), effective now or later.
func (s *Service) Publish(ctx context.Context, staffID, id string, in VersionInput) (*Version, error) {
	p, err := s.store.profile(ctx, s.store.pool, id, false)
	if err != nil {
		return nil, err
	}
	at, err := s.effectiveFrom(in.EffectiveFrom)
	if err != nil {
		return nil, err
	}
	if in.Charges, err = s.withDefaultRecipients(ctx, p.Country, in.Charges); err != nil { // a charge without a recipient takes its kind's default
		return nil, err
	}
	var vid string
	err = s.write(ctx, p.Country, in.Reason, func(tx pgx.Tx) error {
		def, err := s.isDefault(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := ValidateTerms(in.Terms, def); err != nil {
			return err
		}
		if err := s.validateCharges(ctx, tx, p.Country, in.Charges); err != nil {
			return err
		}
		if err := s.checkOrder(ctx, tx, id, at); err != nil {
			return err
		}
		version, err := s.insertVersion(ctx, tx, staffID, id, at, in)
		if err != nil {
			return err
		}
		vid = version.id
		if err := s.store.event(ctx, tx, id, EventPublished, staffID, in.Reason, &version.n, nil, map[string]any{"effective_from": at}); err != nil {
			return err
		}
		return s.record(ctx, tx, staffID, "pricing.profile.publish", id, in.Reason, map[string]any{"version": version.n, "effective_from": at})
	})
	if err != nil {
		return nil, err
	}
	vs, err := s.store.versions(ctx, s.store.pool, `v.id = $2`, vid)
	if err != nil || len(vs) == 0 {
		return nil, err
	}
	return &vs[0], nil
}

func (s *Service) effectiveFrom(in *time.Time) (time.Time, error) {
	now := s.now()
	if in == nil {
		return now, nil
	}
	if in.Before(now.Add(-EffectiveFromSkew)) || in.After(now.Add(EffectiveFromMaxAhead)) {
		return time.Time{}, ErrEffectiveFrom
	}
	return *in, nil
}

// checkOrder keeps versions in force in the order they were published: a new
// version cannot start before the latest one (that one would silently take over later).
func (s *Service) checkOrder(ctx context.Context, tx pgx.Tx, profileID string, at time.Time) error {
	var latest *time.Time
	if err := tx.QueryRow(ctx, `SELECT max(effective_from) FROM pricing_profile_versions WHERE profile_id=$1`, profileID).Scan(&latest); err != nil {
		return err
	}
	if latest != nil && at.Before(*latest) {
		return ErrEffectiveFrom.WithDetails(map[string]any{"after": latest})
	}
	return nil
}

type newVersion struct {
	id string
	n  int
}

func (s *Service) insertVersion(ctx context.Context, tx pgx.Tx, staffID, profileID string, at time.Time, in VersionInput) (newVersion, error) {
	t := in.Terms
	var v newVersion
	err := tx.QueryRow(ctx, `INSERT INTO pricing_profile_versions (profile_id, version, effective_from, commission_rate, booking_fee, waiting_cap_rate,
		  fare_rounding, cancel_free_s, no_show_after_s, cancel_driver_share, vat_rate, vat_base, levy_rate, levy_borne_by, debt_soft_limit,
		  debt_hard_limit, debt_soft_grace_days, payin_min_amount, payout_min_amount, payout_auto_approve, content_hash, published_by, reason)
		SELECT $1, coalesce(max(version),0)+1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, NULLIF($21,'')::uuid, $22
		FROM pricing_profile_versions WHERE profile_id=$1 RETURNING id, version`,
		profileID, at, t.CommissionRate, t.BookingFee, t.WaitingCapRate, t.FareRounding, t.CancelFreeS, t.NoShowAfterS, t.CancelDriverShare,
		t.VATRate, t.VATBase, t.LevyRate, t.LevyBorneBy, t.DebtSoftLimit, t.DebtHardLimit, t.DebtGraceDays, t.PayinMinAmount, t.PayoutMinAmount,
		t.PayoutAutoApprove, ContentHash(t, in.Charges), staffID, in.Reason).Scan(&v.id, &v.n)
	if err != nil {
		return v, err
	}
	for i, c := range in.Charges {
		if err := insertCharge(ctx, tx, v.id, i, c); err != nil {
			return v, err
		}
	}
	return v, nil
}

func insertCharge(ctx context.Context, tx pgx.Tx, versionID string, order int, c ChargeRule) error {
	var from, to *int
	if c.FromTime != "" {
		f, e := minuteOf(c.FromTime), minuteOf(c.ToTime)
		from, to = &f, &e
	}
	days := make([]int16, len(c.Days))
	for i, d := range c.Days {
		days[i] = int16(d)
	}
	_, err := tx.Exec(ctx, `INSERT INTO pricing_profile_charges (version_id, code, label, category, amount_type, amount, min_amount, max_amount,
		  recipient, payee_id, taxable, applies_when, services, vehicle_classes, on_fixed_routes, days, from_minute, to_minute, sort_order)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,NULLIF($10,'')::uuid,$11,$12,$13,$14,$15,$16,$17,$18,$19)`,
		versionID, c.Code, c.Label, c.Category, c.AmountType, c.Amount, c.MinAmount, c.MaxAmount, c.Recipient, c.PayeeID, c.Taxable,
		c.AppliesWhen, orEmpty(c.Services), orEmpty(c.VehicleClasses), c.OnFixedRoutes, days, from, to, order)
	return err
}

// validateCharges checks each charge, the payees and catalogue keys, duplicates and the count.
func (s *Service) validateCharges(ctx context.Context, tx pgx.Tx, country string, charges []ChargeRule) error {
	if len(charges) > MaxChargesPerVersion {
		return ErrTooManyCharges.WithDetails(map[string]any{"max": MaxChargesPerVersion})
	}
	codes := map[string]bool{}
	for i, c := range charges {
		if bad := chargeErrors(c); len(bad) > 0 {
			return ErrChargeInvalid.WithDetails(map[string]any{"index": i, "fields": bad})
		}
		if codes[c.Code] {
			return ErrChargeDuplicate.WithDetails(map[string]any{"code": c.Code})
		}
		codes[c.Code] = true
		if err := s.checkPayee(ctx, tx, country, c.PayeeID, i); err != nil {
			return err
		}
		if err := s.checkKeys(ctx, c, i); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) checkPayee(ctx context.Context, tx pgx.Tx, country, payeeID string, index int) error {
	if payeeID == "" {
		return nil
	}
	var ok bool
	err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM charge_payees WHERE id::text=$1 AND brand_id=$2 AND country_code=$3 AND status='ACTIVE')`,
		payeeID, s.store.brandID, country).Scan(&ok)
	if err == nil && !ok {
		return ErrPayeeInvalid.WithDetails(map[string]any{"index": index})
	}
	return err
}

// checkKeys refuses vehicle classes the catalogue does not know (in a listed
// service, or in any service when none is listed).
func (s *Service) checkKeys(ctx context.Context, c ChargeRule, index int) error {
	if s.catalog == nil {
		return nil
	}
	unknown := ErrChargeInvalid.WithDetails(map[string]any{"index": index, "fields": []map[string]string{{"path": "vehicle_classes", "code": "UNKNOWN"}}})
	for _, cls := range c.VehicleClasses {
		svcs, err := s.catalog.ServicesForClass(ctx, cls)
		if err != nil {
			return err
		}
		if len(svcs) == 0 || (len(c.Services) > 0 && !overlaps(svcs, c.Services)) {
			return unknown
		}
	}
	return nil
}

func overlaps(a, b []string) bool {
	for _, x := range a {
		if contains(b, x) {
			return true
		}
	}
	return false
}

// isDefault reports whether the profile holds its country's open COUNTRY assignment.
func (s *Service) isDefault(ctx context.Context, q querier, id string) (bool, error) {
	var ok bool
	err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pricing_profile_assignments WHERE profile_id=$1 AND scope_kind='COUNTRY' AND valid_to IS NULL)`, id).Scan(&ok)
	return ok, err
}

func validateNaming(name, desc string) error {
	if name == "" || len([]rune(name)) > NameMaxLen {
		return apperr.Validation.WithDetails(map[string]any{"fields": []map[string]string{{"path": "name", "code": "LENGTH"}}})
	}
	if len([]rune(desc)) > DescriptionMaxLen {
		return apperr.Validation.WithDetails(map[string]any{"fields": []map[string]string{{"path": "description", "code": "LENGTH"}}})
	}
	return nil
}

func isUnique(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func orEmpty(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
