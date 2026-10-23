package holds

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/audit"
	"github.com/kalen-limited/hauler-rider/backend/internal/presence"
)

// Audit actions written by this module.
const (
	actionDebtPlace     = "driver.hold.debt.place"
	actionDebtRelease   = "driver.hold.debt.release"
	actionManualPlace   = "driver.hold.place"
	actionManualRelease = "driver.hold.release"
)

// Service owns driver holds.
type Service struct {
	store    store
	debts    Debts
	limits   LimitSource
	presence Presence // nil when presence is disabled
	notify   Notifier // nil = nobody to tell
	now      func() time.Time
}

// New builds the holds service. presence and notify may be nil.
func New(pool *pgxpool.Pool, brandID string, debts Debts, limits LimitSource, p Presence, notify Notifier) *Service {
	return &Service{store: store{pool: pool, brandID: brandID}, debts: debts, limits: limits, presence: p, notify: notify, now: time.Now}
}

// Evaluate applies the debt rule to one driver: keeps the soft clock and places,
// re-causes or lifts the DEBT hold (never a MANUAL one). On a change the driver
// is taken offline (unless on a trip) and their app is told.
func (s *Service) Evaluate(ctx context.Context, driverID string) (Decision, error) {
	var d Decision
	changed := false
	err := pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		// One evaluation per driver at a time, and the debt is read under the lock,
		// so two evaluations (a cash trip and a pay-in) cannot apply out of order.
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('hold:' || $1))`, driverID); err != nil {
			return err
		}
		l, err := s.limits(ctx)
		if err != nil {
			return err
		}
		debt, _, err := s.debts.DriverDebtIn(ctx, tx, driverID)
		if err != nil {
			return err
		}
		clock, err := s.store.clock(ctx, tx, driverID)
		if err != nil {
			return err
		}
		d = Decide(debt, l, clock, s.now())
		if err := s.store.saveClock(ctx, tx, driverID, d.ClockStart); err != nil {
			return err
		}
		changed, err = s.applyDebtHold(ctx, tx, driverID, d, debt)
		return err
	})
	if err == nil && changed {
		s.after(ctx, driverID)
	}
	return d, err
}

// applyDebtHold makes the DEBT hold row match the decision; true when it changed.
func (s *Service) applyDebtHold(ctx context.Context, tx pgx.Tx, driverID string, d Decision, debt decimal.Decimal) (bool, error) {
	id, cause, err := s.store.debtHold(ctx, tx, driverID)
	if err != nil {
		return false, err
	}
	data := map[string]any{"debt": debt, "cause": d.Cause}
	switch {
	case d.Held && id == "":
		if err := s.store.place(ctx, tx, driverID, KindDebt, d.Cause, "", ""); err != nil {
			return false, err
		}
		return true, audit.Record(ctx, tx, s.store.brandID, "", actionDebtPlace, "driver:"+driverID, "", data)
	case d.Held && cause != d.Cause:
		return false, s.store.setCause(ctx, tx, id, d.Cause)
	case !d.Held && id != "":
		if _, err := s.store.release(ctx, tx, driverID, KindDebt, "", ""); err != nil {
			return false, err
		}
		return true, audit.Record(ctx, tx, s.store.brandID, "", actionDebtRelease, "driver:"+driverID, "", data)
	}
	return false, nil
}

// after runs once a hold change has committed.
func (s *Service) after(ctx context.Context, driverID string) {
	if err := s.Enforce(ctx, driverID); err != nil {
		slog.Warn("holds: could not take held driver offline", "driver", driverID, "err", err)
	}
	if s.notify != nil {
		s.notify(ctx, driverID)
	}
}

// Enforce takes a held driver offline. A driver on a trip finishes it first; the
// trip-ended hook calls Enforce again.
func (s *Service) Enforce(ctx context.Context, driverID string) error {
	if s.presence == nil {
		return nil
	}
	hs, err := s.Active(ctx, driverID)
	if err != nil || len(hs) == 0 {
		return err
	}
	p, err := s.presence.Get(ctx, driverID)
	if err != nil || p == nil || p.State == presence.OnTrip {
		return err
	}
	return s.presence.Offline(ctx, driverID)
}

// Active lists a driver's active holds (empty when free to drive).
func (s *Service) Active(ctx context.Context, driverID string) ([]Hold, error) {
	return s.store.active(ctx, s.store.pool, driverID)
}

// ListActive lists every active hold with the driver and their debt, newest first.
func (s *Service) ListActive(ctx context.Context, country string, limit int) ([]ActiveHold, error) {
	return s.store.listActive(ctx, country, limit)
}

// Place puts a MANUAL hold on a driver (staff, with a reason, audited).
func (s *Service) Place(ctx context.Context, staffID, driverID, reason string) (Hold, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return Hold{}, ErrReasonRequired
	}
	if err := s.requireDriver(ctx, driverID); err != nil {
		return Hold{}, err
	}
	err := pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		if err := s.store.place(ctx, tx, driverID, KindManual, CauseStaff, reason, staffID); err != nil {
			return err
		}
		return audit.Record(ctx, tx, s.store.brandID, staffID, actionManualPlace, "driver:"+driverID, reason, nil)
	})
	if err != nil {
		return Hold{}, err
	}
	s.after(ctx, driverID)
	return s.manualHold(ctx, driverID)
}

// Release ends a driver's MANUAL hold (staff, with a reason, audited). A DEBT hold
// is untouched: it lifts only when the debt does. Returns the holds still active.
func (s *Service) Release(ctx context.Context, staffID, driverID, reason string) ([]Hold, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil, ErrReasonRequired
	}
	if _, err := uuid.Parse(driverID); err != nil {
		return nil, ErrNoManualHold
	}
	err := pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		ok, err := s.store.release(ctx, tx, driverID, KindManual, reason, staffID)
		if err != nil {
			return err
		}
		if !ok {
			return ErrNoManualHold
		}
		return audit.Record(ctx, tx, s.store.brandID, staffID, actionManualRelease, "driver:"+driverID, reason, nil)
	})
	if err != nil {
		return nil, err
	}
	if s.notify != nil {
		s.notify(ctx, driverID)
	}
	return s.Active(ctx, driverID)
}

// Status is the driver's debt against the rule, for the wallet (read-only).
func (s *Service) Status(ctx context.Context, driverID string) (Status, error) {
	l, err := s.limits(ctx)
	if err != nil {
		return Status{}, err
	}
	var debt decimal.Decimal
	err = pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		var err error
		debt, _, err = s.debts.DriverDebtIn(ctx, tx, driverID)
		return err
	})
	if err != nil {
		return Status{}, err
	}
	clock, err := s.store.clock(ctx, s.store.pool, driverID)
	if err != nil {
		return Status{}, err
	}
	hs, err := s.Active(ctx, driverID)
	if err != nil {
		return Status{}, err
	}
	d := Decide(debt, l, clock, s.now())
	if hasKind(hs, KindDebt) != d.Held {
		// The hold row lags the rule (limits changed, or money moved without a hook):
		// fix it now rather than show the driver a hold they cannot act on.
		if d, err = s.Evaluate(ctx, driverID); err != nil {
			return Status{}, err
		}
		if hs, err = s.Active(ctx, driverID); err != nil {
			return Status{}, err
		}
	}
	st := Status{Debt: debt, Limits: l, Holds: hs, SoftHoldAt: d.HoldAt}
	if d.Held {
		pay := d.PayToUnhold
		st.PayToUnhold = &pay
	}
	return st, nil
}

func (s *Service) requireDriver(ctx context.Context, driverID string) error {
	if _, err := uuid.Parse(driverID); err != nil {
		return ErrDriverNotFound // a malformed id is "no such driver", not a server error
	}
	ok, err := s.store.driverExists(ctx, driverID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrDriverNotFound
	}
	return nil
}

func (s *Service) manualHold(ctx context.Context, driverID string) (Hold, error) {
	hs, err := s.Active(ctx, driverID)
	if err != nil {
		return Hold{}, err
	}
	for _, h := range hs {
		if h.Kind == KindManual {
			return h, nil
		}
	}
	return Hold{}, ErrNoManualHold
}

func hasKind(hs []Hold, kind string) bool {
	for _, h := range hs {
		if h.Kind == kind {
			return true
		}
	}
	return false
}

// Audit actions of system document holds.
const (
	actionDocumentPlace   = "hold.document.place"
	actionDocumentRelease = "hold.document.release"
)

// SetDocumentHold holds a driver while any of `expired` (document kinds) is
// expired, and lifts the hold once none is. The reason lists the kinds, so the
// driver knows what to renew. Idempotent; the driver goes offline when held.
func (s *Service) SetDocumentHold(ctx context.Context, driverID string, expired []string) error {
	changed := false
	err := pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		active, err := s.store.active(ctx, tx, driverID)
		if err != nil {
			return err
		}
		held := false
		for _, h := range active {
			held = held || h.Kind == KindDocument
		}
		reason := strings.Join(expired, ", ")
		switch {
		case len(expired) > 0 && !held:
			if err := s.store.place(ctx, tx, driverID, KindDocument, CauseDocumentExpired, reason, ""); err != nil {
				return err
			}
			changed = true
			return audit.Record(ctx, tx, s.store.brandID, "", actionDocumentPlace, "driver:"+driverID, reason, nil)
		case len(expired) == 0 && held:
			if _, err := s.store.release(ctx, tx, driverID, KindDocument, "documents renewed", ""); err != nil {
				return err
			}
			changed = true
			return audit.Record(ctx, tx, s.store.brandID, "", actionDocumentRelease, "driver:"+driverID, "documents renewed", nil)
		}
		return nil
	})
	if err == nil && changed {
		s.after(ctx, driverID)
	}
	return err
}
