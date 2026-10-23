package profiles

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// ScopeVersions is the profile that owns a scope (a country's default, or a
// region's own profile) and its versions, newest first; Inherited is true when
// a region has no profile of its own and the country default is shown.
type ScopeVersions struct {
	Profile   *Profile
	Versions  []Version
	Inherited bool
}

// ForScope returns the profile assigned to a country (areaID "") or a region, and its versions.
func (s *Service) ForScope(ctx context.Context, country, areaID string) (*ScopeVersions, error) {
	if err := s.areaIn(ctx, country, areaID); err != nil {
		return nil, err
	}
	kind, id := ScopeCountry, country
	if areaID != "" {
		kind, id = ScopeArea, areaID
	}
	open, err := s.store.assignments(ctx, s.store.pool, `a.valid_to IS NULL AND a.scope_kind=$2 AND a.scope_id=$3 AND a.country_code=$4`, kind, id, country)
	if err != nil {
		return nil, err
	}
	if len(open) == 0 {
		if areaID == "" {
			return &ScopeVersions{Versions: []Version{}}, nil
		}
		out, err := s.ForScope(ctx, country, "")
		if out != nil {
			out.Inherited = true
		}
		return out, err
	}
	p, err := s.Get(ctx, open[0].ProfileID)
	if err != nil {
		return nil, err
	}
	return &ScopeVersions{Profile: p, Versions: p.Versions}, nil
}

// PublishForScope publishes rules for a country's default or a region's own
// profile (the Pricing page and the old policy endpoint). A region without a
// profile gets one: created, published, assigned and switched on in one go.
func (s *Service) PublishForScope(ctx context.Context, staffID, country, areaID string, t Terms, reason string) error {
	if err := s.areaIn(ctx, country, areaID); err != nil {
		return err
	}
	sv, err := s.ForScope(ctx, country, areaID)
	if err != nil {
		return err
	}
	if sv.Profile != nil && !sv.Inherited {
		_, err := s.Publish(ctx, staffID, sv.Profile.ID, VersionInput{Terms: t, Charges: s.keepCharges(sv), Reason: reason})
		return err
	}
	if areaID == "" {
		return ErrNoDefault
	}
	return s.createRegionProfile(ctx, staffID, country, areaID, t, reason)
}

// areaIn refuses a region that is not in the given country (staff limited to
// one country must never reach another country's rates through an area id).
func (s *Service) areaIn(ctx context.Context, country, areaID string) error {
	if areaID == "" {
		return nil
	}
	at, _, ok, err := s.areas.AreaInfo(ctx, areaID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrScopeNotFound
	}
	if at != country {
		return ErrScopeCountryMismatch
	}
	return nil
}

// keepCharges carries the current version's charges into a rules-only publish.
func (s *Service) keepCharges(sv *ScopeVersions) []ChargeRule {
	now := s.now()
	for _, v := range sv.Versions {
		if !v.EffectiveFrom.After(now) {
			return v.Charges
		}
	}
	return nil
}

// createRegionProfile makes a region's own profile on first save.
func (s *Service) createRegionProfile(ctx context.Context, staffID, country, areaID string, t Terms, reason string) error {
	_, name, ok, err := s.areas.AreaInfo(ctx, areaID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrScopeNotFound
	}
	p, err := s.Create(ctx, staffID, CreateInput{Country: country, Name: name, Reason: reason})
	if err == ErrNameTaken {
		p, err = s.Create(ctx, staffID, CreateInput{Country: country, Name: name + " (" + areaID[:8] + ")", Reason: reason})
	}
	if err != nil {
		return err
	}
	if _, err := s.Publish(ctx, staffID, p.ID, VersionInput{Terms: t, Reason: reason}); err != nil {
		return err
	}
	if _, err := s.SetStatus(ctx, staffID, p.ID, StatusActive, reason); err != nil {
		return err
	}
	_, err = s.Assign(ctx, staffID, p.ID, AssignInput{Kind: ScopeArea, ScopeID: areaID, Reason: reason})
	return err
}

// SeedDefault creates the home country's "Country default" from the brand's
// seed file on a database that has none. Once a default exists, staff own the
// rules: re-seeding never publishes over them. Runs inside the seed transaction.
func (s *Service) SeedDefault(ctx context.Context, tx pgx.Tx, country string, t Terms) error {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1 || '/' || $2 || '/pricing'))`, s.store.brandID, country); err != nil {
		return err
	}
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pricing_profile_assignments WHERE brand_id=$1 AND scope_kind='COUNTRY' AND scope_id=$2 AND valid_to IS NULL)`,
		s.store.brandID, country).Scan(&exists); err != nil || exists {
		return err
	}
	return s.seedNewDefault(ctx, tx, country, t)
}

func (s *Service) seedNewDefault(ctx context.Context, tx pgx.Tx, country string, t Terms) error {
	var id string
	if err := tx.QueryRow(ctx, `INSERT INTO pricing_profiles (brand_id, country_code, name, status) VALUES ($1,$2,'Country default','ACTIVE') RETURNING id`,
		s.store.brandID, country).Scan(&id); err != nil {
		return err
	}
	v, err := s.insertVersion(ctx, tx, "", id, s.now(), VersionInput{Terms: t, Reason: "seed"})
	if err != nil {
		return err
	}
	var assignment string
	if err := tx.QueryRow(ctx, `INSERT INTO pricing_profile_assignments (brand_id, country_code, profile_id, scope_kind, scope_id, reason)
		VALUES ($1,$2,$3,'COUNTRY',$4,'seed') RETURNING id`, s.store.brandID, country, id, country).Scan(&assignment); err != nil {
		return err
	}
	if err := s.store.event(ctx, tx, id, EventCreated, "", "seed", nil, nil, map[string]any{"name": "Country default"}); err != nil {
		return err
	}
	if err := s.store.event(ctx, tx, id, EventPublished, "", "seed", &v.n, nil, nil); err != nil {
		return err
	}
	return s.store.event(ctx, tx, id, EventMadeDefault, "", "seed", nil, &assignment, nil)
}
