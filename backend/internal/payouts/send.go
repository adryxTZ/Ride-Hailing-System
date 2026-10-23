package payouts

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/kalen-limited/hauler-rider/backend/internal/audit"
	"github.com/kalen-limited/hauler-rider/backend/internal/providers/payment"
)

// send hands a PENDING payout to the gateway exactly once. The balance is
// checked again under the driver's lock and the payout is marked sent before
// the gateway is called, so a retried job never pays twice. Only a definite
// refusal fails it; any other error leaves it pending for the status checks.
func (s *Service) send(ctx context.Context, id string) error {
	p, ok, err := s.markSent(ctx, id)
	if err != nil || !ok {
		return err
	}
	gw, ok := s.gatewayOf(p)
	if !ok {
		slog.Warn("payouts: gateway of a payout is not configured; it stays pending", "payout", p.ID, "gateway", p.Gateway)
		return nil
	}
	_, name, _ := s.person(ctx, p.DriverID)
	err = gw.Disburse(context.WithoutCancel(ctx), s.request(p, name))
	switch {
	case errors.Is(err, payment.ErrPayoutRejected):
		return s.fail(ctx, p, failureOf(err))
	case err != nil:
		slog.Warn("payouts: unclear gateway reply; waiting for its status", "payout", id, "err", err)
	}
	return s.enqueueCheck(ctx)(p.ID)
}

