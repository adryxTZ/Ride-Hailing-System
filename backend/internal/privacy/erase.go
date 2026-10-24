package privacy

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5"

	"github.com/kalen-limited/hauler-rider/backend/internal/audit"
)

// erased is what is cleaned up after the anonymising transaction commits.
type erased struct {
	userID   string
	photoKey string
	driver   bool
}

// blockers lists what must be settled before an account can be deleted.
func (s *Service) blockers(ctx context.Context, userID string) ([]string, error) {
	out := []string{}
	open, err := s.store.openWork(ctx, userID)
	if err != nil {
		return nil, err
	}
	if open.activeTrip {
		out = append(out, BlockActiveTrip)
	}
	if open.openPayout {
		out = append(out, BlockOpenPayout)
	}
	if open.owedToRider {
		out = append(out, BlockOwedToRider)
	}
	if s.d.Money == nil {
		return out, nil
	}
	if debt, _, err := s.d.Money.DriverDebt(ctx, userID); err != nil {
		return nil, err
	} else if debt.IsPositive() {
		out = append(out, BlockDriverDebt)
	}
	if owed, _, err := s.d.Money.DriverOwed(ctx, userID); err != nil {
		return nil, err
	} else if owed.IsPositive() {
		out = append(out, BlockDriverBalance)
	}
	if dues, err := s.d.Money.RiderDues(ctx, userID); err != nil {
		return nil, err
	} else if dues.IsPositive() {
		out = append(out, BlockRiderDues)
	}
	return out, nil
}

// approveDelete anonymises the account in the approving transaction, or
// returns DELETE_BLOCKED with what is still open. Trips, receipts, ledger,
// audit and consents are kept (legal records); they keep the user id only.
func (s *Service) approveDelete(ctx context.Context, tx pgx.Tx, staffID string, r *Request, reason string) (*erased, error) {
	blocked, err := s.blockers(ctx, r.UserID)
	if err != nil {
		return nil, err
	}
	if len(blocked) > 0 {
		return nil, &blockedError{blockers: blocked}
	}
	e, err := s.store.anonymise(ctx, tx, r.UserID)
	if err != nil {
		return nil, err
	}
	if err := s.store.setStatus(ctx, tx, r.ID, StatusDone, staffID, reason); err != nil {
		return nil, err
	}
	return e, audit.Record(ctx, tx, s.store.brandID, staffID, "privacy.delete", "user:"+r.UserID, reason, map[string]any{"request": r.ID})
}

// blockedError carries the blockers out of the rolled-back transaction, so they
// are saved on the request after it (saving inside would be undone).
type blockedError struct{ blockers []string }

func (e *blockedError) Error() string { return "privacy: delete blocked" }

// afterErase removes what lives outside Postgres: the photo file, and the driver's presence.
func (s *Service) afterErase(ctx context.Context, e *erased) {
	if e.photoKey != "" && s.d.Writer != nil {
		if err := s.d.Writer.Delete(ctx, e.photoKey); err != nil {
			slog.Warn("privacy: delete photo", "err", err)
		}
	}
	if e.driver && s.d.Offline != nil {
		if err := s.d.Offline(ctx, e.userID); err != nil {
			slog.Warn("privacy: driver offline", "err", err)
		}
	}
}
