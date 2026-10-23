package profile

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
)

type Service struct{ store store }

func New(pool *pgxpool.Pool) *Service { return &Service{store: store{pool: pool}} }

func (s *Service) Get(ctx context.Context, userID string) (*Profile, error) {
	p, err := s.store.get(ctx, userID)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, apperr.NotFound
	}
	return p, nil
}

// Update validates and applies the editable fields, then returns the fresh profile.
func (s *Service) Update(ctx context.Context, userID string, u ProfileUpdate) (*Profile, error) {
	var fields []map[string]string
	if u.FullName != nil {
		n := strings.TrimSpace(*u.FullName)
		if len(n) < 2 || len(n) > 80 {
			fields = append(fields, map[string]string{"path": "full_name", "code": "LENGTH"})
		}
		u.FullName = &n
	}
	if u.Email != nil {
		e := strings.ToLower(strings.TrimSpace(*u.Email))
		if !strings.Contains(e, "@") || len(e) > 120 {
			fields = append(fields, map[string]string{"path": "email", "code": "FORMAT"})
		}
		u.Email = &e
	}
	if u.Locale != nil && *u.Locale != "en" && *u.Locale != "sw" {
		fields = append(fields, map[string]string{"path": "locale", "code": "UNSUPPORTED"})
	}
	if len(fields) > 0 {
		return nil, apperr.Validation.WithDetails(map[string]any{"fields": fields})
	}
	if err := s.store.update(ctx, userID, u); err != nil {
		return nil, err
	}
	return s.Get(ctx, userID)
}

func (s *Service) ListPlaces(ctx context.Context, userID string) ([]Place, error) {
	return s.store.listPlaces(ctx, userID)
}

func (s *Service) SavePlace(ctx context.Context, userID string, in PlaceInput) (*Place, error) {
	var fields []map[string]string
	switch in.Kind {
	case "HOME", "WORK", "OTHER":
	default:
		fields = append(fields, map[string]string{"path": "kind", "code": "INVALID"})
	}
	if strings.TrimSpace(in.Label) == "" && in.Kind == "OTHER" {
		fields = append(fields, map[string]string{"path": "label", "code": "REQUIRED"})
	}
	if strings.TrimSpace(in.Address) == "" {
		fields = append(fields, map[string]string{"path": "address", "code": "REQUIRED"})
	}
	if in.Lat < -90 || in.Lat > 90 || in.Lng < -180 || in.Lng > 180 || (in.Lat == 0 && in.Lng == 0) {
		fields = append(fields, map[string]string{"path": "location", "code": "INVALID"})
	}
	if len(fields) > 0 {
		return nil, apperr.Validation.WithDetails(map[string]any{"fields": fields})
	}
	if in.Label == "" {
		in.Label = map[string]string{"HOME": "Home", "WORK": "Work"}[in.Kind]
	}
	return s.store.upsertPlace(ctx, userID, in)
}

func (s *Service) DeletePlace(ctx context.Context, userID, placeID string) error {
	ok, err := s.store.deletePlace(ctx, userID, placeID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrPlaceNotFound
	}
	return nil
}
