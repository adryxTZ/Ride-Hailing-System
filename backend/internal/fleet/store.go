package fleet

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/scope"
)

type store struct {
	pool    *pgxpool.Pool
	brandID string
}

const profileColumns = `u.id, u.full_name, u.phone, COALESCE(p.status::text, 'ONBOARDING'), p.licence_number,
	p.licence_expiry::text, p.national_id, p.status_reason, p.submitted_at, p.reviewed_at, COALESCE(p.created_at, u.created_at),
	p.licence_class, p.country_code`

func scanProfile(row pgx.Row) (*Profile, error) {
	var p Profile
	err := row.Scan(&p.UserID, &p.FullName, &p.Phone, &p.Status, &p.LicenceNumber, &p.LicenceExpiry,
		&p.NationalID, &p.StatusReason, &p.SubmittedAt, &p.ReviewedAt, &p.CreatedAt, &p.LicenceClass, &p.Country)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &p, err
}

// ensureProfile creates the ONBOARDING row the first time a driver opens sign-up.
func (s store) ensureProfile(ctx context.Context, userID string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO driver_profiles (user_id, brand_id, country_code) SELECT $1, $2, country FROM brands WHERE id=$2 ON CONFLICT (user_id) DO NOTHING`, userID, s.brandID)
	return err
}

func (s store) profile(ctx context.Context, userID string) (*Profile, error) {
	return scanProfile(s.pool.QueryRow(ctx, `
		SELECT `+profileColumns+` FROM users u JOIN driver_profiles p ON p.user_id = u.id
		WHERE u.id=$1 AND u.brand_id=$2`, userID, s.brandID))
}

func (s store) saveProfile(ctx context.Context, userID string, in ProfileInput) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `UPDATE users SET full_name=$2, updated_at=now() WHERE id=$1`, userID, in.FullName); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			UPDATE driver_profiles SET licence_number=$2, licence_expiry=NULLIF($3,'')::date, national_id=$4, licence_class=NULLIF($5,''), updated_at=now()
			WHERE user_id=$1`, userID, in.LicenceNumber, in.LicenceExpiry, in.NationalID, in.LicenceClass)
		return err
	})
}

func (s store) setStatus(ctx context.Context, tx pgx.Tx, userID, status string, reason *string, reviewer *string) error {
	_, err := tx.Exec(ctx, `
		UPDATE driver_profiles SET status=$2::driver_status, status_reason=$3, updated_at=now(),
		  reviewed_by=COALESCE($4, reviewed_by), reviewed_at=CASE WHEN $4::uuid IS NULL THEN reviewed_at ELSE now() END,
		  submitted_at=CASE WHEN $2='PENDING_REVIEW' THEN now() ELSE submitted_at END
		WHERE user_id=$1`, userID, status, reason, reviewer)
	return err
}

const vehicleColumns = `id, driver_id, service_key, class_key, plate, make, model, colour, year, status::text, status_reason`

func scanVehicle(row pgx.Row) (*Vehicle, error) {
	var v Vehicle
	err := row.Scan(&v.ID, &v.DriverID, &v.ServiceKey, &v.ClassKey, &v.Plate, &v.Make, &v.Model, &v.Colour, &v.Year, &v.Status, &v.StatusReason)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &v, err
}

func (s store) vehicleOf(ctx context.Context, driverID string) (*Vehicle, error) {
	return scanVehicle(s.pool.QueryRow(ctx, `
		SELECT `+vehicleColumns+` FROM vehicles WHERE driver_id=$1 AND brand_id=$2 ORDER BY created_at DESC LIMIT 1`, driverID, s.brandID))
}

