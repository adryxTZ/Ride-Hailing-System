package trips

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func tripWith(status Status, accepted, arrived *time.Time) *Trip {
	opt, _ := json.Marshal(map[string]any{"cancellation_fee": "2000", "cancel_free_s": 120, "no_show_after_s": 300})
	return &Trip{Status: status, QuoteOption: opt, Timestamps: map[string]*time.Time{"accepted_at": accepted, "arrived_pickup_at": arrived}}
}

func TestCancelFee(t *testing.T) {
	now := time.Date(2026, 10, 20, 9, 10, 0, 0, time.UTC)
	justNow, earlier := now.Add(-60*time.Second), now.Add(-5*time.Minute)
	if f := cancelFeeAt(tripWith(Requested, nil, nil), now); !f.IsZero() {
		t.Fatal("free before a driver accepts")
	}
	if f := cancelFeeAt(tripWith(EnRouteToPickup, &justNow, nil), now); !f.IsZero() {
		t.Fatal("free within 2 minutes of acceptance")
	}
	if f := cancelFeeAt(tripWith(EnRouteToPickup, &earlier, nil), now); !f.Equal(decimal.NewFromInt(2000)) {
		t.Fatalf("fee after the free window, got %s", f)
	}
	if f := cancelFeeAt(tripWith(ArrivedAtPickup, &justNow, &justNow), now); !f.Equal(decimal.NewFromInt(2000)) {
		t.Fatal("fee once the driver is waiting at the pickup")
	}
}

func TestNoShowWait(t *testing.T) {
	now := time.Date(2026, 10, 20, 9, 10, 0, 0, time.UTC)
	recent, long := now.Add(-2*time.Minute), now.Add(-6*time.Minute)
	if noShowAllowedAt(tripWith(ArrivedAtPickup, &long, &recent), now) {
		t.Fatal("no-show only after 5 minutes waiting")
	}
	if !noShowAllowedAt(tripWith(ArrivedAtPickup, &long, &long), now) {
		t.Fatal("no-show allowed after 5 minutes")
	}
}
