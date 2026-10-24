package legal

import (
	"context"
	"net/mail"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/kalen-limited/hauler-rider/backend/internal/audit"
)

// minimumAgeRange bounds {{minimum_age}}.
const (
	minimumAgeLow  = 13
	minimumAgeHigh = 21
)

// Details returns the Legal details form.
func (s *Service) Details(ctx context.Context) (Details, error) {
	return s.store.details(ctx, s.store.pool)
}

// SetDetails replaces the Legal details (audited). Only the catalogue's
// DETAILS keys are accepted; empty values are dropped.
func (s *Service) SetDetails(ctx context.Context, staffID string, in DetailsInput) (Details, error) {
	fields, bad := checkDetails(in.Fields)
	if len(bad) > 0 {
		return Details{}, ErrInvalid.WithDetails(map[string]any{"fields": bad})
	}
	reason := strings.TrimSpace(in.Reason)
	if reason == "" || len([]rune(reason)) > ReasonMaxLen {
		return Details{}, ErrReasonRequired
	}
	err := pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		if err := s.store.setDetails(ctx, tx, fields, staffID, s.now()); err != nil {
			return err
		}
		if err := audit.Record(ctx, tx, s.store.brandID, staffID, "legal.details", "brand:"+s.store.brandID, reason, fields); err != nil {
			return err
		}
		return s.store.bump(ctx, tx)
	})
	if err != nil {
		return Details{}, err
	}
	return s.Details(ctx)
}

// checkDetails trims the values and lists unknown keys and bad values.
func checkDetails(in map[string]string) (map[string]string, []field) {
	out := map[string]string{}
	var bad []field
	for k, v := range in {
		v = strings.TrimSpace(v)
		path := "fields." + k
		switch {
		case !editable(k):
			bad = append(bad, field{"path": path, "code": "NOT_ALLOWED"})
		case len([]rune(v)) > FieldMaxLen:
			bad = append(bad, field{"path": path, "code": "LENGTH"})
		case v != "" && !detailFormatOK(k, v):
			bad = append(bad, field{"path": path, "code": "FORMAT"})
		case v != "":
			out[k] = v
		}
	}
	return out, bad
}

// detailFormatOK checks the values that have a shape (an e-mail, an age).
func detailFormatOK(key, v string) bool {
	switch key {
	case "dpo_email":
		a, err := mail.ParseAddress(v)
		return err == nil && a.Address == v
	case "minimum_age":
		n, err := strconv.Atoi(v)
		return err == nil && n >= minimumAgeLow && n <= minimumAgeHigh
	}
	return true
}

// PlaceholderValue is a placeholder with the value it has now (for the editor's list).
type PlaceholderValue struct {
	Placeholder
	Value string `json:"value"`
}

// PlaceholderValues is the catalogue with today's values in a language.
func (s *Service) PlaceholderValues(ctx context.Context, lang string) ([]PlaceholderValue, error) {
	snap, _, err := s.cache.Get(ctx)
	if err != nil {
		return nil, err
	}
	vals, err := s.values(ctx, snap.details, lang, nil)
	if err != nil {
		return nil, err
	}
	out := make([]PlaceholderValue, 0, len(Placeholders))
	for _, p := range Placeholders {
		out = append(out, PlaceholderValue{Placeholder: p, Value: vals[p.Key]})
	}
	return out, nil
}
