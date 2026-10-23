package messaging

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type store struct {
	pool    *pgxpool.Pool
	brandID string
}

func (s store) insertMessage(ctx context.Context, tripID, senderID, role string, in SendInput) (*Message, error) {
	m := Message{TripID: tripID, SenderID: senderID, SenderRole: role, Body: in.Body, QuickReply: in.QuickReply}
	err := s.pool.QueryRow(ctx, `
		INSERT INTO trip_messages (brand_id, trip_id, sender_id, sender_role, body, quick_reply)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING id, created_at`,
		s.brandID, tripID, senderID, role, in.Body, in.QuickReply).Scan(&m.ID, &m.CreatedAt)
	return &m, err
}

func (s store) messages(ctx context.Context, tripID string) ([]Message, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, trip_id, sender_id, sender_role, body, quick_reply, created_at, read_at
		FROM trip_messages WHERE trip_id=$1 ORDER BY created_at LIMIT $2`, tripID, ChatHistoryLimit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Message{}
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.TripID, &m.SenderID, &m.SenderRole, &m.Body, &m.QuickReply, &m.CreatedAt, &m.ReadAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// markRead stamps every unread line the other side sent; returns how many.
func (s store) markRead(ctx context.Context, tripID, readerID string) (int64, error) {
	tag, err := s.pool.Exec(ctx, `
		UPDATE trip_messages SET read_at=now()
		WHERE trip_id=$1 AND sender_id<>$2 AND read_at IS NULL`, tripID, readerID)
	return tag.RowsAffected(), err
}

func (s store) insertRating(ctx context.Context, r Rating, raterID, rateeID string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO trip_ratings (trip_id, rater_role, rater_id, ratee_id, score, aspects, comment)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		r.TripID, r.RaterRole, raterID, rateeID, r.Score, r.Aspects, r.Comment)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrAlreadyRated
	}
	return err
}

func (s store) ratings(ctx context.Context, tripID string) ([]Rating, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT trip_id, rater_role, score, aspects, comment, created_at FROM trip_ratings WHERE trip_id=$1`, tripID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Rating{}
	for rows.Next() {
		var r Rating
		if err := rows.Scan(&r.TripID, &r.RaterRole, &r.Score, &r.Aspects, &r.Comment, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s store) summary(ctx context.Context, userID string) (Summary, error) {
	var sm Summary
	err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(avg(score), 0)::float8, count(*) FROM trip_ratings WHERE ratee_id=$1`, userID).
		Scan(&sm.Average, &sm.Count)
	return sm, err
}

// upsertDevice binds a push token to the user (a token moves with sign-ins on a shared phone).
func (s store) upsertDevice(ctx context.Context, userID string, in DeviceInput) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO devices (user_id, app, platform, push_token, app_version, last_seen_at)
		VALUES ($1, $2::app_kind, $3, $4, $5, now())
		ON CONFLICT (push_token) DO UPDATE SET user_id=EXCLUDED.user_id, app=EXCLUDED.app,
		  platform=EXCLUDED.platform, app_version=EXCLUDED.app_version, last_seen_at=now()`,
		userID, in.App, in.Platform, in.PushToken, in.AppVersion)
	return err
}

func (s store) deleteDevice(ctx context.Context, userID, token string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM devices WHERE user_id=$1 AND push_token=$2`, userID, token)
	return err
}

func (s store) deleteTokens(ctx context.Context, tokens []string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM devices WHERE push_token = ANY($1)`, tokens)
	return err
}

// tokens returns the user's push tokens for one app.
func (s store) tokens(ctx context.Context, userID, app string) ([]string, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT push_token FROM devices WHERE user_id=$1 AND app=$2::app_kind AND push_token IS NOT NULL`, userID, app)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
