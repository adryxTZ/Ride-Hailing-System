package presence

import (
	"context"
	"testing"

	"github.com/kalen-limited/hauler-rider/backend/internal/providers/routing"
)

// onTripDriver puts d1 online and on trip "a" (Redis integration, see newTestService).
func onTripDriver(t *testing.T) (*Service, context.Context) {
	t.Helper()
	s, _ := newTestService(t)
	ctx := context.Background()
	at := routing.Point{Lat: -6.7764, Lng: 39.229}
	if _, err := s.Online(ctx, "d1", Ping{Location: at}, Capabilities{Services: []string{"taxi"}, Classes: []string{"standard"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetState(ctx, "d1", OnTrip, "a", "r1"); err != nil {
		t.Fatal(err)
	}
	return s, ctx
}

func get(t *testing.T, s *Service, ctx context.Context) *Driver {
	t.Helper()
	d, err := s.Get(ctx, "d1")
	if err != nil || d == nil {
		t.Fatalf("get: %v", err)
	}
	return d
}

func TestFinishingThenQueuedThenPromoted(t *testing.T) {
	s, ctx := onTripDriver(t)
	drop := routing.Point{Lat: -6.7724, Lng: 39.2181}
	_ = s.SetFinishing(ctx, "d1", "other", drop) // a stale trip id is ignored
	if get(t, s, ctx).Finishing {
		t.Fatal("finishing must only follow the current trip")
	}
	_ = s.SetFinishing(ctx, "d1", "a", drop)
	d := get(t, s, ctx)
	if !d.Finishing || d.Dropoff == nil || d.Dropoff.Lat != drop.Lat || d.State != OnTrip {
		t.Fatalf("finishing: %+v", d)
	}
	if ok, _ := s.SetQueued(ctx, "d1", "b"); !ok {
		t.Fatal("first queued trip")
	}
	if ok, _ := s.SetQueued(ctx, "d1", "c"); ok {
		t.Fatal("a second queued trip must be refused")
	}
	_ = s.Promote(ctx, "d1", "a", "b", "r2", OnTrip)
	d = get(t, s, ctx)
	if d.TripID != "b" || d.RiderID != "r2" || d.Finishing || d.QueuedTripID != "" || d.Dropoff != nil {
		t.Fatalf("promoted: %+v", d)
	}
}

func TestReleaseOfferKeepsTheCurrentTrip(t *testing.T) {
	s, ctx := onTripDriver(t)
	_, _ = s.SetQueued(ctx, "d1", "b")
	_ = s.ReleaseOffer(ctx, "d1", "b") // the queued offer was declined
	d := get(t, s, ctx)
	if d.State != OnTrip || d.TripID != "a" || d.QueuedTripID != "" {
		t.Fatalf("released queued offer: %+v", d)
	}
	_ = s.SetState(ctx, "d1", Offered, "x", "r3")
	_ = s.ReleaseOffer(ctx, "d1", "x") // a plain offer: back to idle
	if d := get(t, s, ctx); d.State != Idle || d.TripID != "" {
		t.Fatalf("released plain offer: %+v", d)
	}
}

func TestOfflineHookSeesTheQueuedTrip(t *testing.T) {
	s, ctx := onTripDriver(t)
	_, _ = s.SetQueued(ctx, "d1", "b")
	var seen string
	s.OnOffline(func(_ context.Context, d Driver) { seen = d.QueuedTripID })
	if err := s.Offline(ctx, "d1"); err != nil {
		t.Fatal(err)
	}
	if seen != "b" {
		t.Fatalf("offline hook saw %q", seen)
	}
	if d, _ := s.Get(ctx, "d1"); d != nil {
		t.Fatal("record removed")
	}
}
