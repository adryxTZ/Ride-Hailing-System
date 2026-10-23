package trips

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestWaitingFee(t *testing.T) {
	arrived := time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)
	w := waitingTerms{GraceS: 180, RatePerMin: decimal.NewFromInt(130)}
	if fee, _ := waitingFee(w, arrived, arrived.Add(2*time.Minute)); !fee.IsZero() {
		t.Fatalf("within the free 3 minutes: got %s", fee)
	}
	// 5 min 10 s: 2 min 10 s over → 3 started minutes × 130
	if fee, m := waitingFee(w, arrived, arrived.Add(5*time.Minute+10*time.Second)); !fee.Equal(decimal.NewFromInt(390)) || m != 3 {
		t.Fatalf("got %s for %d minutes, want 390 for 3", fee, m)
	}
	w.Cap = decimal.NewFromInt(450)
	if fee, _ := waitingFee(w, arrived, arrived.Add(20*time.Minute)); !fee.Equal(decimal.NewFromInt(450)) {
		t.Fatalf("cap not applied: %s", fee)
	}
}