// saveVehicle creates the driver's vehicle or updates it; any edit needs a fresh review.
func (s store) saveVehicle(ctx context.Context, driverID string, in VehicleInput) (*Vehicle, error) {
	v, err := scanVehicle(s.pool.QueryRow(ctx, `
		WITH existing AS (SELECT id FROM vehicles WHERE driver_id=$1 AND brand_id=$2 ORDER BY created_at DESC LIMIT 1),
		upd AS (
		  UPDATE vehicles SET service_key=$3, class_key=$4, plate=$5, make=$6, model=$7, colour=$8, year=$9,
		    status='PENDING', status_reason=NULL, updated_at=now()
		  WHERE id = (SELECT id FROM existing) RETURNING `+vehicleColumns+`),
		ins AS (
		  INSERT INTO vehicles (brand_id, driver_id, service_key, class_key, plate, make, model, colour, year)
		  SELECT $2, $1, $3, $4, $5, $6, $7, $8, $9 WHERE NOT EXISTS (SELECT 1 FROM existing)
		  RETURNING `+vehicleColumns+`)
		SELECT * FROM upd UNION ALL SELECT * FROM ins`,
		driverID, s.brandID, in.ServiceKey, in.ClassKey, in.Plate, in.Make, in.Model, in.Colour, in.Year))
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return nil, ErrPlateTaken
	}
	return v, err
}

func (s store) setVehicleStatus(ctx context.Context, tx pgx.Tx, vehicleID, status string, reason *string) error {
	_, err := tx.Exec(ctx, `UPDATE vehicles SET status=$2::review_status, status_reason=$3, updated_at=now() WHERE id=$1`, vehicleID, status, reason)
	return err
}

const documentColumns = `id, owner_kind::text, owner_id, kind, file_key, content_type, status::text, status_reason, expires_on::text, created_at, reviewed_at, size_bytes`

// documents returns the owners' current documents, each with its pending
// renewal attached (archived copies are history and not returned).
func (s store) documents(ctx context.Context, ownerIDs []string) ([]Document, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+documentColumns+`, renewal_of FROM documents WHERE brand_id=$1 AND owner_id = ANY($2) AND archived_at IS NULL
		ORDER BY created_at`, s.brandID, ownerIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var all []Document
	for rows.Next() {
		var d Document
		if err := rows.Scan(&d.ID, &d.OwnerKind, &d.OwnerID, &d.Kind, &d.FileKey, &d.ContentType, &d.Status, &d.StatusReason, &d.ExpiresOn,
			&d.CreatedAt, &d.ReviewedAt, &d.SizeBytes, &d.renewalOf); err != nil {
			return nil, err
		}
		all = append(all, d)
	}
	return attachRenewals(all), rows.Err()
}

// attachRenewals puts each renewal under the document it renews.
func attachRenewals(all []Document) []Document {
	out := []Document{}
	renewals := map[string]*Document{}
	for i := range all {
		if all[i].renewalOf != nil {
			renewals[*all[i].renewalOf] = &all[i]
		}
	}
	for _, d := range all {
		if d.renewalOf == nil {
			d.Renewal = renewals[d.ID]
			out = append(out, d)
		}
	}
	return out
}

// putDocument replaces the owner's current document of that kind (re-upload).
func (s store) putDocument(ctx context.Context, owner, ownerID string, in DocumentInput) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO documents (brand_id, owner_kind, owner_id, kind, file_key, content_type, expires_on, size_bytes)
		VALUES ($1, $2::document_owner, $3, $4, $5, $6, NULLIF($7,'')::date, $8)
		ON CONFLICT (owner_kind, owner_id, kind) WHERE archived_at IS NULL AND renewal_of IS NULL DO UPDATE SET file_key=EXCLUDED.file_key, content_type=EXCLUDED.content_type, size_bytes=EXCLUDED.size_bytes,
		  expires_on=EXCLUDED.expires_on, status='PENDING', status_reason=NULL, reviewed_by=NULL, reviewed_at=NULL, created_at=now()`,
		s.brandID, owner, ownerID, in.Kind, in.FileKey, in.ContentType, deref(in.ExpiresOn), in.sizeBytes)
	return err
}

func (s store) reviewDocument(ctx context.Context, tx pgx.Tx, docID, status string, reason, reviewer *string) (string, error) {
	var ownerID string
	err := tx.QueryRow(ctx, `
		UPDATE documents SET status=$2::review_status, status_reason=$3, reviewed_by=$4, reviewed_at=now()
		WHERE id=$1 AND brand_id=$5 RETURNING owner_id`, docID, status, reason, reviewer, s.brandID).Scan(&ownerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrDriverNotFound
	}
	return ownerID, err
}

// approvePending approves every still-pending document of the owners (bulk approval).
func (s store) approvePending(ctx context.Context, tx pgx.Tx, ownerIDs []string, reviewer string) error {
	_, err := tx.Exec(ctx, `
		UPDATE documents SET status='APPROVED', reviewed_by=$2, reviewed_at=now()
		WHERE owner_id = ANY($1) AND status='PENDING'`, ownerIDs, reviewer)
	return err
}

func (s store) audit(ctx context.Context, tx pgx.Tx, actor, action, target string, reason *string, data any) error {
	raw, _ := json.Marshal(data)
	if data == nil {
		raw = []byte(`{}`)
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO audit_log (brand_id, actor_id, action, target, reason, data) VALUES ($1, NULLIF($2,'')::uuid, $3, $4, $5, $6)`,
		s.brandID, actor, action, target, reason, raw)
	return err
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// DriverRow is one line of the staff driver list.
type DriverRow struct {
	Profile
	Plate        *string  `json:"plate,omitempty"`
	Service      *string  `json:"service,omitempty"`
	VehicleClass *string  `json:"vehicle_class,omitempty"`
	RatingAvg    *float64 `json:"rating,omitempty"`
	Trips        int      `json:"trips_completed"`
}

