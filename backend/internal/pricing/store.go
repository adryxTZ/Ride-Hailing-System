package pricing

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/kalen-limited/hauler-rider/backend/internal/providers/routing"
	"os"
	"path/filepath"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/pricing/engine"
)

// tariffs is the cached snapshot: vehicle-class key → tariff, and the fixed fares.
// Money rules live in pricing profiles (internal/profiles).
type tariffs struct {
	ByClass map[string]engine.Tariff // key: "<country>/<area id or ->/<service_key>/<class_key>"
	Routes  []FixedRoute
}

// scopeKey builds the cache key for a country and optional area.
func scopeKey(country, areaID string) string {
	if areaID == "" {
		areaID = "-"
	}
	return country + "/" + areaID
}

// lookup finds the tariff for a class: the area's own, else the country's.
func (t tariffs) lookup(country, areaID, service, class string) (engine.Tariff, bool) {
	if areaID != "" {
		if tf, ok := t.ByClass[scopeKey(country, areaID)+"/"+service+"/"+class]; ok {
			return tf, true
		}
	}
	tf, ok := t.ByClass[scopeKey(country, "")+"/"+service+"/"+class]
	return tf, ok
}

type store struct {
	pool        *pgxpool.Pool
	brandID     string
	homeCountry string // the brand's home country: the default scope for seeds and legacy callers
}

// Version implements cache.VersionSource for the "pricing" scope.
func (s store) Version(ctx context.Context, scope string) (int64, error) {
	var v int64
	err := s.pool.QueryRow(ctx, `SELECT version FROM config_versions WHERE scope=$1`, scope).Scan(&v)
	return v, err
}

// load reads every tariff of the brand with its brackets (two queries) and the fixed fares.
func (s store) load(ctx context.Context) (tariffs, error) {
	out := tariffs{ByClass: map[string]engine.Tariff{}}
	rows, err := s.pool.Query(ctx, `
		SELECT t.id, sv.key, vc.key, t.currency, t.minimum_fare, t.waiting_rate_per_min, t.pickup_approach_per_km,
		       t.loader_rate, t.loading_grace_s, t.unloading_grace_s, t.surge_enabled, t.engine, t.params, t.cancellation_fee,
		       t.country_code, coalesce(t.area_id::text, '')
		FROM tariffs t
		JOIN vehicle_classes vc ON vc.id = t.vehicle_class_id
		JOIN services sv ON sv.id = vc.service_id
		WHERE sv.brand_id = $1 AND t.status = 'ACTIVE' AND sv.status <> 'DISABLED' AND vc.status <> 'DISABLED'`, s.brandID)
	if err != nil {
		return out, err
	}
	ids := map[string]string{} // tariff id → class key
	for rows.Next() {
		var id, svc, cls, country, area string
		var t engine.Tariff
		var params map[string]decimal.Decimal
		if err := rows.Scan(&id, &svc, &cls, &t.Currency, &t.MinimumFare, &t.WaitingRatePerMin, &t.PickupApproachPerKm,
			&t.LoaderRate, &t.LoadingGraceS, &t.UnloadingGraceS, &t.SurgeEnabled, &t.Engine, &params, &t.CancellationFee, &country, &area); err != nil {
			rows.Close()
			return out, err
		}
		t.Params = params
		key := scopeKey(country, area) + "/" + svc + "/" + cls
		ids[id] = key
		out.ByClass[key] = t
	}
	rows.Close()

	br, err := s.pool.Query(ctx, `
		SELECT tariff_id, kind::text, min_value, max_value, unit_price
		FROM tariff_brackets WHERE tariff_id = ANY($1) ORDER BY tariff_id, kind, min_value`, keys(ids))
	if err != nil {
		return out, err
	}
	defer br.Close()
	for br.Next() {
		var tid, kind string
		var b engine.Bracket
		var max *decimal.Decimal
		if err := br.Scan(&tid, &kind, &b.Min, &max, &b.UnitPrice); err != nil {
			return out, err
		}
		b.Max = max
		t := out.ByClass[ids[tid]]
		if kind == "DISTANCE" {
			t.Distance = append(t.Distance, b)
		} else {
			t.Duration = append(t.Duration, b)
		}
		out.ByClass[ids[tid]] = t
	}

	fr, err := s.pool.Query(ctx, `
		SELECT id, service_key, name, ST_Y(origin::geometry), ST_X(origin::geometry), origin_radius_m,
		       ST_Y(destination::geometry), ST_X(destination::geometry), destination_radius_m, bidirectional,
		       vehicle_classes, price, reverse_price, currency, includes_loaders, surge_applies, priority, valid_from, valid_to,
		       country_code, coalesce(area_id::text, '')
		FROM fixed_routes fr WHERE brand_id=$1 AND status='ACTIVE'
		  AND EXISTS (SELECT 1 FROM services sv WHERE sv.brand_id=fr.brand_id AND sv.key=fr.service_key AND sv.status <> 'DISABLED')
		ORDER BY priority DESC, name`, s.brandID)
	if err != nil {
		return out, err
	}
	defer fr.Close()
	for fr.Next() {
		var r FixedRoute
		if err := fr.Scan(&r.ID, &r.ServiceKey, &r.Name, &r.Origin.Lat, &r.Origin.Lng, &r.OriginRadiusM,
			&r.Destination.Lat, &r.Destination.Lng, &r.DestinationRadiusM, &r.Bidirectional,
			&r.VehicleClasses, &r.Price, &r.ReversePrice, &r.Currency, &r.IncludesLoaders, &r.SurgeApplies, &r.Priority, &r.ValidFrom, &r.ValidTo,
			&r.Country, &r.AreaID); err != nil {
			return out, err
		}
		out.Routes = append(out.Routes, r)
	}
	return out, nil
}

