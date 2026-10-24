package approvals

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// configScope is this module's row in config_versions (bumped on workflow saves).
const configScope = "approvals"

type store struct {
	pool    *pgxpool.Pool
	brandID string
}

// querier is a pool or a transaction.
type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// Version is the workflow cache's version source.
func (s store) Version(ctx context.Context, scope string) (int64, error) {
	var v int64
	err := s.pool.QueryRow(ctx, `SELECT version FROM config_versions WHERE scope=$1`, scope).Scan(&v)
	return v, err
}

func (s store) bump(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `UPDATE config_versions SET version = version + 1, updated_at = now() WHERE scope=$1`, configScope)
	return err
}

// workflows loads every saved workflow of the brand, keyed by action.
func (s store) workflows(ctx context.Context) (map[string]Workflow, error) {
	rows, err := s.pool.Query(ctx, `SELECT action, enabled, definition, version, updated_by::text, updated_at
		FROM approval_workflows WHERE brand_id=$1`, s.brandID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]Workflow{}
	for rows.Next() {
		var w Workflow
		var raw []byte
		var at time.Time
		if err := rows.Scan(&w.Action, &w.Enabled, &raw, &w.Version, &w.UpdatedBy, &at); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, &w.Definition); err != nil {
			return nil, err
		}
		w.UpdatedAt = &at
		out[w.Action] = w
	}
	return out, rows.Err()
}

