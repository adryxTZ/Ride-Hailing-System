package realtime

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"

	"github.com/redis/go-redis/v9"
)

// sendBuffer is how many frames may queue for a slow client before it is dropped.
const sendBuffer = 32

// Hub tracks the sockets connected to this process and fans frames out through
// Redis so a Publish from any instance reaches the user wherever they are connected.
type Hub struct {
	redis *redis.Client
	brand string

	mu    sync.RWMutex
	conns map[string]map[*client]struct{} // user id → live sockets
}

type client struct {
	userID string
	send   chan []byte
}

func NewHub(r *redis.Client, brand string) *Hub {
	return &Hub{redis: r, brand: brand, conns: map[string]map[*client]struct{}{}}
}

func (h *Hub) channel(userID string) string { return "rt:" + h.brand + ":u:" + userID }

// Publish sends a frame to every socket of one user (any process). Errors are
// logged, never returned: realtime is best effort and HTTP stays the source of truth.
func (h *Hub) Publish(ctx context.Context, userID string, f Frame) {
	if h == nil || userID == "" {
		return
	}
	body, _ := json.Marshal(f)
	if err := h.redis.Publish(ctx, h.channel(userID), body).Err(); err != nil {
		slog.Warn("realtime publish failed", "err", err, "type", f.Type)
	}
}

// Run relays Redis messages to local sockets until ctx ends.
func (h *Hub) Run(ctx context.Context) {
	sub := h.redis.PSubscribe(ctx, "rt:"+h.brand+":u:*")
	defer func() { _ = sub.Close() }()
	prefix := "rt:" + h.brand + ":u:"
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-sub.Channel():
			if !ok {
				return
			}
			h.deliver(strings.TrimPrefix(msg.Channel, prefix), []byte(msg.Payload))
		}
	}
}

func (h *Hub) deliver(userID string, body []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.conns[userID] {
		select {
		case c.send <- body:
		default: // slow consumer: drop the frame; the client resyncs over HTTP
		}
	}
}

func (h *Hub) add(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.conns[c.userID] == nil {
		h.conns[c.userID] = map[*client]struct{}{}
	}
	h.conns[c.userID][c] = struct{}{}
}

func (h *Hub) remove(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.conns[c.userID], c)
	if len(h.conns[c.userID]) == 0 {
		delete(h.conns, c.userID)
	}
}

// Connected reports how many sockets this process holds (metrics, tests).
func (h *Hub) Connected() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	n := 0
	for _, set := range h.conns {
		n += len(set)
	}
	return n
}
