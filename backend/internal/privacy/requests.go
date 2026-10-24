package privacy

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/kalen-limited/hauler-rider/backend/internal/audit"
)

// Create files a person's export or deletion request (one open per kind, three a day).
func (s *Service) Create(ctx context.Context, userID, app, kind string) (*Request, error) {
	if (kind != KindExport && kind != KindDelete) || (app != AppRider && app != AppDriver) {
		return nil, ErrInvalid // only rider and driver accounts file requests
	}
	if err := s.rateLimit(ctx, userID); err != nil {
		return nil, err
	}
	due := s.now().Add(time.Duration(s.d.Brand.Privacy.RequestDueDays) * 24 * time.Hour)
	id, err := s.store.insertRequest(ctx, userID, app, kind, due)
	if err != nil {
		return nil, err
	}
	return s.store.request(ctx, nil, id, false)
}

// rateLimit allows RequestsPerDay requests per person per day (Redis; skipped without it).
func (s *Service) rateLimit(ctx context.Context, userID string) error {
	if s.d.Redis == nil {
		return nil
	}
	key := fmt.Sprintf("privacy:rl:%s:%s", s.d.Brand.Code, userID)
	pipe := s.d.Redis.TxPipeline() // the window is set with the first request, in the same round trip
	pipe.SetNX(ctx, key, 0, 24*time.Hour)
	incr := pipe.Incr(ctx, key)
	if _, err := pipe.Exec(ctx); err != nil {
		return err
	}
	n := incr.Val()
	if n > RequestsPerDay {
		return ErrRateLimited
	}
	return nil
}

// Mine lists a person's requests, newest first.
func (s *Service) Mine(ctx context.Context, userID string) ([]Request, error) {
	return s.store.mine(ctx, userID)
}

// Cancel withdraws a request staff have not picked up yet.
func (s *Service) Cancel(ctx context.Context, userID, id string) (*Request, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, ErrNotFound
	}
	ok, err := s.store.cancel(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotOpen
	}
	return s.store.request(ctx, nil, id, false)
}

// Download is a short-lived link to a ready export (owner only).
func (s *Service) Download(ctx context.Context, userID, id string) (string, time.Time, error) {
	if _, err := uuid.Parse(id); err != nil {
		return "", time.Time{}, ErrNotFound
	}
	r, key, err := s.store.export(ctx, userID, id)
	if err != nil {
		return "", time.Time{}, err
	}
	if r.Status != StatusReady || key == "" {
		return "", time.Time{}, ErrExportNotReady
	}
	if r.DownloadExpiresAt == nil || s.now().After(*r.DownloadExpiresAt) {
		return "", time.Time{}, ErrExportExpired
	}
	url, err := s.d.Files.DownloadURL(ctx, key, DownloadURLTTL)
	return url, s.now().Add(DownloadURLTTL), err
}

// List is the staff queue, oldest due first.
func (s *Service) List(ctx context.Context, f Filter) ([]Request, error) {
	if f.Limit <= 0 || f.Limit > listLimitMax {
		f.Limit = listLimitDefault
	}
	list, err := s.store.list(ctx, f)
	for i := range list {
		list[i].Overdue = s.overdue(list[i])
	}
	return list, err
}

// Get is one request; an open deletion shows what would block it now.
func (s *Service) Get(ctx context.Context, id string) (*Request, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, ErrNotFound
	}
	r, err := s.store.request(ctx, nil, id, false)
	if err != nil {
		return nil, err
	}
	r.Overdue = s.overdue(*r)
	if r.Kind == KindDelete && r.Status == StatusRequested {
		if r.Blockers, err = s.blockers(ctx, r.UserID); err != nil {
			return nil, err
		}
	}
	return r, nil
}

// OverdueCount is how many open requests in a country ("" = all) are past their due date (Overview).
func (s *Service) OverdueCount(ctx context.Context, country string) (int, error) {
	return s.store.overdueCount(ctx, country)
}

func (s *Service) overdue(r Request) bool {
	open := r.Status == StatusRequested || r.Status == StatusInProgress
	return open && s.now().After(r.DueAt)
}

// Approve acts on a request: an export is queued for building; a deletion
// anonymises the account at once, or fails with what blocks it.
func (s *Service) Approve(ctx context.Context, staffID, id, reason string) (*Request, error) {
	reason, err := checkReason(reason)
	if err != nil {
		return nil, err
	}
	var deleted *erased
	err = pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		r, err := s.store.request(ctx, tx, id, true)
		if err != nil {
			return err
		}
		if r.Status != StatusRequested {
			return ErrNotOpen
		}
		if r.Kind == KindExport {
			return s.approveExport(ctx, tx, staffID, r, reason)
		}
		deleted, err = s.approveDelete(ctx, tx, staffID, r, reason)
		return err
	})
	var be *blockedError
	if errors.As(err, &be) {
		if serr := s.store.saveBlockers(ctx, id, be.blockers); serr != nil {
			slog.Warn("privacy: save blockers", "err", serr)
		}
		return nil, ErrDeleteBlocked.WithDetails(map[string]any{"blockers": be.blockers})
	}
	if err != nil {
		return nil, err
	}
	if deleted != nil {
		s.afterErase(ctx, deleted)
	}
	return s.Get(ctx, id)
}

func (s *Service) approveExport(ctx context.Context, tx pgx.Tx, staffID string, r *Request, reason string) error {
	if s.d.Writer == nil || s.d.EnqueueExportTx == nil {
		return fmt.Errorf("privacy: exports need a writable file store")
	}
	if err := s.store.setStatus(ctx, tx, r.ID, StatusInProgress, staffID, reason); err != nil {
		return err
	}
	if err := s.d.EnqueueExportTx(ctx, tx, r.ID); err != nil {
		return err
	}
	return audit.Record(ctx, tx, s.store.brandID, staffID, "privacy.export", "user:"+r.UserID, reason, map[string]any{"request": r.ID})
}

// Reject closes a request with the reason the person will see.
func (s *Service) Reject(ctx context.Context, staffID, id, reason string) (*Request, error) {
	reason, err := checkReason(reason)
	if err != nil {
		return nil, err
	}
	err = pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		r, err := s.store.request(ctx, tx, id, true)
		if err != nil {
			return err
		}
		if r.Status != StatusRequested {
			return ErrNotOpen
		}
		if err := s.store.setStatus(ctx, tx, id, StatusRejected, staffID, reason); err != nil {
			return err
		}
		return audit.Record(ctx, tx, s.store.brandID, staffID, "privacy.reject", "user:"+r.UserID, reason, map[string]any{"request": id, "kind": r.Kind})
	})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, id)
}

func checkReason(reason string) (string, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return "", ErrReasonRequired
	}
	if len(reason) > reasonMaxLen {
		return "", ErrInvalid
	}
	return reason, nil
}
