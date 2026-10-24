package profiles

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// Record is a trip's immutable pricing record.
type Record struct {
	TripID string `json:"trip_id"`
	// AreaID is the trip's region (for staff limited to a region).
	AreaID      string          `json:"-"`
	QuoteID     *string         `json:"quote_id"`
	Country     *string         `json:"country"`
	Profile     Ref             `json:"profile"`
	Scope       ScopeRef        `json:"scope"`
	Default     *Ref            `json:"default_profile"`
	Terms       json.RawMessage `json:"terms"`
	Charges     json.RawMessage `json:"charges"`
	Trace       json.RawMessage `json:"trace"`
	Config      json.RawMessage `json:"config"`
	Source      string          `json:"source"`
	Stage       string          `json:"stage"`
	Final       json.RawMessage `json:"final,omitempty"`
	RecordHash  *string         `json:"record_hash"`
	CreatedAt   time.Time       `json:"created_at"`
	FinalizedAt *time.Time      `json:"finalized_at"`
	// SealValid is whether the stored hash still matches the record (nil before
	// sealing, and for records reconstructed from history).
	SealValid *bool `json:"seal_valid"`
}

// Pinned is what a quote stores per vehicle class, and booking copies to the record.
type Pinned struct {
	Profile Ref             `json:"profile"`
	Scope   ScopeRef        `json:"scope"`
	Default Ref             `json:"default"`
	Terms   Effective       `json:"terms"`
	Charges []AppliedCharge `json:"charges"`
	Trace   Trace           `json:"trace"`
	Config  Config          `json:"config"`
}

// QuotePricing is quotes.pricing: the shared resolution and per-class pins.
type QuotePricing struct {
	Profile  Ref                         `json:"profile"`
	Scope    ScopeRef                    `json:"scope"`
	Default  Ref                         `json:"default"`
	Trace    Trace                       `json:"trace"`
	Config   Config                      `json:"config"`
	PerClass map[string]QuoteClassPinned `json:"per_class"`
}

// QuoteClassPinned is one class's merged rules and applied charges on a quote.
type QuoteClassPinned struct {
	Terms   Effective       `json:"terms"`
	Charges []AppliedCharge `json:"charges"`
}

// Record sources.
const (
	SourceLive     = "LIVE"
	SourceFallback = "FALLBACK"
)

