package presence

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/kalen-limited/hauler-rider/backend/internal/providers/routing"
)

// Integration test against a real Redis (NEXA_TEST_REDIS_URL, default local compose).
func newTestService(t *testing.T) (*Service, *time.Time) {
	t.Helper()
	url := os.Getenv("NEXA_TEST_REDIS_URL")
	if url == "" {
		url = "redis://localhost:6380/9"
	}
	opt, err := redis.ParseURL(url)
	if err != nil {
		t.Fatal(err)
	}
	c := redis.NewClient(opt)
	if err := c.Ping(context.Background()).Err(); err != nil {
		t.Skipf("redis not available: %v", err)
	}
	c.FlushDB(context.Background())
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	s := New(c, "test")
	s.now = func() time.Time { return now }
	return s, &now
}

func TestOnlineNearbyAndStaleness(t *testing.T) {
	s, now := newTestService(t)
	ctx := context.Background()
	posta := routing.Point{Lat: -6.8161, Lng: 39.2894} // Dar es Salaam, Posta
	kariakoo := routing.Point{Lat: -6.8205, Lng: 39.2770}
	mbezi := routing.Point{Lat: -6.7271, Lng: 39.1619} // ~15 km away

	caps := Capabilities{Services: []string{"taxi"}, Classes: []string{"standard"}}
	if _, err := s.Online(ctx, "d1", Ping{Location: posta}, caps); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Online(ctx, "d2", Ping{Location: kariakoo}, Capabilities{Services: []string{"other_service"}, Classes: []string{"van"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Online(ctx, "d3", Ping{Location: mbezi}, caps); err != nil {
		t.Fatal(err)
	}

	got, err := s.Nearby(ctx, posta, 4, "taxi", "standard", "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "d1" {
		t.Fatalf("want only d1 within 4 rings for taxi/standard, got %+v", got)
	}
	// Indexes are per service: a driver of another service 1.45 km away is found only by that service.
	other, _ := s.Nearby(ctx, posta, 5, "other_service", "", "", 10) // 5 rings ≈ 1.75 km
	if len(other) != 1 || other[0].ID != "d2" {
		t.Fatalf("want d2 for the other service, got %+v", other)
	}
	if near, _ := s.Nearby(ctx, posta, 3, "other_service", "", "", 10); len(near) != 0 {
		t.Fatalf("1.45 km is outside 3 rings (1.05 km), got %+v", near)
	}

	// Update without going online is rejected.
	if _, err := s.Update(ctx, "ghost", Ping{Location: posta}); err != ErrNotOnline {
		t.Fatalf("want ErrNotOnline, got %v", err)
	}

	// Time passes beyond the staleness window: d1 disappears although its record exists.
	*now = now.Add(Staleness + time.Second)
	got, _ = s.Nearby(ctx, posta, 4, "taxi", "standard", "", 10)
	if len(got) != 0 {
		t.Fatalf("stale driver still visible: %+v", got)
	}
	// A fresh ping brings it back and moves its cell.
	if _, err := s.Update(ctx, "d1", Ping{Location: kariakoo}); err != nil {
		t.Fatal(err)
	}
	got, _ = s.Nearby(ctx, kariakoo, 1, "taxi", "standard", "", 10)
	if len(got) != 1 {
		t.Fatalf("refreshed driver not visible: %+v", got)
	}
	removed, err := s.GC(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if removed < 2 {
		t.Fatalf("GC should drop the two stale members, removed %d", removed)
	}
	if err := s.Offline(ctx, "d1"); err != nil {
		t.Fatal(err)
	}
	if p, _ := s.Get(ctx, "d1"); p != nil {
		t.Fatal("offline driver still has a record")
	}
}