// listDrivers filters by status and a free-text match on name, phone or plate.
func (s store) listDrivers(ctx context.Context, status, q string, sc scope.Scope, limit, offset int) ([]DriverRow, int, error) {
	const where = `u.brand_id=$1 AND ($2='' OR p.status::text=$2)
		AND ($3='' OR u.full_name ILIKE '%'||$3||'%' OR u.phone LIKE '%'||$3||'%' OR v.plate ILIKE '%'||$3||'%')
		AND ` + driverScopeSQL
	var total int
	if err := s.pool.QueryRow(ctx, `
		SELECT count(*) FROM users u JOIN driver_profiles p ON p.user_id=u.id
		LEFT JOIN LATERAL (SELECT plate FROM vehicles WHERE driver_id=u.id ORDER BY created_at DESC LIMIT 1) v ON true
		WHERE `+where, s.brandID, status, q, sc.Country, sc.AreaID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT `+profileColumns+`, v.plate, v.service_key, v.class_key,
		  (SELECT avg(score)::float8 FROM trip_ratings r WHERE r.ratee_id=u.id),
		  (SELECT count(*) FROM trips t WHERE t.driver_id=u.id AND t.status='COMPLETED')::int
		FROM users u JOIN driver_profiles p ON p.user_id=u.id
		LEFT JOIN LATERAL (SELECT plate, service_key, class_key FROM vehicles WHERE driver_id=u.id ORDER BY created_at DESC LIMIT 1) v ON true
		WHERE `+where+`
		ORDER BY (p.status='PENDING_REVIEW') DESC, p.submitted_at DESC NULLS LAST, p.created_at DESC
		LIMIT $6 OFFSET $7`, s.brandID, status, q, sc.Country, sc.AreaID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []DriverRow{}
	for rows.Next() {
		var d DriverRow
		p := &d.Profile
		if err := rows.Scan(&p.UserID, &p.FullName, &p.Phone, &p.Status, &p.LicenceNumber, &p.LicenceExpiry, &p.NationalID,
			&p.StatusReason, &p.SubmittedAt, &p.ReviewedAt, &p.CreatedAt, &p.LicenceClass, &p.Country, &d.Plate, &d.Service, &d.VehicleClass, &d.RatingAvg, &d.Trips); err != nil {
			return nil, 0, err
		}
		out = append(out, d)
	}
	return out, total, rows.Err()
}

// driverScopeSQL keeps drivers of country $4 and, when $5 is set, registered in area $5.
const driverScopeSQL = `($4='' OR p.country_code=$4)
		AND ($5='' OR EXISTS (SELECT 1 FROM driver_areas da WHERE da.driver_id=p.user_id AND da.area_id::text=$5))`

// countScopeSQL is driverScopeSQL numbered for the counts query ($2 country, $3 area).
const countScopeSQL = `($2='' OR p.country_code=$2)
		AND ($3='' OR EXISTS (SELECT 1 FROM driver_areas da WHERE da.driver_id=p.user_id AND da.area_id::text=$3))`

// statusCounts is how many drivers in the scope are in each status (dashboard tiles).
func (s store) statusCounts(ctx context.Context, sc scope.Scope) (map[string]int, error) {
	rows, err := s.pool.Query(ctx, `SELECT p.status::text, count(*) FROM driver_profiles p
		WHERE p.brand_id=$1 AND `+countScopeSQL+` GROUP BY p.status`, s.brandID, sc.Country, sc.AreaID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var k string
		var n int
		if err := rows.Scan(&k, &n); err != nil {
			return nil, err
		}
		out[k] = n
	}
	return out, rows.Err()
}

// AuditEntry is one staff decision.
type AuditEntry struct {
	ID      int64           `json:"id"`
	ActorID *string         `json:"actor_id,omitempty"`
	Actor   *string         `json:"actor,omitempty"`
	Action  string          `json:"action"`
	Target  string          `json:"target"`
	Reason  *string         `json:"reason,omitempty"`
	Data    json.RawMessage `json:"data"`
	At      string          `json:"at"`
}

func (s store) auditFor(ctx context.Context, target, actionPrefix, actorID string, limit int) ([]AuditEntry, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT a.id, a.actor_id, u.full_name, a.action, a.target, a.reason, a.data, a.at::text
		FROM audit_log a LEFT JOIN users u ON u.id=a.actor_id
		WHERE a.brand_id=$1 AND ($2='' OR a.target=$2) AND ($4='' OR a.action LIKE $4 || '%') AND ($5='' OR a.actor_id=NULLIF($5,'')::uuid)
		ORDER BY a.at DESC LIMIT $3`, s.brandID, target, limit, actionPrefix, actorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AuditEntry{}
	for rows.Next() {
		var e AuditEntry
		if err := rows.Scan(&e.ID, &e.ActorID, &e.Actor, &e.Action, &e.Target, &e.Reason, &e.Data, &e.At); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// countryOf is the country a driver works in.
func (s store) countryOf(ctx context.Context, userID string) (string, error) {
	var c *string
	err := s.pool.QueryRow(ctx, `SELECT country_code FROM driver_profiles WHERE user_id=$1`, userID).Scan(&c)
	if errors.Is(err, pgx.ErrNoRows) || c == nil {
		return "", nil
	}
	return *c, err
}

// CountryOf is the country a driver registered in ("" before they chose one).
func (s *Service) CountryOf(ctx context.Context, driverID string) (string, error) {
	return s.store.countryOf(ctx, driverID)
}

// documentTypes reads a country's document types in display order.
func (s store) documentTypes(ctx context.Context, country string) ([]DocumentType, error) {
	rows, err := s.pool.Query(ctx, `SELECT country_code, kind, owner::text, label, requirement, tracks_expiry, expiry_required, blocks_when_expired, sort_order
		FROM document_types WHERE brand_id=$1 AND country_code=$2 ORDER BY sort_order, kind`, s.brandID, country)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DocumentType{}
	for rows.Next() {
		var t DocumentType
		if err := rows.Scan(&t.Country, &t.Kind, &t.Owner, &t.Label, &t.Requirement, &t.TracksExpiry, &t.ExpiryRequired, &t.BlocksWhenExpired, &t.SortOrder); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s store) upsertDocumentType(ctx context.Context, tx pgx.Tx, t DocumentType, staffID string) error {
	_, err := tx.Exec(ctx, `INSERT INTO document_types (brand_id, country_code, kind, owner, label, requirement, tracks_expiry, expiry_required, blocks_when_expired, sort_order, updated_by)
		VALUES ($1,$2,$3,$4::document_owner,$5,$6,$7,$8,$9,$10,$11)
		ON CONFLICT (brand_id, country_code, kind) DO UPDATE SET owner=EXCLUDED.owner, label=EXCLUDED.label, requirement=EXCLUDED.requirement,
		  tracks_expiry=EXCLUDED.tracks_expiry, expiry_required=EXCLUDED.expiry_required, blocks_when_expired=EXCLUDED.blocks_when_expired,
		  sort_order=EXCLUDED.sort_order, updated_by=EXCLUDED.updated_by, updated_at=now()`,
		s.brandID, t.Country, t.Kind, t.Owner, t.Label, t.Requirement, t.TracksExpiry, t.ExpiryRequired, t.BlocksWhenExpired, t.SortOrder, staffID)
	return err
}
