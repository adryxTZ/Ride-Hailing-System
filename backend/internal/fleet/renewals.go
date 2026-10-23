package fleet

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
)

// ErrNotApprovedDriver: renewals are for approved drivers (others use onboarding).
var ErrNotApprovedDriver = apperr.New("DRIVER_NOT_APPROVED", 409, "")

// DriverDocuments is the approved driver's documents page: every type the
// country asks for, the current copy with its expiry state, and any renewal.
type DriverDocuments struct {
	Requirements []Requirement `json:"requirements"`
	Documents    []Document    `json:"documents"`
}

// Documents returns a driver's documents with expiry states.
func (s *Service) Documents(ctx context.Context, userID string) (*DriverDocuments, error) {
	ob, err := s.assemble(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &DriverDocuments{Requirements: ob.Requirements, Documents: ob.Documents}, nil
}

// Renew uploads a new copy of a document for an approved driver. It waits for
// review beside the current one (which stays valid until then); a second upload
// replaces a pending renewal. A kind the driver never had becomes current at once,
// pending review (e.g. a document that staff just made mandatory).
func (s *Service) Renew(ctx context.Context, userID string, in DocumentInput) (*DriverDocuments, error) {
	p, err := s.store.profile(ctx, userID)
	if err != nil {
		return nil, err
	}
	if p == nil || p.Status != StatusApproved {
		return nil, ErrNotApprovedDriver
	}
	t, err := s.uploadType(ctx, userID, &in)
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(in.FileKey, "documents/"+userID+"/") {
		return nil, ErrUploadMissing
	}
	size, err := s.files.Size(ctx, in.FileKey)
	if err != nil {
		return nil, ErrUploadMissing
	}
	in.sizeBytes = size
	ownerID, err := s.ownerID(ctx, userID, t.Owner)
	if err != nil {
		return nil, err
	}
	if err := s.store.putRenewal(ctx, t.Owner, ownerID, in); err != nil {
		return nil, err
	}
	return s.Documents(ctx, userID)
}

// ownerID is the driver, or their current vehicle for vehicle documents.
func (s *Service) ownerID(ctx context.Context, userID, owner string) (string, error) {
	if owner != OwnerVehicle {
		return userID, nil
	}
	v, err := s.store.vehicleOf(ctx, userID)
	if err != nil {
		return "", err
	}
	if v == nil {
		return "", fieldError("vehicle", "REQUIRED")
	}
	return v.ID, nil
}

// promoteRenewalTx makes an approved renewal the current document and archives the old one.
func (s store) promoteRenewalTx(ctx context.Context, tx pgx.Tx, docID string) error {
	var old *string
	err := tx.QueryRow(ctx, `SELECT renewal_of FROM documents WHERE id=$1 AND brand_id=$2`, docID, s.brandID).Scan(&old)
	if errors.Is(err, pgx.ErrNoRows) || old == nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE documents SET archived_at=now() WHERE id=$1`, *old); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE documents SET renewal_of=NULL WHERE id=$1`, docID)
	return err
}

// putRenewal stores a renewal beside the current document, or a first copy as current.
func (s store) putRenewal(ctx context.Context, owner, ownerID string, in DocumentInput) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var current string
		err := tx.QueryRow(ctx, `SELECT id FROM documents WHERE owner_kind=$1::document_owner AND owner_id=$2 AND kind=$3
			AND archived_at IS NULL AND renewal_of IS NULL FOR UPDATE`, owner, ownerID, in.Kind).Scan(&current)
		if errors.Is(err, pgx.ErrNoRows) {
			_, err = tx.Exec(ctx, `INSERT INTO documents (brand_id, owner_kind, owner_id, kind, file_key, content_type, expires_on, size_bytes)
				VALUES ($1,$2::document_owner,$3,$4,$5,$6,NULLIF($7,'')::date,$8)`, s.brandID, owner, ownerID, in.Kind, in.FileKey, in.ContentType, deref(in.ExpiresOn), in.sizeBytes)
			return err
		}
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO documents (brand_id, owner_kind, owner_id, kind, file_key, content_type, expires_on, renewal_of, size_bytes)
			VALUES ($1,$2::document_owner,$3,$4,$5,$6,NULLIF($7,'')::date,$8,$9)
			ON CONFLICT (owner_kind, owner_id, kind) WHERE archived_at IS NULL AND renewal_of IS NOT NULL DO UPDATE SET file_key=EXCLUDED.file_key,
			  content_type=EXCLUDED.content_type, size_bytes=EXCLUDED.size_bytes, expires_on=EXCLUDED.expires_on, status='PENDING', status_reason=NULL, reviewed_by=NULL, reviewed_at=NULL, created_at=now()`,
			s.brandID, owner, ownerID, in.Kind, in.FileKey, in.ContentType, deref(in.ExpiresOn), current, in.sizeBytes)
		return err
	})
}

// SetExpiry lets staff correct a document's expiry date (audited); holds are re-evaluated.
func (s *Service) SetExpiry(ctx context.Context, staffID, driverID, docID, expiresOn, reason string) (*DriverDetail, error) {
	if strings.TrimSpace(reason) == "" {
		return nil, ErrReasonRequired
	}
	if _, err := time.Parse(dateOnly, expiresOn); err != nil { // staff may set a past date: the document is then expired
		return nil, fieldError("expires_on", "FORMAT")
	}
	err := pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE documents SET expires_on=$2::date WHERE id=$1 AND brand_id=$3
			AND (owner_id=$4 OR owner_id IN (SELECT id FROM vehicles WHERE driver_id=$4))`, docID, expiresOn, s.store.brandID, driverID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrDriverNotFound
		}
		return s.store.audit(ctx, tx, staffID, "document.expiry", "driver:"+driverID, &reason, map[string]string{"document": docID, "expires_on": expiresOn})
	})
	if err != nil {
		return nil, err
	}
	s.documentsChanged(ctx, driverID)
	return s.Detail(ctx, driverID)
}

// documentsChanged tells the expiry rules a driver's documents moved.
func (s *Service) documentsChanged(ctx context.Context, driverID string) {
	if s.onDocuments != nil {
		s.onDocuments(ctx, driverID)
	}
}
