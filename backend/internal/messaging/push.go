package messaging

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// ExpoPushURL is Expo's push service; it forwards to FCM / APNs for apps built
// with Expo and needs no server key for development builds and Expo Go.
const ExpoPushURL = "https://exp.host/--/api/v2/push/send"

const (
	expoBatch   = 100 // Expo accepts up to 100 messages per request
	pushTimeout = 10 * time.Second
)

// ExpoPusher sends through Expo's push API. AccessToken is optional (enhanced security).
type ExpoPusher struct {
	URL         string
	AccessToken string
	Client      *http.Client
}

// NewExpoPusher returns a pusher with sane defaults.
func NewExpoPusher(accessToken string) *ExpoPusher {
	return &ExpoPusher{URL: ExpoPushURL, AccessToken: accessToken, Client: &http.Client{Timeout: pushTimeout}}
}

type expoMessage struct {
	To        string            `json:"to"`
	Title     string            `json:"title"`
	Body      string            `json:"body"`
	Data      map[string]string `json:"data,omitempty"`
	Priority  string            `json:"priority,omitempty"`
	ChannelID string            `json:"channelId,omitempty"`
	Sound     string            `json:"sound,omitempty"`
}

type expoTicket struct {
	Status  string `json:"status"`
	Details struct {
		Error string `json:"error"`
	} `json:"details"`
}

// Push implements Pusher.
func (p *ExpoPusher) Push(ctx context.Context, tokens []string, n Notification) ([]string, error) {
	var invalid []string
	for start := 0; start < len(tokens); start += expoBatch {
		end := min(start+expoBatch, len(tokens))
		bad, err := p.send(ctx, tokens[start:end], n)
		if err != nil {
			return invalid, err
		}
		invalid = append(invalid, bad...)
	}
	return invalid, nil
}

func (p *ExpoPusher) send(ctx context.Context, tokens []string, n Notification) ([]string, error) {
	msgs := make([]expoMessage, len(tokens))
	for i, t := range tokens {
		msgs[i] = expoMessage{To: t, Title: n.Title, Body: n.Body, Data: n.Data, Priority: n.Priority, ChannelID: n.Channel, Sound: "default"}
	}
	body, _ := json.Marshal(msgs)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.URL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if p.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+p.AccessToken)
	}
	res, err := p.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("expo push: http %d", res.StatusCode)
	}
	var out struct {
		Data []expoTicket `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return nil, err
	}
	var invalid []string
	for i, t := range out.Data {
		if i < len(tokens) && t.Status == "error" && t.Details.Error == "DeviceNotRegistered" {
			invalid = append(invalid, tokens[i])
		}
	}
	return invalid, nil
}

// LogPusher only logs; used when push is switched off (NEXA_PUSH=off) and in tests.
type LogPusher struct{}

// Push implements Pusher.
func (LogPusher) Push(_ context.Context, tokens []string, n Notification) ([]string, error) {
	slog.Info("push (log only)", "devices", len(tokens), "title", n.Title, "body", n.Body)
	return nil, nil
}
