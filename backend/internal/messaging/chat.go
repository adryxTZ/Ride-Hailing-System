package messaging

import (
	"context"
	"strings"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
)

// FrameChat is the realtime frame type for a new chat line.
const FrameChat = "chat.message"

// pushPreviewLen is how much of a chat line shows in the notification.
const pushPreviewLen = 120

// Send stores a chat line from a participant and delivers it to the other side
// (socket now, push for a phone in the pocket). Chat opens once a driver is
// assigned and closes when the trip ends.
func (s *Service) Send(ctx context.Context, tripID, userID string, in SendInput) (*Message, error) {
	in.Body = strings.TrimSpace(in.Body)
	if in.Body == "" || len([]rune(in.Body)) > MaxMessageLen {
		return nil, apperr.Validation.WithDetails(map[string]any{"fields": []map[string]string{{"path": "body", "code": "INVALID"}}})
	}
	t, role, err := s.participant(ctx, tripID, userID)
	if err != nil {
		return nil, err
	}
	if t.DriverID == "" || t.Terminal {
		return nil, ErrChatClosed
	}
	m, err := s.store.insertMessage(ctx, tripID, userID, role, in)
	if err != nil {
		return nil, err
	}
	to, app := other(t, role)
	if s.realtime != nil {
		s.realtime.Send(ctx, to, FrameChat, m)
	}
	s.push(ctx, to, app, Notification{
		Title:   "New message",
		Body:    preview(m.Body),
		Data:    map[string]string{"type": "chat", "trip_id": tripID},
		Channel: "chat",
	})
	return m, nil
}

// List returns the trip's chat for a participant and marks the other side's lines read.
func (s *Service) List(ctx context.Context, tripID, userID string) ([]Message, error) {
	if _, _, err := s.participant(ctx, tripID, userID); err != nil {
		return nil, err
	}
	if _, err := s.store.markRead(ctx, tripID, userID); err != nil {
		return nil, err
	}
	return s.store.messages(ctx, tripID)
}

// QuickReplies are the canned lines for a role.
func QuickReplies(role string) []QuickReply {
	if role == RoleDriver {
		return driverReplies
	}
	return riderReplies
}

func (s *Service) participant(ctx context.Context, tripID, userID string) (*TripView, string, error) {
	t, err := s.trips.View(ctx, tripID)
	if err != nil {
		return nil, "", err
	}
	if t == nil {
		return nil, "", apperr.NotFound
	}
	role, ok := roleIn(t, userID)
	if !ok {
		return nil, "", ErrNotParticipant
	}
	return t, role, nil
}

func preview(body string) string {
	r := []rune(body)
	if len(r) <= pushPreviewLen {
		return body
	}
	return string(r[:pushPreviewLen]) + "…"
}

var riderReplies = []QuickReply{
	{Key: "on_my_way", Text: map[string]string{"en": "I'm on my way", "sw": "Niko njiani"}},
	{Key: "at_pickup", Text: map[string]string{"en": "I'm at the pickup point", "sw": "Niko mahali pa kuchukuliwa"}},
	{Key: "call_me", Text: map[string]string{"en": "Please call me", "sw": "Tafadhali nipigie"}},
	{Key: "where_are_you", Text: map[string]string{"en": "Where are you?", "sw": "Uko wapi?"}},
}

var driverReplies = []QuickReply{
	{Key: "on_my_way", Text: map[string]string{"en": "I'm on my way", "sw": "Niko njiani"}},
	{Key: "arrived", Text: map[string]string{"en": "I've arrived", "sw": "Nimefika"}},
	{Key: "traffic", Text: map[string]string{"en": "Stuck in traffic, a few minutes", "sw": "Nimekwama kwenye foleni, dakika chache"}},
	{Key: "come_out", Text: map[string]string{"en": "Please come outside", "sw": "Tafadhali toka nje"}},
}
