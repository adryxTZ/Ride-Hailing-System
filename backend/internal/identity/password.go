package identity

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/kalen-limited/hauler-rider/backend/internal/audit"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
)

// DeviceFromUserAgent turns a browser user agent into "Browser on OS" for the audit log.
func DeviceFromUserAgent(ua string) string {
	if ua == "" {
		return "unknown device"
	}
	browser := "Browser"
	switch {
	case strings.Contains(ua, "Edg/"):
		browser = "Edge"
	case strings.Contains(ua, "OPR/"):
		browser = "Opera"
	case strings.Contains(ua, "Chrome/"):
		browser = "Chrome"
	case strings.Contains(ua, "Firefox/"):
		browser = "Firefox"
	case strings.Contains(ua, "Safari/"):
		browser = "Safari"
	}
	os := "unknown device"
	switch {
	case strings.Contains(ua, "iPhone"):
		os = "iPhone"
	case strings.Contains(ua, "iPad"):
		os = "iPad"
	case strings.Contains(ua, "Android"):
		os = "Android"
	case strings.Contains(ua, "Windows"):
		os = "Windows"
	case strings.Contains(ua, "Mac OS X"):
		os = "macOS"
	case strings.Contains(ua, "Linux"):
		os = "Linux"
	}
	return browser + " on " + os
}

// Password sign-in rules for dashboard staff.
const (
	PasswordMinLen   = 6
	LoginMaxFailures = 5                // wrong passwords before the account locks
	LoginLockFor     = 10 * time.Minute // how long it stays locked
	bcryptCost       = 12
)

// Error codes for password sign-in.
var (
	ErrInvalidCredentials = apperr.New("INVALID_CREDENTIALS", http.StatusUnauthorized, "")
	ErrLoginLocked        = apperr.New("LOGIN_LOCKED", http.StatusTooManyRequests, "")
	ErrPasswordTooShort   = apperr.New("PASSWORD_TOO_SHORT", http.StatusBadRequest, "")
	ErrEmailInvalid       = apperr.New("EMAIL_INVALID", http.StatusBadRequest, "")
	ErrEmailTaken         = apperr.New("EMAIL_TAKEN", http.StatusConflict, "")
)

// SetStaffCredentials gives a staff account its dashboard email and password.
// The password is stored as a bcrypt hash; an empty password keeps the current one.
func (s *Service) SetStaffCredentials(ctx context.Context, userID, email, password string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if email != "" && (!strings.Contains(email, "@") || strings.ContainsAny(email, " \t")) {
		return ErrEmailInvalid
	}
	if password != "" && len(password) < PasswordMinLen {
		return ErrPasswordTooShort
	}
	var hash *string
	if password != "" {
		h, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
		if err != nil {
			return err
		}
		hs := string(h)
		hash = &hs
	}
	_, err := s.pool.Exec(ctx, `UPDATE users SET email = COALESCE(NULLIF($2,''), email), password_hash = COALESCE($3, password_hash),
		failed_logins = 0, locked_until = NULL, updated_at = now() WHERE id=$1`, userID, email, hash)
	if err != nil && strings.Contains(err.Error(), "users_brand_email_uq") {
		return ErrEmailTaken
	}
	return err
}

// LoginContext is where a sign-in came from, kept in the audit log.
type LoginContext struct {
	IP        string         `json:"ip"`
	UserAgent string         `json:"user_agent"`
	Device    string         `json:"device"`             // "Chrome on macOS", derived from the user agent
	Location  *LoginLocation `json:"location,omitempty"` // from the browser, only when the person allowed it
}

// LoginLocation is where the browser said it was at sign-in.
type LoginLocation struct {
	Lat       float64 `json:"lat"`
	Lng       float64 `json:"lng"`
	AccuracyM float64 `json:"accuracy_m"`
}

// valid rejects impossible coordinates so the audit never stores junk.
func (l *LoginLocation) valid() bool {
	return l != nil && l.Lat >= -90 && l.Lat <= 90 && l.Lng >= -180 && l.Lng <= 180 && l.AccuracyM >= 0 && !(l.Lat == 0 && l.Lng == 0)
}

// LoginWithPassword signs a staff member into the dashboard. Wrong passwords
// are counted; after LoginMaxFailures the account locks for LoginLockFor.
// Unknown emails and wrong passwords get the same answer. Every outcome for a
// known account is written to the audit log with the device and address.
func (s *Service) LoginWithPassword(ctx context.Context, email, password string, from LoginContext) (*TokenPair, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	from.Device = DeviceFromUserAgent(from.UserAgent)
	if !from.Location.valid() {
		from.Location = nil
	}
	var pair *TokenPair
	var failedUser string
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		u, hash, lockedUntil, err := s.store.findStaffByEmail(ctx, tx, s.brandID, email)
		if err != nil {
			return err
		}
		if u == nil || hash == "" {
			return ErrInvalidCredentials
		}
		now := s.clock.Now()
		if lockedUntil != nil && now.Before(*lockedUntil) {
			return ErrLoginLocked
		}
		if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
			failedUser = u.ID
			return ErrInvalidCredentials
		}
		if u.Status != "ACTIVE" {
			return ErrAccountDisabled
		}
		if !hasRole(u, RoleStaff) {
			return ErrAppMismatch
		}
		if _, err := tx.Exec(ctx, `UPDATE users SET failed_logins=0, locked_until=NULL WHERE id=$1`, u.ID); err != nil {
			return err
		}
		if err := audit.Record(ctx, tx, s.brandID, u.ID, "staff.login", "user:"+u.ID, "", from); err != nil {
			return err
		}
		pair, err = s.issueTokensWith(ctx, tx, u, AppAdmin, &from)
		return err
	})
	if failedUser != "" {
		// The failed transaction rolled back; the attempt is counted on its own.
		s.commitSideEffect(ctx, func(tx pgx.Tx) error {
			if err := s.store.bumpLoginFailures(ctx, tx, failedUser, s.clock.Now().Add(LoginLockFor)); err != nil {
				return err
			}
			return audit.Record(ctx, tx, s.brandID, failedUser, "staff.login_failed", "user:"+failedUser, "", from)
		})
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrInvalidCredentials
	}
	return pair, err
}