// PinTx writes a trip's record from its quote's pricing for the booked class, in
// the booking transaction. A quote without pricing (made before profiles) is
// skipped; the split then pins a fallback record.
func (s *Service) PinTx(ctx context.Context, tx pgx.Tx, tripID, quoteID, vehicleClass string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO trip_pricing_records (trip_id, brand_id, quote_id, country_code, profile_id, profile_version_id, profile_version,
		  default_profile_id, default_version_id, scope_kind, scope_id, scope_name, terms, charges, trace, config, source)
		SELECT $1, q.brand_id, q.id, q.country_code, (q.pricing#>>'{profile,id}')::uuid, (q.pricing#>>'{profile,version_id}')::uuid,
		  (q.pricing#>>'{profile,version}')::int, (q.pricing#>>'{default,id}')::uuid, (q.pricing#>>'{default,version_id}')::uuid,
		  q.pricing#>>'{scope,kind}', q.pricing#>>'{scope,id}', q.pricing#>>'{scope,name}',
		  q.pricing->'per_class'->$3->'terms', coalesce(q.pricing->'per_class'->$3->'charges', '[]'), q.pricing->'trace',
		  q.pricing->'config' || jsonb_build_object('profile_name', q.pricing#>>'{profile,name}', 'default_name', q.pricing#>>'{default,name}'), 'LIVE'
		FROM quotes q WHERE q.id::text=$2 AND q.pricing IS NOT NULL AND q.pricing->'per_class' ? $3
		ON CONFLICT (trip_id) DO NOTHING`, tripID, quoteID, vehicleClass)
	return err
}

// PinnedFor reads a trip's pinned rules and charges inside a transaction (the split's only source of rates).
func (s *Service) PinnedFor(ctx context.Context, tx pgx.Tx, tripID string) (*Pinned, bool, error) {
	var p Pinned
	var terms, charges []byte
	err := tx.QueryRow(ctx, `SELECT profile_id, profile_version_id, profile_version, coalesce(default_version_id::text,''), scope_kind, scope_id, terms, charges
		FROM trip_pricing_records WHERE trip_id=$1`, tripID).
		Scan(&p.Profile.ID, &p.Profile.VersionID, &p.Profile.Version, &p.Default.VersionID, &p.Scope.Kind, &p.Scope.ID, &terms, &charges)
	if err == pgx.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if err := json.Unmarshal(terms, &p.Terms); err != nil {
		return nil, false, err
	}
	if err := json.Unmarshal(charges, &p.Charges); err != nil {
		return nil, false, fmt.Errorf("pricing record %s: charges unreadable: %w", tripID, err)
	}
	return &p, true, nil
}

// PinFallbackTx pins the country (or area) rules in force now for a trip booked
// before profiles existed, marked FALLBACK so the record says so.
func (s *Service) PinFallbackTx(ctx context.Context, tx pgx.Tx, tripID, country, areaID, areaName string) (*Pinned, error) {
	res, err := s.Resolve(ctx, Input{Country: country, AreaID: areaID, AreaName: areaName})
	if err != nil {
		return nil, err
	}
	p := &Pinned{Profile: res.Profile, Scope: res.Scope, Default: res.Default, Terms: res.Terms, Charges: []AppliedCharge{}, Trace: res.Trace, Config: res.Config}
	terms, _ := json.Marshal(p.Terms)
	trace, _ := json.Marshal(p.Trace)
	config, _ := json.Marshal(pinnedConfig{Config: p.Config, ProfileName: p.Profile.Name, DefaultName: p.Default.Name})
	_, err = tx.Exec(ctx, `
		INSERT INTO trip_pricing_records (trip_id, brand_id, country_code, profile_id, profile_version_id, profile_version, default_profile_id,
		  default_version_id, scope_kind, scope_id, scope_name, terms, charges, trace, config, source)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,'[]',$13,$14,'FALLBACK') ON CONFLICT (trip_id) DO NOTHING`,
		tripID, s.store.brandID, country, p.Profile.ID, p.Profile.VersionID, p.Profile.Version, p.Default.ID, p.Default.VersionID,
		p.Scope.Kind, p.Scope.ID, p.Scope.Name, terms, trace, config)
	return p, err
}

// pinnedConfig is the config stored on a record: what resolution ran with, and
// the names in force then (a later rename must not change a sealed record).
type pinnedConfig struct {
	Config
	ProfileName string `json:"profile_name"`
	DefaultName string `json:"default_name"`
}

// recordHashSQL is the seal over a record's pinned columns and the trip's final
// split, '|'-separated in this order: trip id, profile version id, default
// version id, scope kind/id, terms, charges, trace, config, final split. It is
// computed by Postgres from the stored jsonb, so anyone can recompute it.
const recordHashSQL = `encode(sha256(convert_to(r.trip_id::text || '|' || r.profile_version_id::text || '|' || coalesce(r.default_version_id::text,'') || '|' ||
	r.scope_kind || '/' || r.scope_id || '|' || r.terms::text || '|' || r.charges::text || '|' || r.trace::text || '|' || r.config::text || '|' ||
	coalesce(r.final->'breakdown','null'::jsonb)::text, 'UTF8')), 'hex')`

// FinalizeTx seals a completed trip's record with its final split, in the
// completing transaction (one statement).
func (s *Service) FinalizeTx(ctx context.Context, tx pgx.Tx, tripID string) error {
	_, err := tx.Exec(ctx, `
		WITH t AS (SELECT id, fees, coalesce(completed_at, now()) AS done FROM trips WHERE id=$1),
		f AS (SELECT r.trip_id, jsonb_build_object('breakdown', t.fees, 'completed_at', t.done) AS final, t.done
		      FROM trip_pricing_records r JOIN t ON t.id=r.trip_id WHERE r.finalized_at IS NULL)
		UPDATE trip_pricing_records r SET stage='FINAL', final=f.final, finalized_at=f.done,
		  record_hash = (SELECT `+recordHashSQL+` FROM (SELECT r.trip_id, r.profile_version_id, r.default_version_id, r.scope_kind, r.scope_id,
		    r.terms, r.charges, r.trace, r.config, f.final) r)
		FROM f WHERE r.trip_id=f.trip_id`, tripID)
	return err
}

// Record returns a trip's pricing record (staff).
func (s *Service) Record(ctx context.Context, tripID string) (*Record, error) {
	var r Record
	var def Ref
	err := s.store.pool.QueryRow(ctx, `
		SELECT r.trip_id, coalesce((SELECT area_id::text FROM trips WHERE id=r.trip_id),''), r.quote_id::text, r.country_code, r.profile_id, coalesce(r.config->>'profile_name', p.name, ''), r.profile_version_id, r.profile_version,
		  r.scope_kind, r.scope_id, coalesce(r.scope_name,''), coalesce(r.default_profile_id::text,''), coalesce(r.config->>'default_name', d.name, ''), coalesce(r.default_version_id::text,''),
		  coalesce((SELECT version FROM pricing_profile_versions WHERE id=r.default_version_id),0),
		  r.terms, r.charges, r.trace, r.config, r.source, r.stage, r.final, r.record_hash, r.created_at, r.finalized_at,
		  CASE WHEN r.record_hash IS NULL OR r.source = 'BACKFILL' THEN NULL ELSE r.record_hash = `+recordHashSQL+` END
		FROM trip_pricing_records r LEFT JOIN pricing_profiles p ON p.id=r.profile_id LEFT JOIN pricing_profiles d ON d.id=r.default_profile_id
		WHERE r.trip_id::text=$1 AND r.brand_id=$2`, tripID, s.store.brandID).
		Scan(&r.TripID, &r.AreaID, &r.QuoteID, &r.Country, &r.Profile.ID, &r.Profile.Name, &r.Profile.VersionID, &r.Profile.Version,
			&r.Scope.Kind, &r.Scope.ID, &r.Scope.Name, &def.ID, &def.Name, &def.VersionID, &def.Version,
			&r.Terms, &r.Charges, &r.Trace, &r.Config, &r.Source, &r.Stage, &r.Final, &r.RecordHash, &r.CreatedAt, &r.FinalizedAt, &r.SealValid)
	if err == pgx.ErrNoRows {
		return nil, ErrRecordNotFound
	}
	if def.ID != "" {
		r.Default = &def
	}
	return &r, err
}
