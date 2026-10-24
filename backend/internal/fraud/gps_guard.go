package fraud

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

// PingPair is a previous and current ping that struck, kept as evidence.
type PingPair struct {
	From     *Ping   `json:"from,omitempty"`
	To       Ping    `json:"to"`
	SpeedMps float64 `json:"speed_mps,omitempty"`
	JumpM    float64 `json:"jump_m,omitempty"`
}

// Enqueuer queues the flag job (River, outside any transaction).
type Enqueuer func(ctx context.Context, args GPSFlagArgs) error

// GPSGuard counts GPS strikes in Redis on the ping path. A normal ping costs
// nothing (CheckPing is pure); a strike costs one Redis round trip; reaching
// the threshold in the window queues one job, so Postgres is never touched here.
type GPSGuard struct {
	r       *redis.Client
	brand   string
	enqueue Enqueuer
	now     func() time.Time
}

// NewGPSGuard builds the guard.
func NewGPSGuard(r *redis.Client, brand string, enqueue Enqueuer) *GPSGuard {
	return &GPSGuard{r: r, brand: brand, enqueue: enqueue, now: time.Now}
}

func (g *GPSGuard) strikesKey(driverID string) string { return "fraud:gps:" + g.brand + ":" + driverID }
func (g *GPSGuard) mockedKey(driverID string) string {
	return "fraud:gps:mock:" + g.brand + ":" + driverID
}
func (g *GPSGuard) evidenceKey(driverID string) string {
	return "fraud:gps:ev:" + g.brand + ":" + driverID
}

// Observe checks one ping against the previous one. Errors are logged, never
// returned: the ping itself is always accepted.
func (g *GPSGuard) Observe(ctx context.Context, driverID string, prev *Ping, cur Ping) {
	st := CheckPing(prev, cur)
	if !st.Hit {
		return
	}
	n, pairs, err := g.strike(ctx, driverID, st, PingPair{From: prev, To: cur, SpeedMps: st.SpeedMps, JumpM: st.JumpM})
	if err != nil {
		slog.Warn("fraud gps guard", "err", err)
		return
	}
	// Flag once per window: the first mocked ping, or the strike that reaches the threshold.
	if (st.Mocked && n == 1) || (!st.Mocked && n == GPSStrikesToFlag) {
		if err := g.enqueue(ctx, GPSFlagArgs{DriverID: driverID, Mocked: st.Mocked, Pairs: pairs, At: g.now()}); err != nil {
			slog.Warn("fraud gps flag enqueue", "err", err)
		}
	}
}

// strike counts the strike and keeps the last pairs (one pipeline).
func (g *GPSGuard) strike(ctx context.Context, driverID string, st Strike, pair PingPair) (int64, []PingPair, error) {
	raw, _ := json.Marshal(pair)
	key := g.strikesKey(driverID)
	if st.Mocked {
		key = g.mockedKey(driverID)
	}
	pipe := g.r.TxPipeline()
	incr := pipe.Incr(ctx, key)
	pipe.ExpireNX(ctx, key, GPSStrikeWindow)
	pipe.LPush(ctx, g.evidenceKey(driverID), raw)
	pipe.LTrim(ctx, g.evidenceKey(driverID), 0, GPSEvidencePairs-1)
	pipe.Expire(ctx, g.evidenceKey(driverID), GPSStrikeWindow)
	ev := pipe.LRange(ctx, g.evidenceKey(driverID), 0, GPSEvidencePairs-1)
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, nil, err
	}
	var pairs []PingPair
	for _, s := range ev.Val() {
		var p PingPair
		if json.Unmarshal([]byte(s), &p) == nil {
			pairs = append(pairs, p)
		}
	}
	return incr.Val(), pairs, nil
}
