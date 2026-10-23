// Package config loads runtime configuration from the environment and fails
// fast on anything missing. Defaults never point at a shared environment.
package config

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
)

type Config struct {
	Brand         string // brand code selecting brands/<code>.json
	BrandsDir     string
	SeedDir       string
	HTTPAddr      string
	DatabaseURL   string // optional in phase 0; required once migrations exist
	RedisURL      string // optional in phase 0
	MinAppVersion string
	JWTKeysDir    string // Ed25519 signing key location
	I18nDir       string // message catalogues (<lang>.json)
	OTPDevEcho    bool   // echo OTP codes in responses — development only
	OTPTestCode   string // fixed code accepted for OTPTestNumbers (no SMS sent); empty = off
	OTPTestNums   string // comma-separated phone numbers that use OTPTestCode (empty with a code = every number, dev only)
	Log           string
	// Payments: which gateway collects money and where it calls back.
	PaymentGateway   string // "fake" (local/CI) or "selcom"; overrides the brand's providers.payment_gateway
	FakeGatewayDelay string // how long the fake gateway takes to settle, e.g. "5s"
	SelcomBaseURL    string
	SelcomAPIKey     string
	SelcomAPISecret  string
	SelcomVendor     string
	// Selcom Business API (payouts); payouts are off when any is missing.
	SelcomPayoutBaseURL string
	SelcomPayoutAPIKey  string
	SelcomPayoutKeyFile string // path of the RSA private key registered with Selcom
	// Transport regulator (LATRA): adapter fake | cets | pending, and the CeTS settings.
	LatraBaseURL      string
	LatraIssuer       string
	LatraAudience     string
	LatraKID          string
	LatraKeyFile      string
	LatraJWKSURI      string
	LatraTimeout      string
	FakeRegulatorMode string
	PaymentWebhookURL string // public API base for gateway callbacks, e.g. https://api.example.com
	features          string
	corsOrigins       string
}

func Load() (*Config, error) {
	c := &Config{
		Brand:               getenv("NEXA_BRAND", ""),
		BrandsDir:           getenv("NEXA_BRANDS_DIR", "./brands"),
		SeedDir:             getenv("NEXA_SEED_DIR", "./seed"),
		HTTPAddr:            getenv("NEXA_HTTP_ADDR", ":8080"),
		DatabaseURL:         getenv("NEXA_DATABASE_URL", ""),
		RedisURL:            getenv("NEXA_REDIS_URL", ""),
		MinAppVersion:       getenv("NEXA_MIN_APP_VERSION", "0.0.0"),
		JWTKeysDir:          getenv("NEXA_JWT_KEYS_DIR", "./.keys"),
		I18nDir:             getenv("NEXA_I18N_DIR", "./i18n"),
		OTPDevEcho:          getenv("NEXA_OTP_DEV_ECHO", "false") == "true",
		OTPTestCode:         getenv("NEXA_OTP_TEST_CODE", ""),
		OTPTestNums:         getenv("NEXA_OTP_TEST_NUMBERS", ""),
		Log:                 getenv("NEXA_LOG_LEVEL", "info"),
		PaymentGateway:      getenv("NEXA_PAYMENT_GATEWAY", ""),
		FakeGatewayDelay:    getenv("NEXA_FAKE_GATEWAY_DELAY", "5s"),
		SelcomBaseURL:       getenv("NEXA_SELCOM_BASE_URL", ""),
		SelcomAPIKey:        getenv("NEXA_SELCOM_API_KEY", ""),
		SelcomAPISecret:     getenv("NEXA_SELCOM_API_SECRET", ""),
		SelcomVendor:        getenv("NEXA_SELCOM_VENDOR", ""),
		SelcomPayoutBaseURL: getenv("NEXA_SELCOM_PAYOUT_BASE_URL", ""),
		SelcomPayoutAPIKey:  getenv("NEXA_SELCOM_PAYOUT_API_KEY", ""),
		SelcomPayoutKeyFile: getenv("NEXA_SELCOM_PAYOUT_KEY_FILE", ""),
		LatraBaseURL:        getenv("NEXA_REGULATOR_LATRA_BASE_URL", ""),
		LatraIssuer:         getenv("NEXA_REGULATOR_LATRA_JWT_ISSUER", ""),
		LatraAudience:       getenv("NEXA_REGULATOR_LATRA_JWT_AUDIENCE", "cets"),
		LatraKID:            getenv("NEXA_REGULATOR_LATRA_JWT_KID", ""),
		LatraKeyFile:        getenv("NEXA_REGULATOR_LATRA_JWT_KEY_FILE", ""),
		LatraJWKSURI:        getenv("NEXA_REGULATOR_LATRA_JWKS_URI", ""),
		LatraTimeout:        getenv("NEXA_REGULATOR_LATRA_TIMEOUT", "15s"),
		FakeRegulatorMode:   getenv("NEXA_FAKE_REGULATOR_MODE", "ok"),
		PaymentWebhookURL:   getenv("NEXA_PAYMENT_WEBHOOK_BASE_URL", ""),
		features:            getenv("NEXA_FEATURES", ""),
		corsOrigins:         getenv("NEXA_CORS_ORIGINS", ""),
	}
	var missing []string
	if c.Brand == "" {
		missing = append(missing, "NEXA_BRAND")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required environment: %s", strings.Join(missing, ", "))
	}
	return c, nil
}

func (c *Config) LogLevel() slog.Level {
	switch strings.ToLower(c.Log) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// Features parses NEXA_FEATURES="card_payments,scheduled_trips" into flags.
func (c *Config) Features() map[string]bool {
	out := map[string]bool{}
	for _, f := range strings.Split(c.features, ",") {
		if f = strings.TrimSpace(f); f != "" {
			out[f] = true
		}
	}
	return out
}

// CORSOrigins parses NEXA_CORS_ORIGINS ("*" or a comma-separated list).
func (c *Config) CORSOrigins() []string {
	var out []string
	for _, o := range strings.Split(c.corsOrigins, ",") {
		if o = strings.TrimSpace(o); o != "" {
			out = append(out, o)
		}
	}
	return out
}

// OTPTestNumbers returns the normalised allow-list for the fixed test code.
func (c *Config) OTPTestNumbers() []string {
	var out []string
	for _, n := range strings.Split(c.OTPTestNums, ",") {
		if n = strings.TrimSpace(n); n != "" {
			out = append(out, n)
		}
	}
	return out
}

func getenv(k, def string) string {
	if v, ok := os.LookupEnv(k); ok && v != "" {
		return v
	}
	return def
}
