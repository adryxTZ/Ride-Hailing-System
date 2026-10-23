package identity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log/slog"
	"math/big"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/auth"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/phone"
)

// Service implements the identity use cases. It is constructed once in main
// and shared by the HTTP layer and the job workers.
type Service struct {
	pool    *pgxpool.Pool
	store   store
	issuer  TokenIssuer
	jobs    JobEnqueuer
	clock   Clock
	brandID string
	brand   string // brand code, placed in the token
	country string
	locale  string
	// DevEcho returns the OTP code in the request response. Development only.
	DevEcho bool
	// TestCode is accepted for TestNumbers instead of a random code, and no SMS
	// is sent for them (used for QA devices and app-store review). When
	// TestNumbers is empty the test code applies to every number: development only.
	TestCode    string
	TestNumbers map[string]bool
}

// New wires the service. brandID is the database id of the brand this process serves.
func New(pool *pgxpool.Pool, issuer TokenIssuer, jobs JobEnqueuer, brandID, brandCode, country, locale string) *Service {
	return &Service{pool: pool, store: store{pool: pool}, issuer: issuer, jobs: jobs, clock: realClock{},
		brandID: brandID, brand: brandCode, country: country, locale: locale}
}

// RequestOTP creates a one-time code for the phone and enqueues the SMS in the
// same transaction. The second return value is the code itself, only when DevEcho is on.
func (s *Service) RequestOTP(ctx context.Context, rawPhone string, app App) (string, error) {
	normalised, err := phone.Normalize(rawPhone, s.country)
	if err != nil {
		return "", ErrPhoneInvalid
	}
	isTest := s.isTestNumber(normalised)
	code, err := randomDigits(OTPLength)
	if err != nil {
		return "", err
	}
	if isTest {
		code = s.TestCode
	}
	now := s.clock.Now()
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if !isTest {
			last, err := s.store.lastOTPCreatedAt(ctx, tx, s.brandID, normalised)
			if err != nil {
				return err
			}
			if last != nil && now.Sub(*last) < OTPResendAfter {
				return ErrOTPRateLimited
			}
		}
		id, err := s.store.insertOTP(ctx, tx, s.brandID, normalised, app, hashOTP(code, normalised), now.Add(OTPTTL))
		if err != nil {
			return err
		}
		if s.jobs != nil && !isTest {
			_, err = s.jobs.InsertTx(ctx, tx, SendOTPArgs{OTPID: id, Phone: normalised, Code: code}, nil)
		}
		return err
	})
	if err != nil {
		return "", err
	}
	if s.DevEcho {
		return code, nil
	}
	return "", nil
}

// VerifyOTP checks the code, signs the user in (creating the account on first
// use) and returns a token pair. Attempts are counted so codes cannot be brute-forced.
func (s *Service) VerifyOTP(ctx context.Context, rawPhone, code string, app App) (*TokenPair, error) {
	normalised, err := phone.Normalize(rawPhone, s.country)
	if err != nil {
		return nil, ErrPhoneInvalid
	}
	var pair *TokenPair
	var wrongCodeOTPID string // set when the code mismatched; counted after the tx
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		otp, err := s.store.latestOTP(ctx, tx, s.brandID, normalised, app)
		if err != nil {
			return err
		}
		switch {
		case otp == nil || otp.Consumed || s.clock.Now().After(otp.ExpiresAt):
			return ErrOTPInvalid
		case otp.Attempts >= OTPMaxAttempts:
			return ErrOTPLocked
		case otp.CodeHash != hashOTP(code, normalised):
			wrongCodeOTPID = otp.ID
			return ErrOTPInvalid
		}
		if err := s.store.consumeOTP(ctx, tx, otp.ID); err != nil {
			return err
		}
		user, err := s.userForSignIn(ctx, tx, normalised, app)
		if err != nil {
			return err
		}
		pair, err = s.issueTokens(ctx, tx, user, app)
		return err
	})
	if wrongCodeOTPID != "" {
		// The failed transaction above rolled back; the attempt must still be
		// counted, so it is written in its own transaction.
		s.commitSideEffect(ctx, func(tx pgx.Tx) error { return s.store.bumpOTPAttempts(ctx, tx, wrongCodeOTPID) })
	}
	return pair, err
}

// userForSignIn finds or creates the account and enforces app/role fit.
func (s *Service) userForSignIn(ctx context.Context, tx pgx.Tx, phone string, app App) (*User, error) {
	role := roleForApp(app)
	user, err := s.store.findUserByPhone(ctx, tx, s.brandID, phone)
	if err != nil {
		return nil, err
	}
	if user != nil && user.Status != "ACTIVE" {
		return nil, ErrAccountDisabled // blocked by staff: no new sessions
	}
	if user == nil {
		if app == AppAdmin {
			return nil, ErrAppMismatch // staff accounts are created by an admin, never by OTP
		}
		return s.store.createUser(ctx, tx, s.brandID, phone, s.locale, role)
	}
	if app == AppAdmin && !hasRole(user, RoleStaff) {
		return nil, ErrAppMismatch
	}
	if !hasRole(user, role) {
		// A rider may later become a driver (and vice versa); grant the role.
		if err := s.store.grantRole(ctx, tx, user.ID, role); err != nil {
			return nil, err
		}
		user.Roles = append(user.Roles, role)
	}
	return user, nil
}

