package compliance

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
)

// trackedQuery selects the current documents (driver-owned, or owned by the
// driver's vehicle) of approved drivers whose country tracks that kind's expiry.
const trackedQuery = `SELECT d.id, p.user_id, u.phone, d.kind, p.country_code, t.label, d.expires_on, t.blocks_when_expired,
	  EXISTS (SELECT 1 FROM documents r WHERE r.renewal_of=d.id AND r.archived_at IS NULL AND r.status <> 'REJECTED')
	FROM documents d
	JOIN driver_profiles p ON p.user_id = CASE WHEN d.owner_kind='DRIVER' THEN d.owner_id
	  ELSE (SELECT v.driver_id FROM vehicles v WHERE v.id=d.owner_id) END
	JOIN users u ON u.id=p.user_id
	JOIN document_types t ON t.brand_id=d.brand_id AND t.country_code=p.country_code AND t.kind=d.kind
	WHERE d.brand_id=$1 AND d.archived_at IS NULL AND d.renewal_of IS NULL AND d.status='APPROVED'
	  AND p.status='APPROVED' AND t.tracks_expiry AND t.requirement <> 'OFF'`

func (s store) trackedDocuments(ctx context.Context) ([]trackedDoc, error) {
	return s.tracked(ctx, trackedQuery, s.brandID)
}

func (s store) trackedDocumentsOf(ctx context.Context, driverID string) ([]trackedDoc, error) {
	return s.tracked(ctx, trackedQuery+` AND p.user_id=$2`, s.brandID, driverID)
}

func (s store) tracked(ctx context.Context, sql string, args ...any) ([]trackedDoc, error) {
	rows, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[trackedDoc])
}

// documentHeldDrivers lists drivers on an active DOCUMENT hold.
func (s store) documentHeldDrivers(ctx context.Context) ([]string, error) {
	rows, err := s.pool.Query(ctx, `SELECT driver_id::text FROM driver_holds WHERE brand_id=$1 AND kind='DOCUMENT' AND released_at IS NULL`, s.brandID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

func (s store) remindersSent(ctx context.Context, docID string) ([]int, error) {
	rows, err := s.pool.Query(ctx, `SELECT threshold_days FROM document_reminders WHERE document_id=$1`, docID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[int])
}

// markReminder records a reminder before it is sent; false when it already was.
func (s store) markReminder(ctx context.Context, docID string, days int) (bool, error) {
	tag, err := s.pool.Exec(ctx, `INSERT INTO document_reminders (document_id, threshold_days) VALUES ($1,$2) ON CONFLICT DO NOTHING`, docID, days)
	return tag.RowsAffected() > 0, err
}

// labelEN is a document type's English label (the push text; apps translate later).
func labelEN(raw []byte) string {
	var l map[string]string
	_ = json.Unmarshal(raw, &l)
	if l["en"] != "" {
		return l["en"]
	}
	return "document"
}
