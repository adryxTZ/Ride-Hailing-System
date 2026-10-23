package identity

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/kalen-limited/hauler-rider/backend/internal/audit"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
)

// Bounds for the person page lists.
const (
	personNotesLimit   = 100
	personDevicesLimit = 20
	// NoteMaxLen matches the user_notes CHECK.
	NoteMaxLen = 2000
)

// ErrPersonNotFound is returned for an id that is not a rider or driver of this brand.
var ErrPersonNotFound = apperr.New("PERSON_NOT_FOUND", http.StatusNotFound, "")

// Device is a phone a person signed in on (push registration).
type Device struct {
	App        string    `json:"app"`
	Platform   string    `json:"platform"`
	AppVersion *string   `json:"app_version"`
	LastSeenAt time.Time `json:"last_seen_at"`
}

// Note is something staff wrote down about a person (support history).
type Note struct {
	ID         string    `json:"id"`
	Body       string    `json:"body"`
	AuthorID   string    `json:"author_id"`
	AuthorName *string   `json:"author_name"`
	CreatedAt  time.Time `json:"created_at"`
}

// PersonAccount is a rider's or driver's account as staff see it.
type PersonAccount struct {
	UserRow
	EmailVerified bool       `json:"email_verified"`
	PreferredLang *string    `json:"language"`
	LastSeenAt    *time.Time `json:"last_seen_at"` // latest session use
	Sessions      []Session  `json:"sessions"`
	Devices       []Device   `json:"devices"`
	Notes         []Note     `json:"notes"`
}

// PersonAccount returns one person (who holds role) with sessions, devices and staff notes.
func (s *Service) PersonAccount(ctx context.Context, userID, role string) (*PersonAccount, error) {
	var a PersonAccount
	err := s.pool.QueryRow(ctx, `
		SELECT u.id, u.full_name, u.phone, u.email::text, u.status::text,
		  COALESCE((SELECT array_agg(r.role::text) FROM user_roles r WHERE r.user_id=u.id), '{}'), u.created_at,
		  u.email_verified_at IS NOT NULL, u.locale, (SELECT max(last_used_at) FROM sessions WHERE user_id=u.id)
		FROM users u WHERE u.id=$1 AND u.brand_id=$2 AND u.deleted_at IS NULL
		  AND EXISTS (SELECT 1 FROM user_roles r WHERE r.user_id=u.id AND r.role::text=$3)`, userID, s.brandID, role).
		Scan(&a.ID, &a.Name, &a.Phone, &a.Email, &a.Status, &a.Roles, &a.CreatedAt, &a.EmailVerified, &a.PreferredLang, &a.LastSeenAt)
	if err == pgx.ErrNoRows {
		return nil, ErrPersonNotFound
	}
	if err != nil {
		return nil, err
	}
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if a.Sessions, err = s.sessionsOf(ctx, tx, userID); err != nil {
			return err
		}
		if a.Devices, err = devicesOf(ctx, tx, userID); err != nil {
			return err
		}
		a.Notes, err = notesOf(ctx, tx, userID)
		return err
	})
	return &a, err
}

func devicesOf(ctx context.Context, tx pgx.Tx, userID string) ([]Device, error) {
	rows, err := tx.Query(ctx, `SELECT app::text, platform, app_version, last_seen_at FROM devices WHERE user_id=$1 ORDER BY last_seen_at DESC LIMIT $2`,
		userID, personDevicesLimit)
	if err != nil {
		return nil, err
	}
	out, err := pgx.CollectRows(rows, pgx.RowToStructByPos[Device])
	if out == nil {
		out = []Device{}
	}
	return out, err
}

func notesOf(ctx context.Context, tx pgx.Tx, userID string) ([]Note, error) {
	rows, err := tx.Query(ctx, `SELECT n.id, n.body, n.author_id, u.full_name, n.created_at FROM user_notes n LEFT JOIN users u ON u.id=n.author_id
		WHERE n.user_id=$1 ORDER BY n.created_at DESC LIMIT $2`, userID, personNotesLimit)
	if err != nil {
		return nil, err
	}
	out, err := pgx.CollectRows(rows, pgx.RowToStructByPos[Note])
	if out == nil {
		out = []Note{}
	}
	return out, err
}

// AddNote stores a staff note on a person, audited (the note text stays in user_notes only).
func (s *Service) AddNote(ctx context.Context, staffID, userID, body string) (*Note, error) {
	body = strings.TrimSpace(body)
	if body == "" || len([]rune(body)) > NoteMaxLen {
		return nil, apperr.Validation.WithDetails(map[string]any{"fields": []map[string]string{{"path": "body", "code": "LENGTH"}}})
	}
	var n Note
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `INSERT INTO user_notes (brand_id, user_id, author_id, body)
			SELECT $1, u.id, $3, $4 FROM users u WHERE u.id=$2 AND u.brand_id=$1
			RETURNING id, body, author_id, (SELECT full_name FROM users WHERE id=$3), created_at`, s.brandID, userID, staffID, body).
			Scan(&n.ID, &n.Body, &n.AuthorID, &n.AuthorName, &n.CreatedAt)
		if err == pgx.ErrNoRows {
			return ErrPersonNotFound
		}
		if err != nil {
			return err
		}
		return audit.Record(ctx, tx, s.brandID, staffID, "person.note", "user:"+userID, "", map[string]any{"note": n.ID})
	})
	if err != nil {
		return nil, err
	}
	return &n, nil
}
