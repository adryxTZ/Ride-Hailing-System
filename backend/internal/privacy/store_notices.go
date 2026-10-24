package privacy

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/brand"
)

const noticeColumns = `app, version, title, summary, url, links, published_at`

func scanNotice(row pgx.Row) (*Notice, error) {
	var n Notice
	var title, summary, url, links []byte
	if err := row.Scan(&n.App, &n.Version, &title, &summary, &url, &links, &n.PublishedAt); err != nil {
		return nil, err
	}
	_ = json.Unmarshal(title, &n.Title)
	_ = json.Unmarshal(summary, &n.Summary)
	_ = json.Unmarshal(url, &n.URL)
	_ = json.Unmarshal(links, &n.Links)
	return &n, nil
}

// Version is the "legal" scope's config version (the notices cache's invalidation).
func (s store) Version(ctx context.Context, scope string) (int64, error) {
	var v int64
	err := s.pool.QueryRow(ctx, `SELECT version FROM config_versions WHERE scope=$1`, scope).Scan(&v)
	return v, err
}

// notices loads every app's consent versions, oldest first.
func (s store) notices(ctx context.Context) (map[string][]Notice, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+noticeColumns+` FROM consent_documents WHERE brand_id=$1 ORDER BY app, version`, s.brandID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]Notice{}
	for rows.Next() {
		n, err := scanNotice(rows)
		if err != nil {
			return nil, err
		}
		out[n.App] = append(out[n.App], *n)
	}
	return out, rows.Err()
}

// lockLatest returns an app's newest consent version (nil: none) and holds the
// app's consent lock until the transaction ends, so two publishes number apart.
func (s store) lockLatest(ctx context.Context, tx pgx.Tx, app string) (*Notice, error) {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, "consent:"+s.brandID+":"+app); err != nil {
		return nil, err
	}
	n, err := scanNotice(tx.QueryRow(ctx, `SELECT `+noticeColumns+` FROM consent_documents WHERE brand_id=$1 AND app=$2 ORDER BY version DESC LIMIT 1`,
		s.brandID, app))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return n, err
}

func (s store) insertNotice(ctx context.Context, tx pgx.Tx, version int, n NewNotice) error {
	_, err := tx.Exec(ctx, `INSERT INTO consent_documents (brand_id, app, version, title, summary, url, links, documents, published_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,now())`,
		s.brandID, n.App, version, jsonOf(n.Title), jsonOf(n.Summary), jsonOf(n.URL), jsonOf(linksOf(n.Links)), jsonOf(n.Documents))
	return err
}

// linkNotice points an existing version at the documents (only while it has none).
func (s store) linkNotice(ctx context.Context, tx pgx.Tx, version int, n NewNotice) error {
	_, err := tx.Exec(ctx, `UPDATE consent_documents SET url=$4, links=$5, documents=$6
		WHERE brand_id=$1 AND app=$2 AND version=$3 AND documents='{}'::jsonb`,
		s.brandID, n.App, version, jsonOf(n.URL), jsonOf(linksOf(n.Links)), jsonOf(n.Documents))
	return err
}

// insertBrandNotice writes a brand-file notice (never overwriting a stored one).
func (s store) insertBrandNotice(ctx context.Context, d brand.ConsentDoc) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO consent_documents (brand_id, app, version, title, summary, url, published_at) VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (brand_id, app, version) DO NOTHING`,
		s.brandID, d.App, d.Version, jsonOf(d.Title), jsonOf(d.Summary), jsonOf(d.URL), d.PublishedAt)
	return err
}

// jsonOf encodes a column value; a nil map is stored as {} (never JSON null).
func jsonOf(v any) []byte {
	raw, _ := json.Marshal(v)
	if string(raw) == "null" {
		return []byte(`{}`)
	}
	return raw
}

// linksOf is never nil, so the column holds [] rather than {} or null.
func linksOf(l []NoticeLink) []NoticeLink {
	if l == nil {
		return []NoticeLink{}
	}
	return l
}
