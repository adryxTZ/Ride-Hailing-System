package profiles

import (
	"context"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/kalen-limited/hauler-rider/backend/internal/audit"
)

// builtinRecipients is who receives each charge kind when staff set nothing:
// money the driver pays out of pocket at a gate goes back to the driver.
var builtinRecipients = map[string]string{
	"TOLL": "DRIVER", "PARKING": "DRIVER", "AIRPORT_FEE": "DRIVER", "ENTRY_FEE": "DRIVER",
	"LEVY": "THIRD_PARTY", "NIGHT": "FARE", "OTHER": "FARE",
}

// ChargeDefault is who receives a charge kind by default in a country.
type ChargeDefault struct {
	Category  string  `json:"category"`
	Recipient string  `json:"recipient"`
	PayeeID   *string `json:"payee_id"`
	PayeeName *string `json:"payee_name"`
	Custom    bool    `json:"custom"` // false = the built-in default
}

// ChargeDefaults lists every charge kind with its default recipient in a country.
func (s *Service) ChargeDefaults(ctx context.Context, country string) ([]ChargeDefault, error) {
	set, err := s.store.chargeDefaults(ctx, strings.ToUpper(country))
	if err != nil {
		return nil, err
	}
	out := make([]ChargeDefault, 0, len(ChargeCategories))
	for _, cat := range ChargeCategories {
		if d, ok := set[cat]; ok {
			out = append(out, d)
			continue
		}
		out = append(out, ChargeDefault{Category: cat, Recipient: builtinRecipients[cat]})
	}
	return out, nil
}

// SetChargeDefault changes who receives a charge kind by default (audited).
// A THIRD_PARTY default names an active payee of the country.
func (s *Service) SetChargeDefault(ctx context.Context, staffID, country, category, recipient, payeeID, reason string) ([]ChargeDefault, error) {
	country = strings.ToUpper(country)
	if !slices.Contains(ChargeCategories, category) || !slices.Contains(Recipients, recipient) || (recipient == "THIRD_PARTY") != (payeeID != "") {
		return nil, ErrChargeInvalid.WithDetails(map[string]any{"fields": []map[string]string{{"path": "recipient", "code": "NOT_ALLOWED"}}})
	}
	err := s.write(ctx, country, reason, func(tx pgx.Tx) error {
		if err := s.checkPayee(ctx, tx, country, payeeID, 0); err != nil {
			return err
		}
		if err := s.store.setChargeDefault(ctx, tx, country, category, recipient, payeeID, staffID); err != nil {
			return err
		}
		return s.recordSetting(ctx, tx, staffID, "pricing.charge_default", country+":"+category, reason, map[string]any{"recipient": recipient, "payee_id": payeeID})
	})
	if err != nil {
		return nil, err
	}
	return s.ChargeDefaults(ctx, country)
}

// withDefaultRecipients fills a missing recipient from the charge kind's default.
func (s *Service) withDefaultRecipients(ctx context.Context, country string, charges []ChargeRule) ([]ChargeRule, error) {
	missing := false
	for _, c := range charges {
		missing = missing || c.Recipient == ""
	}
	if !missing {
		return charges, nil
	}
	defs, err := s.ChargeDefaults(ctx, country)
	if err != nil {
		return nil, err
	}
	byCat := map[string]ChargeDefault{}
	for _, d := range defs {
		byCat[d.Category] = d
	}
	out := slices.Clone(charges)
	for i, c := range out {
		if d, ok := byCat[c.Category]; ok && c.Recipient == "" {
			out[i].Recipient = d.Recipient
			if d.PayeeID != nil && c.PayeeID == "" {
				out[i].PayeeID = *d.PayeeID
			}
		}
	}
	return out, nil
}

// recordSetting audits a pricing setting change that belongs to no single profile.
func (s *Service) recordSetting(ctx context.Context, tx pgx.Tx, staffID, action, target, reason string, data any) error {
	return audit.Record(ctx, tx, s.store.brandID, staffID, action, "pricing_setting:"+target, reason, data)
}

func (s store) chargeDefaults(ctx context.Context, country string) (map[string]ChargeDefault, error) {
	rows, err := s.pool.Query(ctx, `SELECT d.category, d.recipient, d.payee_id::text, y.name FROM charge_category_defaults d
		LEFT JOIN charge_payees y ON y.id=d.payee_id WHERE d.brand_id=$1 AND d.country_code=$2`, s.brandID, country)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]ChargeDefault{}
	for rows.Next() {
		d := ChargeDefault{Custom: true}
		if err := rows.Scan(&d.Category, &d.Recipient, &d.PayeeID, &d.PayeeName); err != nil {
			return nil, err
		}
		out[d.Category] = d
	}
	return out, rows.Err()
}

func (s store) setChargeDefault(ctx context.Context, tx pgx.Tx, country, category, recipient, payeeID, staffID string) error {
	_, err := tx.Exec(ctx, `INSERT INTO charge_category_defaults (brand_id, country_code, category, recipient, payee_id, updated_by)
		VALUES ($1,$2,$3,$4,NULLIF($5,'')::uuid,NULLIF($6,'')::uuid)
		ON CONFLICT (brand_id, country_code, category) DO UPDATE SET recipient=EXCLUDED.recipient, payee_id=EXCLUDED.payee_id,
		  updated_by=EXCLUDED.updated_by, updated_at=now()`, s.brandID, country, category, recipient, payeeID, staffID)
	return err
}
