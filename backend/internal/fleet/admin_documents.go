package fleet

import (
	"context"
	"strings"
	"time"
)

// Admin document list filters.
const (
	DocsExpiring       = "EXPIRING"
	DocsExpired        = "EXPIRED"
	DocsUnknown        = "UNKNOWN"
	DocsRenewalPending = "RENEWAL_PENDING"
	DocsMissing        = "MISSING" // mandatory types an approved driver has not provided (e.g. made mandatory later)
	adminDocsLimit     = 500
)

// AdminDocument is one row of the staff documents page.
type AdminDocument struct {
	DriverID    string            `json:"driver_id"`
	DriverName  *string           `json:"driver_name,omitempty"`
	Phone       string            `json:"phone"`
	Plate       *string           `json:"plate,omitempty"`
	Kind        string            `json:"kind"`
	Label       map[string]string `json:"label"`
	DocumentID  *string           `json:"document_id,omitempty"`
	ExpiresOn   *string           `json:"expires_on,omitempty"`
	ExpiryState string            `json:"expiry_state"`
	DaysLeft    *int              `json:"days_left,omitempty"`
	RenewalID   *string           `json:"renewal_id,omitempty"`
	Blocks      bool              `json:"blocks_when_expired"`
	Mandatory   bool              `json:"mandatory"`
}

// AdminDocuments lists approved drivers' documents of a country in one state.
func (s *Service) AdminDocuments(ctx context.Context, country, state string) ([]AdminDocument, error) {
	country, state = strings.ToUpper(country), strings.ToUpper(state)
	rows, err := s.store.adminDocuments(ctx, country)
	if err != nil {
		return nil, err
	}
	today := s.localToday(country)
	out := []AdminDocument{}
	for _, r := range rows {
		r.ExpiryState, r.DaysLeft = s.adminState(r, today)
		if matches(r, state) {
			out = append(out, r)
		}
	}
	return out, nil
}

func (s *Service) adminState(r AdminDocument, today time.Time) (string, *int) {
	if r.DocumentID == nil {
		return DocsMissing, nil
	}
	return ExpiryState(true, r.ExpiresOn, today)
}

func matches(r AdminDocument, state string) bool {
	switch state {
	case DocsRenewalPending:
		return r.RenewalID != nil
	case DocsMissing:
		return r.ExpiryState == DocsMissing && r.Mandatory
	case DocsExpiring, DocsExpired, DocsUnknown:
		return r.ExpiryState == state
	}
	return r.ExpiryState != DocsMissing || r.Mandatory
}

// adminDocuments reads, for every approved driver of a country, each type the
// country asks for that tracks expiry (or is mandatory), with the current copy and any pending renewal.
func (s store) adminDocuments(ctx context.Context, country string) ([]AdminDocument, error) {
	rows, err := s.pool.Query(ctx, `SELECT p.user_id, u.full_name, u.phone, v.plate, t.kind, t.label, d.id, d.expires_on::text, r.id,
		  t.blocks_when_expired, t.requirement='MANDATORY'
		FROM driver_profiles p JOIN users u ON u.id=p.user_id
		LEFT JOIN LATERAL (SELECT id, plate FROM vehicles WHERE driver_id=p.user_id ORDER BY created_at DESC LIMIT 1) v ON true
		JOIN document_types t ON t.brand_id=p.brand_id AND t.country_code=p.country_code AND t.requirement <> 'OFF'
		  AND (t.tracks_expiry OR t.requirement='MANDATORY')
		LEFT JOIN documents d ON d.kind=t.kind AND d.archived_at IS NULL AND d.renewal_of IS NULL AND d.status <> 'REJECTED'
		  AND d.owner_id = CASE WHEN t.owner='DRIVER' THEN p.user_id ELSE v.id END
		LEFT JOIN documents r ON r.renewal_of=d.id AND r.archived_at IS NULL AND r.status='PENDING'
		WHERE p.brand_id=$1 AND p.country_code=$2 AND p.status='APPROVED'
		ORDER BY d.expires_on NULLS FIRST LIMIT $3`, s.brandID, country, adminDocsLimit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AdminDocument{}
	for rows.Next() {
		var a AdminDocument
		if err := rows.Scan(&a.DriverID, &a.DriverName, &a.Phone, &a.Plate, &a.Kind, &a.Label, &a.DocumentID, &a.ExpiresOn, &a.RenewalID, &a.Blocks, &a.Mandatory); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
