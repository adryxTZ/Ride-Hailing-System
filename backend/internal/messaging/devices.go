package messaging

import (
	"context"
	"log/slog"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
)

var validApps = map[string]bool{RoleRider: true, RoleDriver: true}
var validPlatforms = map[string]bool{"ANDROID": true, "IOS": true, "WEB": true}

// RegisterDevice binds a phone's push token to the signed-in user.
func (s *Service) RegisterDevice(ctx context.Context, userID string, in DeviceInput) error {
	if in.PushToken == "" || !validApps[in.App] || !validPlatforms[in.Platform] {
		return apperr.Validation
	}
	return s.store.upsertDevice(ctx, userID, in)
}

// UnregisterDevice removes a token (sign-out).
func (s *Service) UnregisterDevice(ctx context.Context, userID, token string) error {
	return s.store.deleteDevice(ctx, userID, token)
}

// push sends to every device of the user's app and forgets dead tokens. Never fails the caller.
func (s *Service) push(ctx context.Context, userID, app string, n Notification) {
	if s.pusher == nil || userID == "" {
		return
	}
	tokens, err := s.store.tokens(ctx, userID, app)
	if err != nil || len(tokens) == 0 {
		return
	}
	invalid, err := s.pusher.Push(ctx, tokens, n)
	if err != nil {
		slog.Warn("push failed", "user", userID, "err", err)
	}
	if len(invalid) > 0 {
		_ = s.store.deleteTokens(ctx, invalid)
	}
}
