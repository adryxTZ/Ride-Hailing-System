package profiles

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// store holds the queries; no rules here.
type store struct {
	pool    *pgxpool.Pool
	brandID string
}

// querier is a pool or a transaction.
type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// Version implements cache.VersionSource (profiles share the "pricing" scope).
func (s store) Version(ctx context.Context, scope string) (int64, error) {
	var v int64
	err := s.pool.QueryRow(ctx, `SELECT version FROM config_versions WHERE scope=$1`, scope).Scan(&v)
	return v, err
}

// bump makes every API instance reload pricing within seconds.
func bump(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `UPDATE config_versions SET version = version + 1, updated_at = now() WHERE scope='pricing'`)
	return err
}

const versionColumns = `v.id, v.profile_id, v.version, v.effective_from, v.commission_rate, v.booking_fee, v.waiting_cap_rate, v.fare_rounding,
	v.cancel_free_s, v.no_show_after_s, v.cancel_driver_share, v.vat_rate, v.vat_base, v.levy_rate, v.levy_borne_by,
	v.debt_soft_limit, v.debt_hard_limit, v.debt_soft_grace_days, v.payin_min_amount, v.payout_min_amount, v.payout_auto_approve,
	v.content_hash, v.published_by, v.reason, v.created_at`

func scanVersion(row pgx.Row) (Version, error) {
	var v Version
	t := &v.Terms
	err := row.Scan(&v.ID, &v.ProfileID, &v.Version, &v.EffectiveFrom, &t.CommissionRate, &t.BookingFee, &t.WaitingCapRate, &t.FareRounding,
		&t.CancelFreeS, &t.NoShowAfterS, &t.CancelDriverShare, &t.VATRate, &t.VATBase, &t.LevyRate, &t.LevyBorneBy,
		&t.DebtSoftLimit, &t.DebtHardLimit, &t.DebtGraceDays, &t.PayinMinAmount, &t.PayoutMinAmount, &t.PayoutAutoApprove,
		&v.ContentHash, &v.PublishedBy, &v.Reason, &v.CreatedAt)
	v.Charges = []ChargeRule{}
	return v, err
}

