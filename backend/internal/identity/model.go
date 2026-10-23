package identity

import (
	"net/http"
	"time"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
)

// Policy constants. Named so the rules are visible in one place.
const (
	OTPLength        = 6
	OTPTTL           = 5 * time.Minute
	OTPResendAfter   = 60 * time.Second
	OTPMaxAttempts   = 5
	RefreshTTL       = 30 * 24 * time.Hour
	RefreshTokenSize = 32 // bytes of entropy
)

// Role values match the user_role enum in the database.
const (
	RoleRider  = "RIDER"
	RoleDriver = "DRIVER"
	RoleStaff  = "STAFF"
)

// App identifies which client is signing in; it decides the role granted on
// first sign-in and is checked against the account on later sign-ins.
type App string

const (
	AppRider  App = "rider"
	AppDriver App = "driver"
	AppAdmin  App = "admin"
)

func (a App) dbValue() string {
	switch a {
	case AppRider:
		return "RIDER"
	case AppDriver:
		return "DRIVER"
	default:
		return "ADMIN"
	}
}

type User struct {
	ID       string
	BrandID  string
	Phone    string
	FullName *string
	Locale   *string
	Status   string
	Roles    []string
}

type OTP struct {
	ID        string
	Phone     string
	App       App
	CodeHash  string
	Attempts  int
	ExpiresAt time.Time
	Consumed  bool
}

type RefreshToken struct {
	ID        string
	UserID    string
	FamilyID  string
	ExpiresAt time.Time
	RevokedAt *time.Time
}

// TokenPair is what sign-in and refresh return to clients.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

// Error codes owned by this module (messages come from the i18n catalogue).
var (
	ErrOTPRateLimited  = apperr.New("OTP_RATE_LIMITED", http.StatusTooManyRequests, "")
	ErrOTPInvalid      = apperr.New("OTP_INVALID", http.StatusUnauthorized, "")
	ErrOTPLocked       = apperr.New("OTP_LOCKED", http.StatusTooManyRequests, "")
	ErrPhoneInvalid    = apperr.New("PHONE_INVALID", http.StatusBadRequest, "")
	ErrAppMismatch     = apperr.New("APP_MISMATCH", http.StatusForbidden, "")
	ErrRefreshInvalid  = apperr.New("REFRESH_INVALID", http.StatusUnauthorized, "")
	ErrAccountDisabled = apperr.New("ACCOUNT_DISABLED", http.StatusForbidden, "")
)
