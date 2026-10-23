package trips

import (
	"testing"
	"time"
)

func TestBookingTiming(t *testing.T) {
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	s := &Service{now: func() time.Time { return now }}
	if st, _, err := s.bookingTiming(nil); err != nil || st != Requested {
		t.Fatalf("no time = ride now, got %s %v", st, err)
	}
	soon := now.Add(10 * time.Minute)
	if _, _, err := s.bookingTiming(&soon); err == nil {
		t.Fatal("10 minutes ahead is too soon to schedule")
	}
	far := now.Add(8 * 24 * time.Hour)
	if _, _, err := s.bookingTiming(&far); err == nil {
		t.Fatal("8 days ahead is too far")
	}
	ok := now.Add(2 * time.Hour)
	st, ends, err := s.bookingTiming(&ok)
	if err != nil || st != Scheduled || !ends.Equal(ok.Add(DispatchWindow)) {
		t.Fatalf("2 hours ahead schedules with matching until pickup + window, got %s %s %v", st, ends, err)
	}
}

func TestScheduledRideCanBeCancelled(t *testing.T) {
	if !RiderMayCancel(Scheduled) || !CanTransition(Scheduled, CancelledByRider, Lifecycle{}) {
		t.Fatal("a scheduled ride can be cancelled by the rider")
	}
}