// saveWorkflow writes the next version of a workflow and its history row.
func (s store) saveWorkflow(ctx context.Context, tx pgx.Tx, action, staffID string, in WorkflowInput) (int, error) {
	raw, err := json.Marshal(in.Definition)
	if err != nil {
		return 0, err
	}
	var version int
	err = tx.QueryRow(ctx, `
		INSERT INTO approval_workflows (brand_id, action, enabled, definition, version, updated_by, updated_at)
		VALUES ($1, $2, $3, $4, 1, $5, now())
		ON CONFLICT (brand_id, action) DO UPDATE SET enabled=EXCLUDED.enabled, definition=EXCLUDED.definition,
		  version=approval_workflows.version+1, updated_by=EXCLUDED.updated_by, updated_at=now()
		RETURNING version`, s.brandID, action, in.Enabled, raw, staffID).Scan(&version)
	if err != nil {
		return 0, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO approval_workflow_versions (brand_id, action, version, enabled, definition, created_by, reason)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`, s.brandID, action, version, in.Enabled, raw, staffID, in.Reason)
	return version, err
}

// history is a workflow's saved versions, newest first, with who saved them.
func (s store) history(ctx context.Context, action string) ([]WorkflowVersion, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT v.version, v.enabled, v.definition, v.created_by::text, coalesce(u.full_name, u.phone), v.reason, v.created_at
		FROM approval_workflow_versions v LEFT JOIN users u ON u.id = v.created_by
		WHERE v.brand_id=$1 AND v.action=$2 ORDER BY v.version DESC`, s.brandID, action)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []WorkflowVersion{}
	for rows.Next() {
		var v WorkflowVersion
		var raw []byte
		if err := rows.Scan(&v.Version, &v.Enabled, &raw, &v.CreatedBy, &v.ByName, &v.Reason, &v.CreatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, &v.Definition); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// newRequest is a request about to be stored.
type newRequest struct {
	sub       Submission
	payload   []byte
	hash      string
	summary   Summary
	workflow  Workflow
	expiresAt time.Time
}

// insertRequest stores a request; the same pending change returns the existing id (created=false).
func (s store) insertRequest(ctx context.Context, tx pgx.Tx, n newRequest) (string, bool, error) {
	summary, _ := json.Marshal(n.summary)
	flow, _ := json.Marshal(n.workflow.Definition)
	var id string
	err := tx.QueryRow(ctx, `
		INSERT INTO approval_requests (brand_id, action, requester_id, reason, country_code, amount, target, summary,
		  payload, payload_hash, workflow, workflow_version, expires_at)
		VALUES ($1, $2, $3, $4, NULLIF($5,''), $6, $7, $8, $9, $10, $11, $12, $13)
		ON CONFLICT (brand_id, action, payload_hash) WHERE status = 'PENDING' DO NOTHING
		RETURNING id`, s.brandID, n.sub.Action, n.sub.RequesterID, n.sub.Reason, n.sub.Country, n.sub.Amount, n.sub.Target,
		summary, n.payload, n.hash, flow, n.workflow.Version, n.expiresAt).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		err = tx.QueryRow(ctx, `SELECT id FROM approval_requests WHERE brand_id=$1 AND action=$2 AND payload_hash=$3 AND status='PENDING'`,
			s.brandID, n.sub.Action, n.hash).Scan(&id)
		return id, false, err
	}
	return id, err == nil, err
}

const requestColumns = `r.id, r.action, r.status, r.requester_id::text, coalesce(u.full_name, u.phone, ''), r.reason,
	r.country_code, r.amount, r.target, r.summary, r.payload, r.workflow, r.workflow_version, r.current_step,
	r.expires_at, r.created_at, r.decided_at, r.executed_at, r.result, r.error`

const requestFrom = ` FROM approval_requests r LEFT JOIN users u ON u.id = r.requester_id`

func scanRequest(row pgx.Row) (*Request, error) {
	var r Request
	var summary, payload, flow, result []byte
	err := row.Scan(&r.ID, &r.Action, &r.Status, &r.Requester.ID, &r.Requester.Name, &r.Reason, &r.Country, &r.Amount,
		&r.Target, &summary, &payload, &flow, &r.WorkflowVersion, &r.CurrentStep, &r.ExpiresAt, &r.CreatedAt,
		&r.DecidedAt, &r.ExecutedAt, &result, &r.Error)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(summary, &r.Summary)
	_ = json.Unmarshal(flow, &r.Workflow)
	r.Payload, r.Result = payload, result
	r.Decisions = []Decision{}
	if r.Summary.Changes == nil {
		r.Summary.Changes = []Change{}
	}
	return &r, nil
}

// request reads one request (lock: FOR UPDATE inside a decision's transaction).
func (s store) request(ctx context.Context, q querier, id string, lock bool) (*Request, error) {
	sql := `SELECT ` + requestColumns + requestFrom + ` WHERE r.id::text=$1 AND r.brand_id=$2`
	if lock {
		sql += ` FOR UPDATE OF r`
	}
	r, err := scanRequest(q.QueryRow(ctx, sql, id, s.brandID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	ds, err := s.decisions(ctx, q, []string{r.ID})
	r.Decisions = ds[r.ID]
	if r.Decisions == nil {
		r.Decisions = []Decision{}
	}
	return r, err
}

// decisions loads the decisions of several requests in one query.
func (s store) decisions(ctx context.Context, q querier, ids []string) (map[string][]Decision, error) {
	out := map[string][]Decision{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := q.Query(ctx, `
		SELECT d.request_id::text, d.step, d.approver_id::text, coalesce(u.full_name, u.phone, ''), d.decision, d.as_role, d.note, d.created_at
		FROM approval_decisions d LEFT JOIN users u ON u.id = d.approver_id
		WHERE d.request_id = ANY($1::uuid[]) ORDER BY d.id`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var d Decision
		if err := rows.Scan(&id, &d.Step, &d.Approver.ID, &d.Approver.Name, &d.Decision, &d.AsRole, &d.Note, &d.At); err != nil {
			return nil, err
		}
		out[id] = append(out[id], d)
	}
	return out, rows.Err()
}

// listFilter selects an inbox box.
type listFilter struct {
	box       string
	viewerID  string
	countries []string // empty = all countries and country-less requests
	limit     int
}

// list returns one box's requests, newest first, with their decisions (two queries).
func (s store) list(ctx context.Context, f listFilter) ([]Request, error) {
	// A person always sees their own requests; otherwise only their countries
	// (a country-less request only when they are not limited).
	sql := `SELECT ` + requestColumns + requestFrom + ` WHERE r.brand_id=$1
		AND (r.requester_id::text = $3 OR $2::text[] IS NULL OR r.country_code = ANY($2::text[]))`
	switch f.box {
	case BoxMine:
		sql += ` AND r.requester_id::text = $3`
	case BoxDone:
		sql += ` AND r.status <> 'PENDING'`
	default:
		sql += ` AND r.status = 'PENDING'`
	}
	sql += ` ORDER BY r.created_at DESC LIMIT $4`
	var countries []string
	if len(f.countries) > 0 {
		countries = f.countries
	}
	rows, err := s.pool.Query(ctx, sql, s.brandID, countries, f.viewerID, f.limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Request{}
	var ids []string
	for rows.Next() {
		r, err := scanRequest(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *r)
		ids = append(ids, r.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	ds, err := s.decisions(ctx, s.pool, ids)
	for i := range out {
		if d := ds[out[i].ID]; d != nil {
			out[i].Decisions = d
		}
	}
	return out, err
}

// addDecision records one decision.
func (s store) addDecision(ctx context.Context, tx pgx.Tx, requestID string, step int, approverID, decision, asRole, note string) error {
	_, err := tx.Exec(ctx, `INSERT INTO approval_decisions (request_id, step, approver_id, decision, as_role, note)
		VALUES ($1, $2, $3, $4, $5, $6)`, requestID, step, approverID, decision, asRole, note)
	return err
}

// advance moves a pending request to its next step, or to a final status.
func (s store) advance(ctx context.Context, tx pgx.Tx, id string, step int, status string) error {
	_, err := tx.Exec(ctx, `UPDATE approval_requests SET current_step=$2, status=$3,
		decided_at = CASE WHEN $3 <> 'PENDING' THEN now() ELSE decided_at END WHERE id=$1`, id, step, status)
	return err
}

// claim moves an APPROVED request to EXECUTING; false = someone else has it (runs once).
func (s store) claim(ctx context.Context, id string) (bool, error) {
	tag, err := s.pool.Exec(ctx, `UPDATE approval_requests SET status='EXECUTING' WHERE id=$1 AND status='APPROVED'`, id)
	return tag.RowsAffected() == 1, err
}

// finish stores an execution's outcome.
func (s store) finish(ctx context.Context, tx pgx.Tx, id, status string, result []byte, errText string) error {
	_, err := tx.Exec(ctx, `UPDATE approval_requests SET status=$2, result=$3, error=NULLIF($4,''), executed_at=now()
		WHERE id=$1 AND status='EXECUTING'`, id, status, result, errText)
	return err
}

// expireDue marks pending requests past their time as expired and returns them.
func (s store) expireDue(ctx context.Context, tx pgx.Tx, now time.Time) ([]string, error) {
	rows, err := tx.Query(ctx, `UPDATE approval_requests SET status='EXPIRED', decided_at=$2
		WHERE brand_id=$1 AND status='PENDING' AND expires_at <= $2 RETURNING id::text`, s.brandID, now)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}
