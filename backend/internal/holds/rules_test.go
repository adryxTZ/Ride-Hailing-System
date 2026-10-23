package holds

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestDecide(t *testing.T) {
	l := Limits{Soft: decimal.NewFromInt(1000), Hard: decimal.NewFromInt(5000), GraceDays: 7, MinPayin: decimal.NewFromInt(1000)}
	t0 := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	at := func(d time.Duration) time.Time { return t0.Add(d) }
	cases := []struct {
		name  string
		debt  string
		clock *time.Time
		now   time.Time
		held  bool
		cause string
		pay   string
	}{
		{"below soft is free", "999", nil, t0, false, "", "0"},
		{"at soft starts the clock", "1000", nil, t0, false, "", "0"},
		{"one minute before grace ends", "1000", &t0, at(7*day - time.Minute), false, "", "0"},
		{"grace over: soft hold, pay the minimum", "1000", &t0, at(7 * day), true, CauseSoftLimitOverdue, "1000"},
		{"overdue with more debt pays to just under soft", "3400", &t0, at(8 * day), true, CauseSoftLimitOverdue, "2401"},
		{"hard limit holds at once", "5000", nil, t0, true, CauseHardLimit, "1000"},
		{"hard: pay to just under hard, at least the minimum", "7200.50", nil, t0, true, CauseHardLimit, "2201"},
		{"after a payment into the soft band, still overdue", "3000", &t0, at(8 * day), true, CauseSoftLimitOverdue, "2001"},
		{"fractional debt rounds up, never above the whole debt", "1234.50", &t0, at(9 * day), true, CauseSoftLimitOverdue, "1000"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := Decide(decimal.RequireFromString(c.debt), l, c.clock, c.now)
			if d.Held != c.held || d.Cause != c.cause || !d.PayToUnhold.Equal(decimal.RequireFromString(c.pay)) {
				t.Fatalf("got held=%v cause=%q pay=%s; want %v %q %s", d.Held, d.Cause, d.PayToUnhold, c.held, c.cause, c.pay)
			}
		})
	}
}

func TestDecideKeepsTheSoftClock(t *testing.T) {
	l := Limits{Soft: decimal.NewFromInt(1000), Hard: decimal.NewFromInt(5000), GraceDays: 7, MinPayin: decimal.NewFromInt(1000)}
	start := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	d := Decide(decimal.NewFromInt(1500), l, &start, start.Add(3*day))
	if d.ClockStart == nil || !d.ClockStart.Equal(start) || !d.HoldAt.Equal(start.Add(7*day)) {
		t.Fatalf("clock not kept: %+v", d)
	}
	if free := Decide(decimal.NewFromInt(999), l, &start, start.Add(3*day)); free.ClockStart != nil {
		t.Fatal("clock must clear below the soft limit")
	}
}