// Refresh rotates a refresh token. Reuse of an already-rotated token revokes
// the whole family: that is the signature of a stolen token.
func (s *Service) Refresh(ctx context.Context, rawRefresh string, app App) (*TokenPair, error) {
	var pair *TokenPair
	var reusedFamily string // set on reuse detection; revoked after the tx
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		rt, err := s.store.findRefresh(ctx, tx, hashToken(rawRefresh))
		if err != nil {
			return err
		}
		if rt == nil || s.clock.Now().After(rt.ExpiresAt) {
			return ErrRefreshInvalid
		}
		if rt.RevokedAt != nil {
			reusedFamily = rt.FamilyID
			return ErrRefreshInvalid
		}
		if err := s.store.revokeRefresh(ctx, tx, rt.ID); err != nil {
			return err
		}
		user, err := s.store.userByID(ctx, tx, rt.UserID)
		if err != nil || user == nil || user.Status != "ACTIVE" {
			return ErrRefreshInvalid // a blocked account's sessions end at the next refresh
		}
		if err := s.store.touchSession(ctx, tx, rt.FamilyID); err != nil {
			return err
		}
		pair, err = s.issueTokensInFamily(ctx, tx, user, app, rt.FamilyID)
		return err
	})
	if reusedFamily != "" {
		// Reuse of a rotated token means it leaked: kill every session in the family.
		s.commitSideEffect(ctx, func(tx pgx.Tx) error { return s.store.revokeFamily(ctx, tx, reusedFamily) })
	}
	return pair, err
}

// commitSideEffect runs fn in its own transaction so it survives the rollback
// of a failed business operation. Failures are logged, never surfaced: the
// caller's error is the one the user must see.
func (s *Service) commitSideEffect(ctx context.Context, fn func(pgx.Tx) error) {
	if err := pgx.BeginFunc(ctx, s.pool, fn); err != nil {
		slog.Error("identity side effect failed", "err", err)
	}
}

// Logout revokes the session family the refresh token belongs to.
func (s *Service) Logout(ctx context.Context, rawRefresh string) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		rt, err := s.store.findRefresh(ctx, tx, hashToken(rawRefresh))
		if err != nil || rt == nil {
			return err
		}
		return s.store.revokeFamily(ctx, tx, rt.FamilyID)
	})
}

func (s *Service) issueTokens(ctx context.Context, tx pgx.Tx, u *User, app App) (*TokenPair, error) {
	return s.issueTokensWith(ctx, tx, u, app, nil)
}

// issueTokensWith starts a new sign-in session (recorded with where it came from) and issues its first tokens.
func (s *Service) issueTokensWith(ctx context.Context, tx pgx.Tx, u *User, app App, from *LoginContext) (*TokenPair, error) {
	family := newID()
	if err := s.store.startSession(ctx, tx, family, u.ID, string(app), from); err != nil {
		return nil, err
	}
	return s.issueTokensInFamily(ctx, tx, u, app, family)
}

func (s *Service) issueTokensInFamily(ctx context.Context, tx pgx.Tx, u *User, app App, family string) (*TokenPair, error) {
	now := s.clock.Now()
	roles := u.Roles
	if app == AppAdmin { // the dashboard needs the staff roles, their permissions and any country/region limit
		extra, err := s.staffClaims(ctx, tx, u.ID)
		if err != nil {
			return nil, err
		}
		roles = append(append([]string{}, u.Roles...), extra...)
	}
	access, err := s.issuer.Issue(u.ID, s.brand, string(app), family, roles, now)
	if err != nil {
		return nil, err
	}
	raw, err := randomToken()
	if err != nil {
		return nil, err
	}
	if err := s.store.insertRefresh(ctx, tx, u.ID, family, hashToken(raw), now.Add(RefreshTTL)); err != nil {
		return nil, err
	}
	return &TokenPair{AccessToken: access, RefreshToken: raw, ExpiresIn: int(auth.AccessTTL.Seconds())}, nil
}

// isTestNumber reports whether the fixed test code applies to this phone.
func (s *Service) isTestNumber(normalised string) bool {
	if s.TestCode == "" {
		return false
	}
	if len(s.TestNumbers) == 0 {
		return true
	}
	return s.TestNumbers[normalised]
}

// --- helpers -------------------------------------------------------------

func roleForApp(app App) string {
	switch app {
	case AppDriver:
		return RoleDriver
	case AppAdmin:
		return RoleStaff
	default:
		return RoleRider
	}
}

func hasRole(u *User, role string) bool {
	for _, r := range u.Roles {
		if r == role {
			return true
		}
	}
	return false
}

func hashOTP(code, phone string) string {
	sum := sha256.Sum256([]byte(phone + ":" + code))
	return hex.EncodeToString(sum[:])
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func randomToken() (string, error) {
	b := make([]byte, RefreshTokenSize)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func randomDigits(n int) (string, error) {
	out := make([]byte, n)
	for i := range out {
		v, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		out[i] = byte('0' + v.Int64())
	}
	return string(out), nil
}

func newID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
