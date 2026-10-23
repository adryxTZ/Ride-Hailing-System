package branding

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// store reads and writes brands and brand_themes; no rules here.
type store struct {
	pool    *pgxpool.Pool
	brandID string
}

// querier is a pool or a transaction.
type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// Version implements cache.VersionSource for the 'branding' scope.
func (s store) Version(ctx context.Context, scope string) (int64, error) {
	var v int64
	err := s.pool.QueryRow(ctx, `SELECT version FROM config_versions WHERE scope=$1`, scope).Scan(&v)
	return v, err
}

// bump tells every API instance to reload branding (inside the writing transaction).
func (s store) bump(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `UPDATE config_versions SET version = version + 1, updated_at = now() WHERE scope='branding'`)
	return err
}

func (s store) identity(ctx context.Context) (Identity, error) {
	var i Identity
	var legal, phone, email, emergency *string
	err := s.pool.QueryRow(ctx, `SELECT display_name, legal_entity, support_phone, support_email, emergency_phone, identity_edited_at
		FROM brands WHERE id=$1`, s.brandID).Scan(&i.Name, &legal, &phone, &email, &emergency, &i.EditedAt)
	i.LegalEntity, i.SupportPhone, i.SupportEmail, i.EmergencyPhone = deref(legal), deref(phone), deref(email), deref(emergency)
	return i, err
}

func (s store) setIdentity(ctx context.Context, tx pgx.Tx, in IdentityInput, staffID string) error {
	_, err := tx.Exec(ctx, `UPDATE brands SET display_name=$2, legal_entity=NULLIF($3,''), support_phone=NULLIF($4,''),
		support_email=NULLIF($5,''), emergency_phone=NULLIF($6,''), identity_edited_at=now(), identity_edited_by=$7, updated_at=now()
		WHERE id=$1`, s.brandID, in.Name, in.LegalEntity, in.SupportPhone, in.SupportEmail, in.EmergencyPhone, staffID)
	return err
}

const themeColumns = `id, version, status::text, coalesce(name,''), source, roles, palette, dark, typography, shape, assets,
	coalesce(wcag_report,'{}'), change_reason, created_by, created_at, published_at, published_by`

func scanTheme(row pgx.Row) (*ThemeVersion, error) {
	var t ThemeVersion
	var roles, palette, dark, typo, shape, assets, report []byte
	err := row.Scan(&t.ID, &t.Version, &t.Status, &t.Name, &t.Source, &roles, &palette, &dark, &typo, &shape, &assets,
		&report, &t.Reason, &t.CreatedBy, &t.CreatedAt, &t.PublishedAt, &t.PublishedBy)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrThemeNotFound
	}
	if err != nil {
		return nil, err
	}
	for _, f := range []struct {
		raw []byte
		dst any
	}{{roles, &t.Roles}, {palette, &t.Palette}, {dark, &t.Dark}, {typo, &t.Typography}, {shape, &t.Shape}, {assets, &t.Assets}, {report, &t.Report}} {
		if len(f.raw) > 0 {
			if err := json.Unmarshal(f.raw, f.dst); err != nil {
				return nil, err
			}
		}
	}
	return &t, nil
}

func (s store) theme(ctx context.Context, q querier, id string, lock bool) (*ThemeVersion, error) {
	sql := `SELECT ` + themeColumns + ` FROM brand_themes WHERE id=$1 AND brand_id=$2`
	if lock {
		sql += ` FOR UPDATE`
	}
	return scanTheme(q.QueryRow(ctx, sql, id, s.brandID))
}

func (s store) published(ctx context.Context) (*ThemeVersion, error) {
	return scanTheme(s.pool.QueryRow(ctx, `SELECT `+themeColumns+` FROM brand_themes WHERE brand_id=$1 AND status='PUBLISHED'`, s.brandID))
}

func (s store) themes(ctx context.Context) ([]ThemeVersion, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+themeColumns+` FROM brand_themes WHERE brand_id=$1 ORDER BY version DESC`, s.brandID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ThemeVersion{}
	for rows.Next() {
		t, err := scanTheme(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *t)
	}
	return out, rows.Err()
}

// writeDraft inserts a new draft (id "") or replaces one, storing the derived tokens.
func (s store) writeDraft(ctx context.Context, tx pgx.Tx, id, staffID string, t *ThemeVersion) (string, error) {
	vals := []any{marshal(t.Roles), marshal(t.Palette), marshal(t.Dark), marshal(t.Typography), marshal(t.Shape), marshal(t.Assets), marshal(t.Report)}
	if id == "" {
		err := tx.QueryRow(ctx, `INSERT INTO brand_themes (brand_id, version, status, name, source, roles, palette, dark, typography, shape, assets, wcag_report, created_by)
			SELECT $1, coalesce(max(version),0)+1, 'DRAFT', $2, $3, $4, $5, $6, $7, $8, $9, $10, $11 FROM brand_themes WHERE brand_id=$1
			RETURNING id`, append(append([]any{s.brandID, t.Name, t.Source}, vals...), staffID)...).Scan(&id)
		return id, err
	}
	_, err := tx.Exec(ctx, `UPDATE brand_themes SET name=$3, source=$4, roles=$5, palette=$6, dark=$7, typography=$8, shape=$9, assets=$10, wcag_report=$11
		WHERE id=$1 AND brand_id=$2 AND status='DRAFT'`, append([]any{id, s.brandID, t.Name, t.Source}, vals...)...)
	return id, err
}

func (s store) deleteDraft(ctx context.Context, tx pgx.Tx, id string) (bool, error) {
	tag, err := tx.Exec(ctx, `DELETE FROM brand_themes WHERE id=$1 AND brand_id=$2 AND status='DRAFT'`, id, s.brandID)
	return tag.RowsAffected() > 0, err
}

// publish archives the live version and makes id live.
func (s store) publish(ctx context.Context, tx pgx.Tx, id, staffID, reason string) error {
	if _, err := tx.Exec(ctx, `UPDATE brand_themes SET status='ARCHIVED' WHERE brand_id=$1 AND status='PUBLISHED'`, s.brandID); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `UPDATE brand_themes SET status='PUBLISHED', published_at=now(), published_by=$3, change_reason=$4
		WHERE id=$1 AND brand_id=$2`, id, s.brandID, staffID, reason)
	return err
}

func marshal(v any) []byte {
	raw, _ := json.Marshal(v)
	return raw
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
