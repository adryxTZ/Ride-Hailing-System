package identity

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// store is the persistence layer. Every method is one query; rules live in service.go.
type store struct{ pool *pgxpool.Pool }

func (s store) latestOTP(ctx context.Context, q pgx.Tx, brandID, phone string, app App) (*OTP, error) {
	var o OTP
	var consumed *time.Time
	err := q.QueryRow(ctx, `
		SELECT id, phone, code_hash, attempts, expires_at, consumed_at
		FROM otp_codes WHERE brand_id=$1 AND phone=$2 AND app=$3
		ORDER BY created_at DESC LIMIT 1 FOR UPDATE`, brandID, phone, app.dbValue()).
		Scan(&o.ID, &o.Phone, &o.CodeHash, &o.Attempts, &o.ExpiresAt, &consumed)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	o.App, o.Consumed = app, consumed != nil
	return &o, nil
}

func (s store) lastOTPCreatedAt(ctx context.Context, q pgx.Tx, brandID, phone string) (*time.Time, error) {
	var t *time.Time
	err := q.QueryRow(ctx, `SELECT max(created_at) FROM otp_codes WHERE brand_id=$1 AND phone=$2`, brandID, phone).Scan(&t)
	return t, err
}

func (s store) insertOTP(ctx context.Context, q pgx.Tx, brandID, phone string, app App, codeHash string, expires time.Time) (string, error) {
	var id string
	err := q.QueryRow(ctx, `
		INSERT INTO otp_codes (brand_id, phone, app, code_hash, expires_at)
		VALUES ($1,$2,$3,$4,$5) RETURNING id`, brandID, phone, app.dbValue(), codeHash, expires).Scan(&id)
	return id, err
}

func (s store) bumpOTPAttempts(ctx context.Context, q pgx.Tx, id string) error {
	_, err := q.Exec(ctx, `UPDATE otp_codes SET attempts = attempts + 1 WHERE id=$1`, id)
	return err
}

func (s store) consumeOTP(ctx context.Context, q pgx.Tx, id string) error {
	_, err := q.Exec(ctx, `UPDATE otp_codes SET consumed_at = now() WHERE id=$1`, id)
	return err
}

func (s store) findUserByPhone(ctx context.Context, q pgx.Tx, brandID, phone string) (*User, error) {
	var u User
	err := q.QueryRow(ctx, `
		SELECT u.id, u.brand_id, u.phone, u.full_name, u.locale, u.status,
		       COALESCE(array_agg(r.role::text) FILTER (WHERE r.role IS NOT NULL), '{}')
		FROM users u LEFT JOIN user_roles r ON r.user_id = u.id
		WHERE u.brand_id=$1 AND u.phone=$2 AND u.deleted_at IS NULL
		GROUP BY u.id`, brandID, phone).
		Scan(&u.ID, &u.BrandID, &u.Phone, &u.FullName, &u.Locale, &u.Status, &u.Roles)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &u, err
}

func (s store) createUser(ctx context.Context, q pgx.Tx, brandID, phone, locale, role string) (*User, error) {
	var u User
	err := q.QueryRow(ctx, `INSERT INTO users (brand_id, phone, locale) VALUES ($1,$2,$3) RETURNING id, brand_id, phone, locale, status`,
		brandID, phone, locale).Scan(&u.ID, &u.BrandID, &u.Phone, &u.Locale, &u.Status)
	if err != nil {
		return nil, err
	}
	if _, err := q.Exec(ctx, `INSERT INTO user_roles (user_id, role) VALUES ($1,$2)`, u.ID, role); err != nil {
		return nil, err
	}
	u.Roles = []string{role}
	return &u, nil
}

func (s store) grantRole(ctx context.Context, q pgx.Tx, userID, role string) error {
	_, err := q.Exec(ctx, `INSERT INTO user_roles (user_id, role) VALUES ($1,$2) ON CONFLICT DO NOTHING`, userID, role)
	return err
}

func (s store) insertRefresh(ctx context.Context, q pgx.Tx, userID, familyID, hash string, expires time.Time) error {
	_, err := q.Exec(ctx, `INSERT INTO refresh_tokens (user_id, family_id, token_hash, expires_at) VALUES ($1,$2,$3,$4)`,
		userID, familyID, hash, expires)
	return err
}

