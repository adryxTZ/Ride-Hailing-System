// Package audit records staff decisions in the append-only audit_log table
// (migration 00017): who did what to which record, why, and with what data.
// Every admin write goes through Record in the same transaction as the change.
package audit

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgconn"
)

// Execer is a pool or a transaction.
type Execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// Record appends one entry. actor "" = system/operator command; reason may be empty.
func Record(ctx context.Context, db Execer, brandID, actor, action, target, reason string, data any) error {
	raw, _ := json.Marshal(data)
	if data == nil {
		raw = []byte(`{}`)
	}
	_, err := db.Exec(ctx, `
		INSERT INTO audit_log (brand_id, actor_id, action, target, reason, data)
		VALUES ($1, NULLIF($2,'')::uuid, $3, $4, NULLIF($5,''), $6)`, brandID, actor, action, target, reason, raw)
	return err
}