// seedRoutes upserts seed/pricing/fixed_routes.json.
func (s store) seedRoutes(ctx context.Context, tx pgx.Tx, raw []byte) (int, error) {
	var f struct {
		Routes []struct {
			Service            string                     `json:"service"`
			Name               string                     `json:"name"`
			Origin             struct{ Lat, Lng float64 } `json:"origin"`
			OriginRadiusM      int                        `json:"origin_radius_m"`
			Destination        struct{ Lat, Lng float64 } `json:"destination"`
			DestinationRadiusM int                        `json:"destination_radius_m"`
			Bidirectional      bool                       `json:"bidirectional"`
			VehicleClasses     []string                   `json:"vehicle_classes"`
			Price              decimal.Decimal            `json:"price"`
			ReversePrice       *decimal.Decimal           `json:"reverse_price"`
			Currency           string                     `json:"currency"`
			IncludesLoaders    bool                       `json:"includes_loaders"`
			SurgeApplies       bool                       `json:"surge_applies"`
			Priority           int                        `json:"priority"`
		} `json:"routes"`
	}
	if err := json.Unmarshal(raw, &f); err != nil {
		return 0, fmt.Errorf("fixed_routes.json: %w", err)
	}
	for _, r := range f.Routes {
		if r.VehicleClasses == nil {
			r.VehicleClasses = []string{}
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO fixed_routes (brand_id, service_key, name, origin, origin_radius_m, destination, destination_radius_m, bidirectional,
			  vehicle_classes, price, reverse_price, currency, includes_loaders, surge_applies, priority, country_code)
			VALUES ($1,$2,$3, ST_GeogFromText($4), $5, ST_GeogFromText($6), $7, $8, $9, $10, $15, $11, $12, $13, $14, $16)
			ON CONFLICT (brand_id, service_key, name) DO UPDATE SET origin=EXCLUDED.origin, origin_radius_m=EXCLUDED.origin_radius_m, country_code=EXCLUDED.country_code,
			  destination=EXCLUDED.destination, destination_radius_m=EXCLUDED.destination_radius_m, bidirectional=EXCLUDED.bidirectional,
			  vehicle_classes=EXCLUDED.vehicle_classes, price=EXCLUDED.price, reverse_price=EXCLUDED.reverse_price, currency=EXCLUDED.currency,
			  includes_loaders=EXCLUDED.includes_loaders, surge_applies=EXCLUDED.surge_applies, priority=EXCLUDED.priority, updated_at=now()`,
			s.brandID, r.Service, r.Name, fmt.Sprintf("SRID=4326;POINT(%f %f)", r.Origin.Lng, r.Origin.Lat), r.OriginRadiusM,
			fmt.Sprintf("SRID=4326;POINT(%f %f)", r.Destination.Lng, r.Destination.Lat), r.DestinationRadiusM, r.Bidirectional,
			r.VehicleClasses, r.Price, r.Currency, r.IncludesLoaders, r.SurgeApplies, r.Priority, r.ReversePrice, s.homeCountry); err != nil {
			return 0, err
		}
	}
	return len(f.Routes), nil
}

// saveQuote persists a quote so a booking can lock exactly that price.
func (s store) saveQuote(ctx context.Context, req Request, q *Quote, attrs json.RawMessage) error {
	options, _ := json.Marshal(q.Options)
	pricing, _ := json.Marshal(q.pricing)
	route, _ := json.Marshal(q.Route)
	var userID *string
	if req.UserID != "" {
		userID = &req.UserID
	}
	var dropWKT *string
	if req.Dropoff != nil {
		w := fmt.Sprintf("SRID=4326;POINT(%f %f)", req.Dropoff.Lng, req.Dropoff.Lat)
		dropWKT = &w
	}
	return s.pool.QueryRow(ctx, `
		INSERT INTO quotes (brand_id, user_id, service_key, pickup, dropoff, attributes, route, surge, options, pricing_version, expires_at, waypoints, country_code, area_id, pricing)
		VALUES ($1, $2, $3, ST_GeogFromText($4), ST_GeogFromText($5), $6, $7, $8, $9, $10, $11, $12, $13, NULLIF($14,'')::uuid, $15) RETURNING id`,
		s.brandID, userID, q.ServiceKey, fmt.Sprintf("SRID=4326;POINT(%f %f)", req.Pickup.Lng, req.Pickup.Lat), dropWKT,
		orEmpty(attrs), route, q.Surge, options, q.PricingVersion, q.ExpiresAt, waypointsJSON(req.Waypoints), q.Country, q.Area.ID, pricing).Scan(&q.ID)
}

// seedFile mirrors seed/pricing/<service>.json.
type seedFile struct {
	Service  string `json:"service"`
	Currency string `json:"currency"`
	Tariffs  map[string]struct {
		MinimumFare         decimal.Decimal            `json:"minimum_fare"`
		WaitingRatePerMin   decimal.Decimal            `json:"waiting_rate_per_min"`
		PickupApproachPerKm decimal.Decimal            `json:"pickup_approach_per_km"`
		LoaderRate          decimal.Decimal            `json:"loader_rate"`
		CancellationFee     decimal.Decimal            `json:"cancellation_fee"`
		Engine              string                     `json:"engine"`
		Params              map[string]decimal.Decimal `json:"params"`
		LoadingGraceS       int                        `json:"loading_grace_s"`
		UnloadingGraceS     int                        `json:"unloading_grace_s"`
		Distance            [][]*json.Number           `json:"distance"`
		Duration            [][]*json.Number           `json:"duration"`
	} `json:"tariffs"`
}

// policySeeder publishes fee_policy.json as the home country's default profile (inside the seed transaction).
type policySeeder func(ctx context.Context, tx pgx.Tx, raw []byte) error

// seed upserts tariffs from seed/pricing/*.json plus fee_policy.json, bumping the pricing version once.
func (s store) seed(ctx context.Context, dir string, seedPolicy policySeeder) (int, error) {
	paths, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	n := 0
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		for _, p := range paths {
			raw, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			if filepath.Base(p) == "fee_policy.json" {
				if err := seedPolicy(ctx, tx, raw); err != nil {
					return err
				}
				continue
			}
			if filepath.Base(p) == "fixed_routes.json" {
				k, err := s.seedRoutes(ctx, tx, raw)
				if err != nil {
					return err
				}
				n += k
				continue
			}
			var f seedFile
			if err := json.Unmarshal(raw, &f); err != nil {
				return fmt.Errorf("%s: %w", filepath.Base(p), err)
			}
			for cls, t := range f.Tariffs {
				var tid string
				eng := t.Engine
				if eng == "" {
					eng = engine.DefaultKey
				}
				if engine.Get(eng) == nil {
					return fmt.Errorf("%s/%s: unknown pricing engine %q", f.Service, cls, eng)
				}
				params, _ := json.Marshal(t.Params)
				if t.Params == nil {
					params = []byte("{}")
				}
				if err := tx.QueryRow(ctx, `
					INSERT INTO tariffs (vehicle_class_id, currency, minimum_fare, waiting_rate_per_min, pickup_approach_per_km, loader_rate, loading_grace_s, unloading_grace_s, engine, params, cancellation_fee, country_code)
					SELECT vc.id, $3, $4, $5, $6, $7, $8, $9, $11, $12, $13, $14 FROM vehicle_classes vc JOIN services sv ON sv.id = vc.service_id
					WHERE sv.brand_id=$1 AND sv.key=$2 AND vc.key=$10
					ON CONFLICT ON CONSTRAINT tariffs_scope_uq DO UPDATE SET currency=EXCLUDED.currency, minimum_fare=EXCLUDED.minimum_fare,
					  waiting_rate_per_min=EXCLUDED.waiting_rate_per_min, pickup_approach_per_km=EXCLUDED.pickup_approach_per_km,
					  loader_rate=EXCLUDED.loader_rate, loading_grace_s=EXCLUDED.loading_grace_s, unloading_grace_s=EXCLUDED.unloading_grace_s,
					  engine=EXCLUDED.engine, params=EXCLUDED.params, cancellation_fee=EXCLUDED.cancellation_fee, updated_at=now()
					RETURNING id`,
					s.brandID, f.Service, f.Currency, t.MinimumFare, t.WaitingRatePerMin, t.PickupApproachPerKm, t.LoaderRate, t.LoadingGraceS, t.UnloadingGraceS, cls, eng, params, t.CancellationFee, s.homeCountry).Scan(&tid); err != nil {
					return fmt.Errorf("%s/%s: %w (is the catalogue seeded?)", f.Service, cls, err)
				}
				if _, err := tx.Exec(ctx, `DELETE FROM tariff_brackets WHERE tariff_id=$1`, tid); err != nil {
					return err
				}
				for kind, rows := range map[string][][]*json.Number{"DISTANCE": t.Distance, "DURATION": t.Duration} {
					for i, r := range rows {
						if len(r) != 3 {
							return fmt.Errorf("%s/%s %s bracket %d: need [min, max|null, price]", f.Service, cls, kind, i)
						}
						var max *string
						if r[1] != nil {
							v := r[1].String()
							max = &v
						}
						if _, err := tx.Exec(ctx, `INSERT INTO tariff_brackets (tariff_id, kind, min_value, max_value, unit_price, sort_order) VALUES ($1,$2,$3,$4,$5,$6)`,
							tid, kind, r[0].String(), max, r[2].String(), i); err != nil {
							return err
						}
					}
				}
				n++
			}
		}
		_, err := tx.Exec(ctx, `UPDATE config_versions SET version = version + 1, updated_at = now() WHERE scope='pricing'`)
		return err
	})
	return n, err
}

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func orEmpty(r json.RawMessage) json.RawMessage {
	if len(r) == 0 {
		return json.RawMessage(`{}`)
	}
	return r
}

func waypointsJSON(w []routing.Point) []byte {
	if len(w) == 0 {
		return []byte("[]")
	}
	b, _ := json.Marshal(w)
	return b
}
