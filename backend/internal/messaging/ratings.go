package messaging

import (
	"context"
	"strings"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
)

// maxAspects bounds the chips one rating may carry.
const maxAspects = 8

// completedStatus is the only status a trip can be rated in.
const completedStatus = "COMPLETED"

// Rate records the caller's rating of the other participant on a completed trip.
// Each side rates once.
func (s *Service) Rate(ctx context.Context, tripID, userID string, in RateInput) (*Rating, error) {
	if err := validateRating(in); err != nil {
		return nil, err
	}
	t, role, err := s.participant(ctx, tripID, userID)
	if err != nil {
		return nil, err
	}
	if t.Status != completedStatus || t.DriverID == "" {
		return nil, ErrNotRateable
	}
	ratee, _ := other(t, role)
	r := Rating{TripID: tripID, RaterRole: role, Score: in.Score, Aspects: in.Aspects, Comment: in.Comment}
	if r.Aspects == nil {
		r.Aspects = []string{}
	}
	if err := s.store.insertRating(ctx, r, userID, ratee); err != nil {
		return nil, err
	}
	return &r, nil
}

// Ratings returns the ratings on a trip for one of its participants.
func (s *Service) Ratings(ctx context.Context, tripID, userID string) ([]Rating, error) {
	if _, _, err := s.participant(ctx, tripID, userID); err != nil {
		return nil, err
	}
	return s.store.ratings(ctx, tripID)
}

// Summary is a person's average rating and how many ratings it is based on.
func (s *Service) Summary(ctx context.Context, userID string) (Summary, error) {
	return s.store.summary(ctx, userID)
}

func validateRating(in RateInput) error {
	var fields []map[string]string
	if in.Score < MinScore || in.Score > MaxScore {
		fields = append(fields, map[string]string{"path": "score", "code": "OUT_OF_RANGE"})
	}
	if len(in.Aspects) > maxAspects {
		fields = append(fields, map[string]string{"path": "aspects", "code": "TOO_MANY"})
	}
	if in.Comment != nil && len([]rune(strings.TrimSpace(*in.Comment))) > MaxCommentLen {
		fields = append(fields, map[string]string{"path": "comment", "code": "TOO_LONG"})
	}
	if len(fields) > 0 {
		return apperr.Validation.WithDetails(map[string]any{"fields": fields})
	}
	return nil
}
