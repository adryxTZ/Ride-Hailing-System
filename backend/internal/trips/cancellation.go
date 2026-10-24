package trips

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
)

// Cancellation reasons with money attached.
const (
	// ReasonNoShow is the driver's cancel after waiting the no-show time at the pickup.
	ReasonNoShow = "rider_no_show"
	// AdjustmentDues is the line for an earlier cancellation fee collected on a later trip.
	AdjustmentDues = "PREVIOUS_CANCELLATION"
)

// ErrNoShowTooEarly: the driver must wait the no-show time before cancelling for a no-show.
var ErrNoShowTooEarly = apperr.New("NO_SHOW_TOO_EARLY", http.StatusConflict, "")

// cancelTerms are the cancellation rules snapshotted on the quote option.
type cancelTerms struct {
	Fee         decimal.Decimal `json:"cancellation_fee"`
	FreeS       int             `json:"cancel_free_s"`
	NoShowAfter int             `json:"no_show_after_s"`
	DriverShare decimal.Decimal `json:"cancel_driver_share"`
}

func cancelTermsOf(option json.RawMessage) cancelTerms {
	var c cancelTerms
	_ = json.Unmarshal(option, &c)
	return c
}

// cancelFeeAt is what the rider pays if they cancel at `now`: nothing before a
// driver accepts or within the free window after; the fee once the window has
// passed or the driver is waiting at the pickup. A queued trip is free to
// cancel until it becomes current, and its window starts then.
func cancelFeeAt(t *Trip, now time.Time) decimal.Decimal {
	c := cancelTermsOf(t.QuoteOption)
	if !c.Fee.IsPositive() {
		return decimal.Zero
	}
	if t.QueuedAfter != nil && t.Status == Accepted {
		return decimal.Zero // the driver is still finishing the trip ahead
	}
	switch t.Status {
	case ArrivedAtPickup:
		return c.Fee
	case Accepted, EnRouteToPickup:
		if acc := freeCancelFrom(t); acc != nil && now.Sub(*acc) > time.Duration(c.FreeS)*time.Second {
			return c.Fee
		}
	}
	return decimal.Zero
}

// noShowAllowedAt reports whether the driver has waited long enough to cancel as a no-show.
func noShowAllowedAt(t *Trip, now time.Time) bool {
	arrived := t.Timestamps["arrived_pickup_at"]
	return t.Status == ArrivedAtPickup && arrived != nil &&
		now.Sub(*arrived) >= time.Duration(cancelTermsOf(t.QuoteOption).NoShowAfter)*time.Second
}

// NoShow ends the trip for a rider who did not come: CANCELLED_BY_DRIVER with
// the cancellation fee, and no new driver is searched.
func (s *Service) noShow(ctx context.Context, tx pgx.Tx, t *Trip, driverID string) error {
	if !noShowAllowedAt(t, s.now()) {
		return ErrNoShowTooEarly.WithDetails(map[string]any{"wait_s": cancelTermsOf(t.QuoteOption).NoShowAfter})
	}
	fee := cancelTermsOf(t.QuoteOption).Fee
	if err := s.store.setStatus(ctx, tx, t.ID, CancelledByDriver, "cancelled_at",
		`cancelled_by='DRIVER', cancel_reason=$3, cancellation_fee=NULLIF($4::numeric, 0)`, ReasonNoShow, fee); err != nil {
		return err
	}
	_, err := s.store.appendEvent(ctx, tx, t.ID, "trip.no_show", "DRIVER", &driverID, map[string]any{"status": CancelledByDriver, "fee": fee})
	return err
}

// duesAdjustment is the line that collects what the rider owes from earlier cancellations.
func duesAdjustment(dues decimal.Decimal) []Adjustment {
	if !dues.IsPositive() {
		return nil
	}
	return []Adjustment{{Code: AdjustmentDues, Amount: dues}}
}

// DuesIn is the earlier-cancellation amount collected on this trip (excluded from its fare split).
func (t *Trip) DuesIn() decimal.Decimal {
	sum := decimal.Zero
	for _, a := range t.Adjustments {
		if a.Code == AdjustmentDues {
			sum = sum.Add(a.Amount)
		}
	}
	return sum
}

// CancelShareOf is the driver's share of a cancellation fee on this trip.
func (t *Trip) CancelShareOf() decimal.Decimal {
	return cancelTermsOf(t.QuoteOption).DriverShare
}
