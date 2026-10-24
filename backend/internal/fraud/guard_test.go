package fraud

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// TestGuardQueuesOncePerWindow: three strikes queue one flag job, a fourth in
// the same window queues nothing more, a mocked ping queues at once. Needs Redis.
func TestGuardQueuesOncePerWindow(t *testing.T) {
	url := os.Getenv("NEXA_TEST_REDIS_URL")
	if url == "" {
		url = "redis://localhost:6380/9"
	}
	opt, err := redis.ParseURL(url)
	if err != nil {
		t.Fatal(err)
	}
	r := redis.NewClient(opt)
	ctx := context.Background()
	if err := r.Ping(ctx).Err(); err != nil {
		t.Skipf("redis not available: %v", err)
	}
	var queued []GPSFlagArgs
	g := NewGPSGuard(r, "test", func(_ context.Context, a GPSFlagArgs) error { queued = append(queued, a); return nil })
	driver := "guard-test-" + time.Now().Format("150405.000")
	t.Cleanup(func() { r.Del(ctx, g.strikesKey(driver), g.mockedKey(driver), g.evidenceKey(driver)) })
	t0 := time.Now()
	jump := func(i int) (Ping, Ping) {
		a := Ping{Lat: -6.80, Lng: 39.28, At: t0.Add(time.Duration(i) * time.Minute)}
		return a, Ping{Lat: -6.75, Lng: 39.28, At: a.At.Add(2 * time.Second)}
	}
	for i := 0; i < 4; i++ {
		a, b := jump(i)
		g.Observe(ctx, driver, &a, b)
		if i == 1 && len(queued) != 0 {
			t.Fatal("two strikes must not flag")
		}
	}
	if len(queued) != 1 || len(queued[0].Pairs) != GPSEvidencePairs || queued[0].Mocked {
		t.Fatalf("three strikes must queue one flag with the last pairs, got %+v", queued)
	}
	normal := Ping{Lat: -6.80, Lng: 39.28, At: t0}
	g.Observe(ctx, driver, &normal, Ping{Lat: -6.8001, Lng: 39.28, At: t0.Add(10 * time.Second)})
	g.Observe(ctx, driver, &normal, Ping{Lat: -6.80, Lng: 39.28, At: t0.Add(20 * time.Second), Mocked: true})
	if len(queued) != 2 || !queued[1].Mocked {
		t.Fatalf("a mocked ping must queue at once, got %d", len(queued))
	}
}
