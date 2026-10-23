package realtime

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/coder/websocket"
)

const (
	authTimeout  = 10 * time.Second
	pingInterval = 20 * time.Second
	writeTimeout = 10 * time.Second
	readLimit    = 4 << 10 // clients only send small control frames
)

// Verifier checks an access token and returns the user id (subject) it was issued to.
type Verifier func(token string) (userID string, err error)

// Handler upgrades GET /v1/ws. origins are the allowed browser origins ("*" = any);
// native apps send no Origin header and are always accepted.
func (h *Hub) Handler(verify Verifier, origins []string) http.HandlerFunc {
	opts := &websocket.AcceptOptions{OriginPatterns: origins}
	for _, o := range origins {
		if o == "*" {
			opts.InsecureSkipVerify = true
		}
	}
	return func(w http.ResponseWriter, r *http.Request) {
		ws, err := websocket.Accept(w, r, opts)
		if err != nil {
			return
		}
		ws.SetReadLimit(readLimit)
		defer func() { _ = ws.CloseNow() }()
		userID, ok := authenticate(r.Context(), ws, verify)
		if !ok {
			_ = ws.Close(websocket.StatusPolicyViolation, "auth required")
			return
		}
		c := &client{userID: userID, send: make(chan []byte, sendBuffer)}
		h.add(c)
		defer h.remove(c)
		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()
		go readLoop(ctx, cancel, ws, c)
		writeLoop(ctx, ws, c)
	}
}

// authenticate waits for the first frame and checks its token.
func authenticate(ctx context.Context, ws *websocket.Conn, verify Verifier) (string, bool) {
	actx, cancel := context.WithTimeout(ctx, authTimeout)
	defer cancel()
	_, data, err := ws.Read(actx)
	if err != nil {
		return "", false
	}
	var f clientFrame
	if json.Unmarshal(data, &f) != nil || f.Type != "auth" || f.Token == "" {
		return "", false
	}
	userID, err := verify(f.Token)
	if err != nil || userID == "" {
		return "", false
	}
	ready, _ := json.Marshal(NewFrame(TypeReady, map[string]string{"user_id": userID}))
	return userID, write(ctx, ws, ready) == nil
}

// readLoop answers pings and ends the connection when the client goes away.
func readLoop(ctx context.Context, cancel context.CancelFunc, ws *websocket.Conn, c *client) {
	defer cancel()
	for {
		_, data, err := ws.Read(ctx)
		if err != nil {
			return
		}
		var f clientFrame
		if json.Unmarshal(data, &f) == nil && f.Type == "ping" {
			pong, _ := json.Marshal(NewFrame(TypePong, nil))
			select {
			case c.send <- pong:
			default:
			}
		}
	}
}

// writeLoop forwards queued frames and pings the client so dead sockets close.
func writeLoop(ctx context.Context, ws *websocket.Conn, c *client) {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case body := <-c.send:
			if write(ctx, ws, body) != nil {
				return
			}
		case <-ticker.C:
			pctx, cancel := context.WithTimeout(ctx, writeTimeout)
			err := ws.Ping(pctx)
			cancel()
			if err != nil {
				return
			}
		}
	}
}

func write(ctx context.Context, ws *websocket.Conn, body []byte) error {
	wctx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	return ws.Write(wctx, websocket.MessageText, body)
}