// versions reads versions (newest first) with their charges; where filters v.* (args after brand).
func (s store) versions(ctx context.Context, q querier, where string, args ...any) ([]Version, error) {
	rows, err := q.Query(ctx, `SELECT `+versionColumns+` FROM pricing_profile_versions v JOIN pricing_profiles p ON p.id=v.profile_id
		WHERE p.brand_id=$1 AND `+where+` ORDER BY v.profile_id, v.effective_from DESC, v.version DESC`, append([]any{s.brandID}, args...)...)
	if err != nil {
		return nil, err
	}
	var out []Version
	index := map[string]int{}
	for rows.Next() {
		v, err := scanVersion(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		index[v.ID] = len(out)
		out = append(out, v)
	}
	rows.Close()
	if err := rows.Err(); err != nil || len(out) == 0 {
		return out, err
	}
	return out, s.attachCharges(ctx, q, out, index)
}

func (s store) attachCharges(ctx context.Context, q querier, vs []Version, index map[string]int) error {
	ids := make([]string, 0, len(index))
	for id := range index {
		ids = append(ids, id)
	}
	rows, err := q.Query(ctx, `SELECT version_id, code, label, category, amount_type, amount, min_amount, max_amount, recipient,
		coalesce(payee_id::text,''), taxable, applies_when, services, vehicle_classes, on_fixed_routes, days,
		coalesce(lpad((from_minute/60)::text,2,'0')||':'||lpad((from_minute%60)::text,2,'0'),''),
		coalesce(lpad((to_minute/60)::text,2,'0')||':'||lpad((to_minute%60)::text,2,'0'),'')
		FROM pricing_profile_charges WHERE version_id = ANY($1) ORDER BY version_id, sort_order`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var vid string
		var c ChargeRule
		var days []int16
		if err := rows.Scan(&vid, &c.Code, &c.Label, &c.Category, &c.AmountType, &c.Amount, &c.MinAmount, &c.MaxAmount, &c.Recipient,
			&c.PayeeID, &c.Taxable, &c.AppliesWhen, &c.Services, &c.VehicleClasses, &c.OnFixedRoutes, &days, &c.FromTime, &c.ToTime); err != nil {
			return err
		}
		for _, d := range days {
			c.Days = append(c.Days, int(d))
		}
		v := &vs[index[vid]]
		v.Charges = append(v.Charges, c)
	}
	return rows.Err()
}

const profileColumns = `p.id, p.country_code, p.name, coalesce(p.description,''), p.status, p.active_from, p.active_until, p.updated_at`

func scanProfile(row pgx.Row) (Profile, error) {
	var p Profile
	err := row.Scan(&p.ID, &p.Country, &p.Name, &p.Description, &p.Status, &p.ActiveFrom, &p.ActiveUntil, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, ErrProfileNotFound
	}
	return p, err
}

func (s store) profiles(ctx context.Context, country string) ([]Profile, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+profileColumns+` FROM pricing_profiles p WHERE p.brand_id=$1 AND ($2='' OR p.country_code=$2) ORDER BY p.name`, s.brandID, country)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Profile{}
	for rows.Next() {
		p, err := scanProfile(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s store) profile(ctx context.Context, q querier, id string, lock bool) (Profile, error) {
	sql := `SELECT ` + profileColumns + ` FROM pricing_profiles p WHERE p.id=$1 AND p.brand_id=$2`
	if lock {
		sql += ` FOR UPDATE`
	}
	return scanProfile(q.QueryRow(ctx, sql, id, s.brandID))
}

const scopeColumns = `a.id, a.profile_id, a.scope_kind, a.scope_id,
	coalesce(CASE WHEN a.scope_kind='AREA' THEN (SELECT name FROM areas WHERE id::text=a.scope_id) ELSE (SELECT name->>'en' FROM countries c WHERE c.brand_id=a.brand_id AND c.code=a.scope_id) END, a.scope_id),
	a.priority, a.valid_from, a.valid_to, a.reason`

// assignments reads assignments (open first, newest first); where filters a.*.
func (s store) assignments(ctx context.Context, q querier, where string, args ...any) ([]Scope, error) {
	rows, err := q.Query(ctx, `SELECT `+scopeColumns+` FROM pricing_profile_assignments a WHERE a.brand_id=$1 AND `+where+`
		ORDER BY a.valid_to NULLS FIRST, a.valid_from DESC`, append([]any{s.brandID}, args...)...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[Scope])
}

func (s store) events(ctx context.Context, profileID string) ([]Event, error) {
	rows, err := s.pool.Query(ctx, `SELECT e.at, e.kind, e.version, e.assignment_id::text, e.actor_id::text, u.full_name, e.reason, e.data
		FROM pricing_profile_events e LEFT JOIN users u ON u.id=e.actor_id WHERE e.profile_id=$1 ORDER BY e.at DESC, e.id DESC LIMIT $2`, profileID, listEventsLimit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[Event])
}

// event appends to a profile's history.
func (s store) event(ctx context.Context, tx pgx.Tx, profileID, kind, actor, reason string, version *int, assignmentID *string, data map[string]any) error {
	raw, _ := json.Marshal(data)
	if data == nil {
		raw = []byte(`{}`)
	}
	_, err := tx.Exec(ctx, `INSERT INTO pricing_profile_events (brand_id, profile_id, kind, version, assignment_id, actor_id, reason, data)
		VALUES ($1,$2,$3,$4,$5::uuid,NULLIF($6,'')::uuid,$7,$8)`, s.brandID, profileID, kind, version, assignmentID, actor, reason, raw)
	return err
}

func (s store) payeeNames(ctx context.Context) (map[string]string, error) {
	rows, err := s.pool.Query(ctx, `SELECT id::text, name FROM charge_payees WHERE brand_id=$1`, s.brandID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		out[id] = name
	}
	return out, rows.Err()
}

// liveVersions keeps, per profile, the version in force at $2 and those due
// after it up to $3 (the snapshot and the list never need older ones).
const liveVersions = `(v.effective_from > $2 AND v.effective_from <= $3 OR v.id IN (
	SELECT DISTINCT ON (profile_id) id FROM pricing_profile_versions WHERE effective_from <= $2 ORDER BY profile_id, effective_from DESC, version DESC))`

// snapshotHorizon is how far ahead future-dated versions are kept in memory.
func snapshotHorizon(now time.Time) time.Time { return now.Add(EffectiveFromMaxAhead) }
