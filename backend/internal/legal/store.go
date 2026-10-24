package legal

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
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

const docColumns = `id, app, kind, version, status, title, summary, body, source, change_reason, created_by, created_at, updated_at, published_by, published_at`

func scanDoc(row pgx.Row) (*Document, error) {
	var d Document
	var title, summary, body []byte
	err := row.Scan(&d.ID, &d.App, &d.Kind, &d.Version, &d.Status, &title, &summary, &body, &d.Source, &d.Reason,
		&d.CreatedBy, &d.CreatedAt, &d.UpdatedAt, &d.PublishedBy, &d.PublishedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	d.Title, d.Summary, d.Body = Text{}, Text{}, Text{}
	_ = json.Unmarshal(title, &d.Title)
	_ = json.Unmarshal(summary, &d.Summary)
	_ = json.Unmarshal(body, &d.Body)
	return &d, nil
}

func scanDocs(rows pgx.Rows, err error) ([]Document, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Document{}
	for rows.Next() {
		d, err := scanDoc(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *d)
	}
	return out, rows.Err()
}

// Version is the config version of the "legal" scope (the cache's invalidation).
func (s store) Version(ctx context.Context, scope string) (int64, error) {
	var v int64
	err := s.pool.QueryRow(ctx, `SELECT version FROM config_versions WHERE scope=$1`, scope).Scan(&v)
	return v, err
}

// bump tells every API instance to reload the legal texts and consent versions.
func (s store) bump(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `UPDATE config_versions SET version = version + 1, updated_at = now() WHERE scope='legal'`)
	return err
}

// lock serialises writes to one document (or to the starter pack) across instances.
func (s store) lock(ctx context.Context, tx pgx.Tx, key string) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, "legal:"+s.brandID+":"+key)
	return err
}

// live returns every published version (the public pages' cache).
func (s store) live(ctx context.Context, q querier) ([]Document, error) {
	return scanDocs(q.Query(ctx, `SELECT `+docColumns+` FROM legal_documents WHERE brand_id=$1 AND status='PUBLISHED'`, s.brandID))
}

// current returns the live versions and drafts (the overview), without bodies.
func (s store) current(ctx context.Context) ([]Document, error) {
	return scanDocs(s.pool.Query(ctx, `SELECT id, app, kind, version, status, title, '{}'::jsonb, '{}'::jsonb, source, change_reason, created_by,
		created_at, updated_at, published_by, published_at FROM legal_documents WHERE brand_id=$1 AND status IN ('PUBLISHED','DRAFT')`, s.brandID))
}

// versions lists one document's versions, newest first.
func (s store) versions(ctx context.Context, app, kind string) ([]Document, error) {
	return scanDocs(s.pool.Query(ctx, `SELECT `+docColumns+` FROM legal_documents WHERE brand_id=$1 AND app=$2 AND kind=$3 ORDER BY version DESC`,
		s.brandID, app, kind))
}

// draft returns the document's draft, locked for the transaction.
func (s store) draft(ctx context.Context, tx pgx.Tx, app, kind string) (*Document, error) {
	return scanDoc(tx.QueryRow(ctx, `SELECT `+docColumns+` FROM legal_documents WHERE brand_id=$1 AND app=$2 AND kind=$3 AND status='DRAFT' FOR UPDATE`,
		s.brandID, app, kind))
}

// insertDraft creates the next version as a draft.
func (s store) insertDraft(ctx context.Context, tx pgx.Tx, app, kind, staffID string, in DraftInput) (string, error) {
	var id string
	err := tx.QueryRow(ctx, `INSERT INTO legal_documents (brand_id, app, kind, version, title, summary, body, created_by)
		SELECT $1, $2, $3, coalesce(max(version), 0) + 1, $4, $5, $6, NULLIF($7,'')::uuid FROM legal_documents WHERE brand_id=$1 AND app=$2 AND kind=$3
		RETURNING id`, s.brandID, app, kind, jsonOf(in.Title), jsonOf(in.Summary), jsonOf(in.Body), staffID).Scan(&id)
	return id, err
}

func (s store) updateDraft(ctx context.Context, tx pgx.Tx, id string, in DraftInput) error {
	_, err := tx.Exec(ctx, `UPDATE legal_documents SET title=$2, summary=$3, body=$4, updated_at=now() WHERE id=$1 AND status='DRAFT'`,
		id, jsonOf(in.Title), jsonOf(in.Summary), jsonOf(in.Body))
	return err
}

func (s store) deleteDraft(ctx context.Context, tx pgx.Tx, id string) error {
	_, err := tx.Exec(ctx, `DELETE FROM legal_documents WHERE id=$1 AND status='DRAFT'`, id)
	return err
}

// publish archives the live version and makes the draft live.
func (s store) publish(ctx context.Context, tx pgx.Tx, d *Document, staffID, reason string) error {
	if _, err := tx.Exec(ctx, `UPDATE legal_documents SET status='ARCHIVED', updated_at=now() WHERE brand_id=$1 AND app=$2 AND kind=$3 AND status='PUBLISHED'`,
		s.brandID, d.App, d.Kind); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `UPDATE legal_documents SET status='PUBLISHED', published_at=now(), published_by=NULLIF($2,'')::uuid, change_reason=$3, updated_at=now()
		WHERE id=$1 AND status='DRAFT'`, d.ID, staffID, reason)
	return err
}

// insertStarter adds a starter-pack text as published version 1.
func (s store) insertStarter(ctx context.Context, tx pgx.Tx, d Document) (string, error) {
	var id string
	err := tx.QueryRow(ctx, `INSERT INTO legal_documents (brand_id, app, kind, version, status, title, summary, body, source, change_reason, published_at)
		VALUES ($1,$2,$3,1,'PUBLISHED',$4,$5,$6,'STARTER',$7,now()) RETURNING id`,
		s.brandID, d.App, d.Kind, jsonOf(d.Title), jsonOf(d.Summary), jsonOf(d.Body), starterReason).Scan(&id)
	return id, err
}

// count is how many versions the brand has (0 = the starter pack is due).
func (s store) count(ctx context.Context, tx pgx.Tx) (int, error) {
	var n int
	err := tx.QueryRow(ctx, `SELECT count(*) FROM legal_documents WHERE brand_id=$1`, s.brandID).Scan(&n)
	return n, err
}

// details returns the Legal details form (empty when never saved).
func (s store) details(ctx context.Context, q querier) (Details, error) {
	d := Details{Fields: map[string]string{}}
	var raw []byte
	err := q.QueryRow(ctx, `SELECT fields, updated_by::text, updated_at FROM legal_details WHERE brand_id=$1`, s.brandID).Scan(&raw, &d.UpdatedBy, &d.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return d, nil
	}
	if err == nil {
		_ = json.Unmarshal(raw, &d.Fields)
	}
	return d, err
}

func (s store) setDetails(ctx context.Context, tx pgx.Tx, fields map[string]string, staffID string, at time.Time) error {
	_, err := tx.Exec(ctx, `INSERT INTO legal_details (brand_id, fields, updated_by, updated_at) VALUES ($1,$2,NULLIF($3,'')::uuid,$4)
		ON CONFLICT (brand_id) DO UPDATE SET fields=EXCLUDED.fields, updated_by=EXCLUDED.updated_by, updated_at=EXCLUDED.updated_at`,
		s.brandID, jsonOf(fields), staffID, at)
	return err
}

func jsonOf(v any) []byte {
	raw, _ := json.Marshal(v)
	return raw
}
