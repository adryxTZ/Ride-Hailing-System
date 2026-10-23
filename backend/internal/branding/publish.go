package branding

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/kalen-limited/hauler-rider/backend/internal/audit"
)

// Publish makes a draft (or an archived version, which is a rollback) the live
// theme. The contrast report must pass and a reason is required; the previous
// live version is archived, and every API instance reloads within seconds.
func (s *Service) Publish(ctx context.Context, staffID, id, reason string) (*ThemeVersion, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil, ErrReasonRequired
	}
	err := pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		t, err := s.store.theme(ctx, tx, id, true)
		if err != nil {
			return err
		}
		if t.Status == StatusPublished {
			return ErrAlreadyLive
		}
		if !t.Report.Pass {
			return ErrContrastFails.WithDetails(map[string]any{"checks": t.Report.Checks})
		}
		if err := s.store.publish(ctx, tx, id, staffID, reason); err != nil {
			return err
		}
		action := "branding.theme.publish"
		if t.Status == StatusArchived {
			action = "branding.theme.rollback"
		}
		if err := audit.Record(ctx, tx, s.store.brandID, staffID, action, "theme:"+id, reason, map[string]any{"version": t.Version}); err != nil {
			return err
		}
		return s.store.bump(ctx, tx)
	})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, id)
}