func (s store) findRefresh(ctx context.Context, q pgx.Tx, hash string) (*RefreshToken, error) {
	var r RefreshToken
	err := q.QueryRow(ctx, `SELECT id, user_id, family_id, expires_at, revoked_at FROM refresh_tokens WHERE token_hash=$1 FOR UPDATE`, hash).
		Scan(&r.ID, &r.UserID, &r.FamilyID, &r.ExpiresAt, &r.RevokedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &r, err
}

func (s store) revokeRefresh(ctx context.Context, q pgx.Tx, id string) error {
	_, err := q.Exec(ctx, `UPDATE refresh_tokens SET revoked_at = now() WHERE id=$1 AND revoked_at IS NULL`, id)
	return err
}

func (s store) revokeFamily(ctx context.Context, q pgx.Tx, familyID string) error {
	_, err := q.Exec(ctx, `UPDATE refresh_tokens SET revoked_at = now() WHERE family_id=$1 AND revoked_at IS NULL`, familyID)
	return err
}

func (s store) userByID(ctx context.Context, q pgx.Tx, id string) (*User, error) {
	var u User
	err := q.QueryRow(ctx, `
		SELECT u.id, u.brand_id, u.phone, u.full_name, u.locale, u.status,
		       COALESCE(array_agg(r.role::text) FILTER (WHERE r.role IS NOT NULL), '{}')
		FROM users u LEFT JOIN user_roles r ON r.user_id = u.id
		WHERE u.id=$1 AND u.deleted_at IS NULL GROUP BY u.id`, id).
		Scan(&u.ID, &u.BrandID, &u.Phone, &u.FullName, &u.Locale, &u.Status, &u.Roles)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &u, err
}

// findStaffByEmail loads a user by dashboard email with their password hash and lock.
func (s store) findStaffByEmail(ctx context.Context, q pgx.Tx, brandID, email string) (*User, string, *time.Time, error) {
	var u User
	var hash *string
	var locked *time.Time
	err := q.QueryRow(ctx, `
		SELECT u.id, u.brand_id, u.phone, u.full_name, u.locale, u.status, u.password_hash, u.locked_until,
		       COALESCE(array_agg(r.role::text) FILTER (WHERE r.role IS NOT NULL), '{}')
		FROM users u LEFT JOIN user_roles r ON r.user_id = u.id
		WHERE u.brand_id=$1 AND u.email=$2 AND u.deleted_at IS NULL
		GROUP BY u.id`, brandID, email).
		Scan(&u.ID, &u.BrandID, &u.Phone, &u.FullName, &u.Locale, &u.Status, &hash, &locked, &u.Roles)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", nil, nil
	}
	if err != nil {
		return nil, "", nil, err
	}
	h := ""
	if hash != nil {
		h = *hash
	}
	return &u, h, locked, nil
}

// bumpLoginFailures counts a wrong password and locks the account at the limit.
func (s store) bumpLoginFailures(ctx context.Context, q pgx.Tx, userID string, lockUntil time.Time) error {
	_, err := q.Exec(ctx, `UPDATE users SET failed_logins = failed_logins + 1,
		locked_until = CASE WHEN failed_logins + 1 >= $2 THEN $3 ELSE locked_until END WHERE id=$1`, userID, LoginMaxFailures, lockUntil)
	return err
}

// startSession records a new sign-in session (one refresh-token family).
func (s store) startSession(ctx context.Context, q pgx.Tx, familyID, userID, app string, from *LoginContext) error {
	var ip, ua, device *string
	var loc any
	if from != nil {
		ip, ua, device = &from.IP, &from.UserAgent, &from.Device
		if from.Location != nil {
			loc = from.Location
		}
	}
	_, err := q.Exec(ctx, `INSERT INTO sessions (family_id, user_id, app, ip, user_agent, device, location) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		familyID, userID, app, ip, ua, device, loc)
	return err
}

// touchSession marks a session as used now (each token refresh).
func (s store) touchSession(ctx context.Context, q pgx.Tx, familyID string) error {
	_, err := q.Exec(ctx, `UPDATE sessions SET last_used_at=now() WHERE family_id=$1`, familyID)
	return err
}