// markSent re-checks the balance and stamps sent_at (false when there is nothing to send).
func (s *Service) markSent(ctx context.Context, id string) (*Payout, bool, error) {
	var out *Payout
	var short bool
	err := pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		p, err := s.store.get(ctx, tx, id, "", true)
		if err != nil || p == nil || p.Status != StatusPending || p.SentAt != nil {
			return err
		}
		if err := s.store.lockDriver(ctx, tx, p.DriverID); err != nil {
			return err
		}
		owed, _, err := s.ledger.DriverOwedIn(ctx, tx, p.DriverID)
		if err != nil {
			return err
		}
		if p.Amount.GreaterThan(owed) {
			short, out = true, p
			_, err := s.store.move(ctx, tx, id, StatusPending, StatusFailed, `, failure_code=$4, completed_at=now()`, FailBalanceChanged)
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE driver_payouts SET sent_at=now() WHERE id=$1`, id); err != nil {
			return err
		}
		out = p
		return nil
	})
	if short {
		s.tell(ctx, out.DriverID)
		return nil, false, err
	}
	return out, out != nil, err
}

// Refresh asks the gateway what happened to a sent payout and applies it: PAID
// posts to the ledger in the same transaction as the status change.
func (s *Service) Refresh(ctx context.Context, id string) (*Payout, error) {
	p, err := s.store.get(ctx, s.store.pool, id, "", false)
	if err != nil || p == nil || p.Status != StatusPending || p.SentAt == nil || p.ProviderRef == nil {
		return p, err
	}
	gw, ok := s.gatewayOf(p)
	if !ok {
		return p, nil
	}
	st, err := gw.PayoutStatus(ctx, *p.ProviderRef)
	if err != nil {
		slog.Warn("payouts: status check failed", "payout", id, "err", err)
		return p, nil // unknown is not failed
	}
	if err := s.store.touch(ctx, id); err != nil {
		return nil, err
	}
	switch st.State {
	case payment.PayoutPaid:
		return p, s.paid(ctx, p, st.ProviderTxnID)
	case payment.PayoutFailed:
		return p, s.fail(ctx, p, st.FailureCode)
	}
	return p, nil
}

// paid closes the payout and posts it in one transaction.
func (s *Service) paid(ctx context.Context, p *Payout, providerTxn string) error {
	moved := false
	err := pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		ok, err := s.store.move(ctx, tx, p.ID, StatusPending, StatusPaid, `, completed_at=now()`)
		if err != nil || !ok {
			return err
		}
		moved = true
		ref := providerTxn
		if ref == "" && p.ProviderRef != nil {
			ref = *p.ProviderRef
		}
		_, err = s.ledger.PostPayoutPaidTx(ctx, tx, p.ID, p.DriverID, p.Amount, p.Currency, ref)
		return err
	})
	if err == nil && moved {
		s.ledger.WalletMoved(ctx, p.DriverID)
		s.tell(ctx, p.DriverID)
	}
	return err
}

// fail closes the payout as FAILED (no money moved; the balance stays with the driver).
func (s *Service) fail(ctx context.Context, p *Payout, code string) error {
	err := pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		_, err := s.store.move(ctx, tx, p.ID, StatusPending, StatusFailed, `, failure_code=$4, completed_at=now()`, code)
		return err
	})
	if err == nil {
		s.tell(ctx, p.DriverID)
	}
	return err
}

// Reconcile checks open payouts nobody looked at lately and sends any whose send job was lost.
func (s *Service) Reconcile(ctx context.Context) error {
	if !s.Enabled() {
		return nil
	}
	sent, unsent, err := s.store.toCheck(ctx, reconcileBatch)
	if err != nil {
		return err
	}
	for _, id := range sent {
		if _, err := s.Refresh(ctx, id); err != nil {
			slog.Warn("payouts: reconcile refresh failed", "payout", id, "err", err)
		}
	}
	for _, id := range unsent {
		if err := s.send(ctx, id); err != nil {
			slog.Warn("payouts: reconcile send failed", "payout", id, "err", err)
		}
	}
	return s.recheckRefused(ctx)
}

// recheckRefused asks again about recently refused payouts: if the gateway now
// says PAID, the money left after all, so it is posted (the driver can no longer
// cash it out twice) and flagged loudly for Finance.
func (s *Service) recheckRefused(ctx context.Context) error {
	ids, err := s.store.recentlyRefused(ctx, lateWindow, reconcileBatch)
	if err != nil {
		return err
	}
	for _, id := range ids {
		p, err := s.store.get(ctx, s.store.pool, id, "", false)
		if err != nil || p == nil || p.ProviderRef == nil {
			continue
		}
		_ = s.store.touch(ctx, id)
		gw, ok := s.gatewayOf(p)
		if !ok {
			continue
		}
		st, err := gw.PayoutStatus(ctx, *p.ProviderRef)
		if err != nil || st.State != payment.PayoutPaid {
			continue
		}
		slog.Error("payouts: a refused payout was paid after all; posting it", "payout", id, "driver", p.DriverID)
		if err := s.settleLate(ctx, p, st.ProviderTxnID); err != nil {
			slog.Warn("payouts: late payment not posted", "payout", id, "err", err)
		}
	}
	return nil
}

// settleLate turns a FAILED payout the gateway paid into PAID, posted and audited.
func (s *Service) settleLate(ctx context.Context, p *Payout, providerTxn string) error {
	err := pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		ok, err := s.store.move(ctx, tx, p.ID, StatusFailed, StatusPaid, `, completed_at=now()`)
		if err != nil || !ok {
			return err
		}
		if _, err := s.ledger.PostPayoutPaidTx(ctx, tx, p.ID, p.DriverID, p.Amount, p.Currency, providerTxn); err != nil {
			return err
		}
		return audit.Record(ctx, tx, s.store.brandID, "", actionLatePaid, "payout:"+p.ID, "",
			map[string]any{"amount": p.Amount, "driver": p.DriverID, "failure_was": p.FailureCode})
	})
	if err == nil {
		s.ledger.WalletMoved(ctx, p.DriverID)
		s.tell(ctx, p.DriverID)
	}
	return err
}

// enqueueCheck schedules the first status check of a sent payout.
func (s *Service) enqueueCheck(ctx context.Context) func(id string) error {
	return func(id string) error {
		if s.jobs == nil {
			return nil
		}
		_, err := s.jobs.Insert(ctx, CheckArgs{PayoutID: id}, &river.InsertOpts{ScheduledAt: s.now().Add(FirstCheck), UniqueOpts: river.UniqueOpts{ByArgs: true}})
		return err
	}
}

// failureOf picks the failure code out of a gateway refusal ("payment: payout rejected: WALLET_INVALID").
func failureOf(err error) string {
	msg := err.Error()
	if i := strings.LastIndex(msg, ": "); i >= 0 && i+2 < len(msg) {
		if code := strings.Fields(msg[i+2:]); len(code) > 0 && strings.ToUpper(code[0]) == code[0] {
			return code[0]
		}
	}
	return payment.FailPayoutRefused
}
