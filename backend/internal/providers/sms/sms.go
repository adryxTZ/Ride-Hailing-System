// Package sms defines the SMS provider port and a development implementation.
// Production providers (NextSMS, Africa's Talking, …) implement Sender in their
// own sub-packages and are selected per brand via brands/<code>.json providers.
package sms

import (
	"context"
	"log/slog"
)

// Sender delivers one text message. Implementations must be safe for
// concurrent use and must not retry internally (the job queue retries).
type Sender interface {
	Send(ctx context.Context, to, body string) error
}

// Log is the development sender: it writes the message to the log instead of
// sending it, so OTP codes are visible while testing without an SMS account.
type Log struct{}

func (Log) Send(_ context.Context, to, body string) error {
	slog.Info("sms (dev log sender)", "to", to, "body", body)
	return nil
}
