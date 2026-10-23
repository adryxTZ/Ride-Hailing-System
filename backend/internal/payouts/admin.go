package payouts

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/kalen-limited/hauler-rider/backend/internal/audit"
)

// Audit actions for Finance decisions.
const (
	actionApprove = "payout.approve"
	actionReject  = "payout.reject"
	// actionLatePaid: a payout recorded as refused was paid by the gateway after all.
	actionLatePaid = "payout.late_paid"
)

// AdminList is the payouts list for Finance (open ones first) in one currency ("" = all).
func (s *Service) AdminList(ctx context.Context, status, currency string, limit int) ([]AdminPayout, error) {
	if limit <= 0 || limit > adminListLimit {
		limit = adminListLimit
	}
	return s.store.adminList(ctx, strings.ToUpper(status), currency, limit)
}

// Totals are the open and paid-today sums in a currency, from midnight `since`.
func (s *Service) Totals(ctx context.Context, currency string, since time.Time) (Totals, error) {
	return s.store.totals(ctx, currency, since)
}

// Approve sends a requested payout. Under the driver's lock the balance is
// checked again (it may have dropped since the request), then the payout goes
// PENDING with its send job, audited.
func (s *Service) Approve(ctx context.Context, staffID, id, reason string) (*Payout, error) {
	if !s.Enabled() {
		return nil, ErrUnavailable
	}
	var driverID string
	err := pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		p, err := s.store.get(ctx, tx, id, "", true)
		if err != nil || p == nil {
			return firstErr(err, ErrNotFound)
		}
		if p.Status != StatusRequested {
			return ErrNotActionable
		}
		driverID = p.DriverID
		if err := s.store.lockDriver(ctx, tx, p.DriverID); err != nil {
			return err
		}
		owed, _, err := s.ledger.DriverOwedIn(ctx, tx, p.DriverID)
		if err != nil {
			return err
		}
		if p.Amount.GreaterThan(owed) {
			return ErrAboveBalance.WithDetails(map[string]any{"max": owed.Floor()})
		}
		gw, ok := s.gatewayOf(p)
		if !ok {
			return ErrUnavailable
		}
		ref := gw.PayoutReference(s.request(p, ""))
		if _, err := s.store.move(ctx, tx, id, StatusRequested, StatusPending, `, decided_by=$4, decided_at=now(), decision_reason=$5, provider_ref=$6`, staffID, reason, ref); err != nil {
			return err
		}
		if err := s.enqueueSend(ctx, tx, id); err != nil {
			return err
		}
		return audit.Record(ctx, tx, s.store.brandID, staffID, actionApprove, "payout:"+id, reason, map[string]any{"amount": p.Amount, "driver": p.DriverID})
	})
	if err != nil {
		return nil, err
	}
	s.tell(ctx, driverID)
	return s.store.get(ctx, s.store.pool, id, "", false)
}

// Reject refuses a requested payout with a reason, audited.
func (s *Service) Reject(ctx context.Context, staffID, id, reason string) (*Payout, error) {
	if strings.TrimSpace(reason) == "" {
		return nil, ErrReasonRequired
	}
	var driverID string
	err := pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		p, err := s.store.get(ctx, tx, id, "", true)
		if err != nil || p == nil {
			return firstErr(err, ErrNotFound)
		}
		driverID = p.DriverID
		ok, err := s.store.move(ctx, tx, id, StatusRequested, StatusRejected, `, decided_by=$4, decided_at=now(), decision_reason=$5, completed_at=now()`, staffID, reason)
		if err != nil || !ok {
			return firstErr(err, ErrNotActionable)
		}
		return audit.Record(ctx, tx, s.store.brandID, staffID, actionReject, "payout:"+id, reason, map[string]any{"amount": p.Amount, "driver": p.DriverID})
	})
	if err != nil {
		return nil, err
	}
	s.tell(ctx, driverID)
	return s.store.get(ctx, s.store.pool, id, "", false)
}

// AdminGet returns one payout for Finance (e.g. to check the caller may act on its currency).
func (s *Service) AdminGet(ctx context.Context, id string) (*Payout, error) {
	p, err := s.store.get(ctx, s.store.pool, id, "", false)
	if err == nil && p == nil {
		return nil, ErrNotFound
	}
	return p, err
}
