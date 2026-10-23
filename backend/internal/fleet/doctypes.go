package fleet

import (
	"context"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
)

// Document requirement levels, chosen by staff per country.
const (
	RequireMandatory = "MANDATORY" // blocks sign-up until uploaded
	RequireOptional  = "OPTIONAL"  // asked for, never blocks
	RequireOff       = "OFF"       // not asked for at all
)

// docTypesTTL is how long a country's document types are kept in memory (staff edits clear it).
const docTypesTTL = 30 * time.Second

var kindPattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]{1,31}$`)

// DocumentType is one document a country asks drivers for, and how it is treated.
type DocumentType struct {
	Country           string            `json:"country"`
	Kind              string            `json:"kind"`
	Owner             string            `json:"owner"` // DRIVER or VEHICLE
	Label             map[string]string `json:"label"`
	Requirement       string            `json:"requirement"`
	TracksExpiry      bool              `json:"tracks_expiry"`
	ExpiryRequired    bool              `json:"expiry_required"`
	BlocksWhenExpired bool              `json:"blocks_when_expired"`
	SortOrder         int               `json:"sort_order"`
}

// ErrDocumentTypeInvalid: a staff edit that breaks the rules (bad kind code, missing label, expiry flags without tracking).
var ErrDocumentTypeInvalid = apperr.New("DOCUMENT_TYPE_INVALID", 400, "")

// docTypeCache keeps each country's types for docTypesTTL.
type docTypeCache struct {
	mu   sync.Mutex
	byCC map[string]cachedTypes
}

type cachedTypes struct {
	types []DocumentType
	at    time.Time
}

// DocumentTypes returns a country's document types in display order (all levels, OFF included).
func (s *Service) DocumentTypes(ctx context.Context, country string) ([]DocumentType, error) {
	s.docTypes.mu.Lock()
	c, ok := s.docTypes.byCC[country]
	s.docTypes.mu.Unlock()
	if ok && s.now().Sub(c.at) < docTypesTTL {
		return c.types, nil
	}
	types, err := s.store.documentTypes(ctx, country)
	if err != nil {
		return nil, err
	}
	s.docTypes.mu.Lock()
	if s.docTypes.byCC == nil {
		s.docTypes.byCC = map[string]cachedTypes{}
	}
	s.docTypes.byCC[country] = cachedTypes{types: types, at: s.now()}
	s.docTypes.mu.Unlock()
	return types, nil
}

// SaveDocumentType creates or changes a country's document type, audited.
// Making a type mandatory never suspends drivers already approved: staff see
// who lacks it on the documents page.
func (s *Service) SaveDocumentType(ctx context.Context, staffID string, t DocumentType, reason string) ([]DocumentType, error) {
	if strings.TrimSpace(reason) == "" {
		return nil, ErrReasonRequired
	}
	if err := validateDocType(&t); err != nil {
		return nil, err
	}
	err := pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		if err := s.store.upsertDocumentType(ctx, tx, t, staffID); err != nil {
			return err
		}
		return s.store.audit(ctx, tx, staffID, "document_type.save", "document_type:"+t.Country+"/"+t.Kind, &reason, map[string]string{
			"requirement": t.Requirement, "owner": t.Owner})
	})
	if err != nil {
		return nil, err
	}
	s.docTypes.mu.Lock()
	delete(s.docTypes.byCC, t.Country)
	s.docTypes.mu.Unlock()
	return s.DocumentTypes(ctx, t.Country)
}

func validateDocType(t *DocumentType) error {
	t.Kind, t.Country = strings.ToUpper(strings.TrimSpace(t.Kind)), strings.ToUpper(t.Country)
	bad := func(field string) error {
		return ErrDocumentTypeInvalid.WithDetails(map[string]any{"fields": []map[string]string{{"path": field, "code": "INVALID"}}})
	}
	switch {
	case !kindPattern.MatchString(t.Kind):
		return bad("kind")
	case t.Owner != OwnerDriver && t.Owner != OwnerVehicle:
		return bad("owner")
	case t.Requirement != RequireMandatory && t.Requirement != RequireOptional && t.Requirement != RequireOff:
		return bad("requirement")
	case strings.TrimSpace(t.Label["en"]) == "":
		return bad("label.en")
	case !t.TracksExpiry && (t.ExpiryRequired || t.BlocksWhenExpired):
		return bad("tracks_expiry")
	}
	return nil
}

// typeOf finds a kind among a country's types (nil when unknown).
func typeOf(types []DocumentType, kind string) *DocumentType {
	for i := range types {
		if types[i].Kind == kind {
			return &types[i]
		}
	}
	return nil
}
