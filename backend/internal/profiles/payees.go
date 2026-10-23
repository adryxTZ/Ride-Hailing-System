package profiles

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/kalen-limited/hauler-rider/backend/internal/audit"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
)

// PayeeInput creates a payee or changes one.
type PayeeInput struct {
	Country   string  `json:"country"`
	Name      *string `json:"name"`
	Kind      string  `json:"kind"`
	Reference *string `json:"reference"`
	Status    *string `json:"status"`
	Reason    string  `json:"reason"`
}

// referenceMaxLen matches the charge_payees CHECK.
const referenceMaxLen = 80

const payeeColumns = `id, country_code, name, kind, coalesce(reference,''), status`

// Payees lists a country's payees.
func (s *Service) Payees(ctx context.Context, country string) ([]Payee, error) {
	rows, err := s.store.pool.Query(ctx, `SELECT `+payeeColumns+` FROM charge_payees WHERE brand_id=$1 AND ($2='' OR country_code=$2) ORDER BY name`,
		s.store.brandID, strings.ToUpper(country))
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[Payee])
}

// CreatePayee adds a payee to a country (audited).
func (s *Service) CreatePayee(ctx context.Context, staffID string, in PayeeInput) (*Payee, error) {
	in.Country = strings.ToUpper(strings.TrimSpace(in.Country))
	name := strings.TrimSpace(deref(in.Name))
	if len(in.Country) != 2 || name == "" || len([]rune(name)) > NameMaxLen || len(deref(in.Reference)) > referenceMaxLen || !contains(PayeeKinds, in.Kind) {
		return nil, apperr.Validation.WithDetails(map[string]any{"fields": []map[string]string{{"path": "name", "code": "LENGTH_OR_KIND"}}})
	}
	var p Payee
	err := s.write(ctx, in.Country, in.Reason, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `INSERT INTO charge_payees (brand_id, country_code, name, kind, reference, created_by)
			VALUES ($1,$2,$3,$4,NULLIF($5,''),NULLIF($6,'')::uuid) RETURNING `+payeeColumns, s.store.brandID, in.Country, name, in.Kind, deref(in.Reference), staffID).
			Scan(&p.ID, &p.Country, &p.Name, &p.Kind, &p.Reference, &p.Status)
		if isUnique(err) {
			return ErrNameTaken
		}
		if err != nil {
			return err
		}
		return audit.Record(ctx, tx, s.store.brandID, staffID, "pricing.payee.create", "payee:"+p.ID, in.Reason, in)
	})
	return &p, err
}

// UpdatePayee renames a payee or switches it off (not while an in-force version
// charges for it). allowed is the caller's country check, run before anything is saved.
func (s *Service) UpdatePayee(ctx context.Context, staffID, id string, in PayeeInput, allowed func(country string) bool) (*Payee, error) {
	var p Payee
	err := s.store.pool.QueryRow(ctx, `SELECT `+payeeColumns+` FROM charge_payees WHERE id::text=$1 AND brand_id=$2`, id, s.store.brandID).
		Scan(&p.ID, &p.Country, &p.Name, &p.Kind, &p.Reference, &p.Status)
	if err == pgx.ErrNoRows {
		return nil, ErrPayeeNotFound
	}
	if err != nil {
		return nil, err
	}
	if allowed != nil && !allowed(p.Country) {
		return nil, ErrPayeeNotFound
	}
	err = s.write(ctx, p.Country, in.Reason, func(tx pgx.Tx) error {
		if in.Status != nil && *in.Status == StatusInactive && p.Status != StatusInactive {
			if err := s.payeeUnused(ctx, tx, id); err != nil {
				return err
			}
		}
		applyPayee(&p, in)
		if len([]rune(p.Name)) > NameMaxLen || len(p.Reference) > referenceMaxLen {
			return apperr.Validation.WithDetails(map[string]any{"fields": []map[string]string{{"path": "name", "code": "LENGTH"}}})
		}
		_, err := tx.Exec(ctx, `UPDATE charge_payees SET name=$2, reference=NULLIF($3,''), status=$4, updated_at=now() WHERE id=$1`, id, p.Name, p.Reference, p.Status)
		if isUnique(err) {
			return ErrNameTaken
		}
		if err != nil {
			return err
		}
		return audit.Record(ctx, tx, s.store.brandID, staffID, "pricing.payee.update", "payee:"+id, in.Reason, in)
	})
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func applyPayee(p *Payee, in PayeeInput) {
	if in.Name != nil && strings.TrimSpace(*in.Name) != "" {
		p.Name = strings.TrimSpace(*in.Name)
	}
	if in.Reference != nil {
		p.Reference = *in.Reference
	}
	if in.Status != nil && (*in.Status == StatusActive || *in.Status == StatusInactive) {
		p.Status = *in.Status
	}
}

// payeeUnused refuses switching off a payee that the latest version of an active profile charges for.
func (s *Service) payeeUnused(ctx context.Context, tx pgx.Tx, payeeID string) error {
	var used bool
	err := tx.QueryRow(ctx, `SELECT EXISTS (
		SELECT 1 FROM pricing_profile_charges c JOIN pricing_profile_versions v ON v.id=c.version_id JOIN pricing_profiles p ON p.id=v.profile_id
		WHERE c.payee_id::text=$1 AND p.status='ACTIVE'
		  AND v.version = (SELECT max(version) FROM pricing_profile_versions WHERE profile_id=p.id))`, payeeID).Scan(&used)
	if err == nil && used {
		return ErrPayeeInUse
	}
	return err
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
