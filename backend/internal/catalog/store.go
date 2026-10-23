package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// store is the persistence layer: one query to load the catalogue, upserts
// for seeding, and the version bump every writer must call.
type store struct{ pool *pgxpool.Pool }

// Version implements cache.VersionSource for the "catalog" scope.
func (s store) Version(ctx context.Context, scope string) (int64, error) {
	var v int64
	err := s.pool.QueryRow(ctx, `SELECT version FROM config_versions WHERE scope=$1`, scope).Scan(&v)
	return v, err
}

func (s store) bumpVersion(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `UPDATE config_versions SET version = version + 1, updated_at = now() WHERE scope='catalog'`)
	return err
}

// load reads the whole catalogue for a brand in three queries (no N+1).
func (s store) load(ctx context.Context, brandID string) (Catalogue, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, key, category::text, name, description, status::text, sort_order, COALESCE(icon,''), lifecycle, request_schema, dispatch, compliance
		FROM services WHERE brand_id=$1 AND status <> 'DISABLED' ORDER BY sort_order, key`, brandID)
	if err != nil {
		return Catalogue{}, err
	}
	defer rows.Close()
	byID := map[string]*Service{}
	var order []string
	for rows.Next() {
		var id string
		var svc Service
		if err := rows.Scan(&id, &svc.Key, &svc.Category, &svc.Name, &svc.Description, &svc.Status, &svc.SortOrder, &svc.Icon,
			&svc.Lifecycle, &svc.RequestSchema, &svc.Dispatch, &svc.Compliance); err != nil {
			return Catalogue{}, err
		}
		svc.VehicleClasses = []VehicleClass{}
		byID[id] = &svc
		order = append(order, id)
	}
	if err := rows.Err(); err != nil {
		return Catalogue{}, err
	}
	if len(order) == 0 {
		return Catalogue{Services: []Service{}}, nil
	}

	vc, err := s.pool.Query(ctx, `
		SELECT service_id, key, name, description, COALESCE(icon,''), sort_order, seats, COALESCE(regulator_type,''), status::text
		FROM vehicle_classes WHERE service_id = ANY($1) AND status <> 'DISABLED' ORDER BY sort_order, key`, order)
	if err != nil {
		return Catalogue{}, err
	}
	defer vc.Close()
	for vc.Next() {
		var sid string
		var c VehicleClass
		if err := vc.Scan(&sid, &c.Key, &c.Name, &c.Description, &c.Icon, &c.SortOrder, &c.Seats, &c.RegulatorType, &c.Status); err != nil {
			return Catalogue{}, err
		}
		byID[sid].VehicleClasses = append(byID[sid].VehicleClasses, c)
	}

	out := Catalogue{Services: make([]Service, 0, len(order))}
	for _, id := range order {
		out.Services = append(out.Services, *byID[id])
	}
	return out, nil
}

// seedFile is the on-disk shape of seed/catalog/<service>.json.
type seedFile struct {
	Service
	Status string `json:"status"`
}

// Seed upserts every seed/catalog/*.json for the brand and bumps the version once.
func (s store) seed(ctx context.Context, brandID, dir string) (int, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return 0, err
	}
	sort.Strings(paths)
	n := 0
	var seeded []string
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		for _, p := range paths {
			raw, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			var f seedFile
			if err := json.Unmarshal(raw, &f); err != nil {
				return fmt.Errorf("%s: %w", filepath.Base(p), err)
			}
			if f.Status == "" {
				f.Status = "ACTIVE"
			}
			if err := s.upsertService(ctx, tx, brandID, f); err != nil {
				return fmt.Errorf("%s: %w", filepath.Base(p), err)
			}
			seeded = append(seeded, f.Key)
			n++
		}
		// A service whose seed file is gone is switched off, not deleted: old trips keep their names.
		if _, err := tx.Exec(ctx, `UPDATE services SET status='DISABLED', updated_at=now()
			WHERE brand_id=$1 AND NOT (key = ANY($2)) AND status <> 'DISABLED'`, brandID, seeded); err != nil {
			return err
		}
		return s.bumpVersion(ctx, tx)
	})
	return n, err
}

func (s store) upsertService(ctx context.Context, tx pgx.Tx, brandID string, f seedFile) error {
	name, _ := json.Marshal(f.Name)
	desc, _ := json.Marshal(f.Description)
	var sid string
	err := tx.QueryRow(ctx, `
		INSERT INTO services (brand_id, key, category, name, description, status, sort_order, icon, lifecycle, request_schema, dispatch, compliance)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		ON CONFLICT (brand_id, key) DO UPDATE SET category=EXCLUDED.category, name=EXCLUDED.name, description=EXCLUDED.description,
		  status=EXCLUDED.status, sort_order=EXCLUDED.sort_order, icon=EXCLUDED.icon, lifecycle=EXCLUDED.lifecycle,
		  request_schema=EXCLUDED.request_schema, dispatch=EXCLUDED.dispatch, compliance=EXCLUDED.compliance, updated_at=now()
		RETURNING id`,
		brandID, f.Key, f.Category, name, desc, f.Status, f.SortOrder, nilIfEmpty(f.Icon),
		orEmptyJSON(f.Lifecycle), orEmptyJSON(f.RequestSchema), orEmptyJSON(f.Dispatch), orEmptyJSON(f.Compliance)).Scan(&sid)
	if err != nil {
		return err
	}
	for _, c := range f.VehicleClasses {
		cn, _ := json.Marshal(c.Name)
		cd, _ := json.Marshal(c.Description)
		if _, err := tx.Exec(ctx, `
			INSERT INTO vehicle_classes (service_id, key, name, description, icon, sort_order, seats, regulator_type, status)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
			ON CONFLICT (service_id, key) DO UPDATE SET name=EXCLUDED.name, description=EXCLUDED.description, icon=EXCLUDED.icon,
			  sort_order=EXCLUDED.sort_order, seats=EXCLUDED.seats, regulator_type=EXCLUDED.regulator_type, status=EXCLUDED.status`,
			sid, c.Key, cn, cd, nilIfEmpty(c.Icon), c.SortOrder, c.Seats, nilIfEmpty(c.RegulatorType), classStatus(c.Status)); err != nil {
			return err
		}
	}
	return nil
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func orEmptyJSON(r json.RawMessage) json.RawMessage {
	if len(r) == 0 {
		return json.RawMessage(`{}`)
	}
	return r
}

// classStatus defaults a seeded class to ACTIVE; DISABLED keeps old trips readable but stops new bookings.
func classStatus(s string) string {
	if s == "" {
		return "ACTIVE"
	}
	return s
}
