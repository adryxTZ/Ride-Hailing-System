package messaging

import (
	"net/http"
	"time"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
)

// Role of a trip participant.
const (
	RoleRider  = "RIDER"
	RoleDriver = "DRIVER"
)

// Message is one chat line on a trip.
type Message struct {
	ID         string     `json:"id"`
	TripID     string     `json:"trip_id"`
	SenderID   string     `json:"sender_id"`
	SenderRole string     `json:"sender_role"`
	Body       string     `json:"body"`
	QuickReply *string    `json:"quick_reply,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	ReadAt     *time.Time `json:"read_at,omitempty"`
}

// SendInput is a new chat line; QuickReply is the key of a canned reply when used.
type SendInput struct {
	Body       string  `json:"body"`
	QuickReply *string `json:"quick_reply"`
}

// Rating is one side's verdict on the other after a completed trip.
type Rating struct {
	TripID    string    `json:"trip_id"`
	RaterRole string    `json:"rater_role"`
	Score     int       `json:"score"`
	Aspects   []string  `json:"aspects"`
	Comment   *string   `json:"comment,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// RateInput is what the app sends.
type RateInput struct {
	Score   int      `json:"score"`
	Aspects []string `json:"aspects"`
	Comment *string  `json:"comment"`
}

// Summary is a person's rating as others see it.
type Summary struct {
	Average float64 `json:"average"`
	Count   int     `json:"count"`
}

// DeviceInput registers a phone for push.
type DeviceInput struct {
	App        string `json:"app"`      // RIDER | DRIVER
	Platform   string `json:"platform"` // ANDROID | IOS | WEB
	PushToken  string `json:"push_token"`
	AppVersion string `json:"app_version"`
}

// QuickReply is a canned chat line shown as a chip.
type QuickReply struct {
	Key  string            `json:"key"`
	Text map[string]string `json:"text"`
}

// Limits.
const (
	MaxMessageLen = 500
	MaxCommentLen = 500
	MinScore      = 1
	MaxScore      = 5
	// ChatHistoryLimit bounds one page of chat.
	ChatHistoryLimit = 200
)

var (
	ErrChatClosed     = apperr.New("CHAT_CLOSED", http.StatusConflict, "")
	ErrNotRateable    = apperr.New("TRIP_NOT_RATEABLE", http.StatusConflict, "")
	ErrAlreadyRated   = apperr.New("ALREADY_RATED", http.StatusConflict, "")
	ErrNotParticipant = apperr.New("FORBIDDEN", http.StatusForbidden, "")
)
