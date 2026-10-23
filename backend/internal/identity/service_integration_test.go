package identity

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/auth"
)

// Integration tests run against a real Postgres when NEXA_TEST_DATABASE_URL is
// set (locally: the compose database). CI provides one via services.
func testPool(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()
	url := os.Getenv("NEXA_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("NEXA_TEST_DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	var brandID string
	if err := pool.QueryRow(context.Background(), `SELECT id FROM brands ORDER BY created_at LIMIT 1`).Scan(&brandID); err != nil {
		t.Skip("no seeded brand: run `nexa migrate && nexa seed`")
	}
	return pool, brandID
}

func newService(t *testing.T) (*Service, *fixedClock) {
	t.Helper()
	pool, brandID := testPool(t)
	keys, err := auth.LoadOrGenerate(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	clock := &fixedClock{t: time.Now()}
	svc := New(pool, keys, nil, brandID, "test", "TZ", "en")
	svc.clock = clock
	svc.DevEcho = true
	return svc, clock
}

type fixedClock struct{ t time.Time }

func (c *fixedClock) Now() time.Time { return c.t }

func uniquePhone() string {
	return "07" + time.Now().Format("0405.000000")[0:2] + time.Now().Format("05.000000")[3:9]
}

func TestOTPSignInRefreshAndReuseDetection(t *testing.T) {
	svc, clock := newService(t)
	ctx := context.Background()
	phone := uniquePhone()

	code, err := svc.RequestOTP(ctx, phone, AppRider)
	if err != nil || len(code) != OTPLength {
		t.Fatalf("request otp: %v (code %q)", err, code)
	}
	if _, err := svc.RequestOTP(ctx, phone, AppRider); !errors.Is(err, ErrOTPRateLimited) {
		t.Fatalf("expected rate limit, got %v", err)
	}
	if _, err := svc.VerifyOTP(ctx, phone, "000000", AppRider); !errors.Is(err, ErrOTPInvalid) {
		t.Fatalf("expected invalid, got %v", err)
	}
	pair, err := svc.VerifyOTP(ctx, phone, code, AppRider)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if _, err := svc.VerifyOTP(ctx, phone, code, AppRider); !errors.Is(err, ErrOTPInvalid) {
		t.Fatalf("code must be single-use, got %v", err)
	}

	// rotation
	clock.t = clock.t.Add(time.Minute)
	pair2, err := svc.Refresh(ctx, pair.RefreshToken, AppRider)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	// reuse of the rotated token revokes the whole family, including pair2
	if _, err := svc.Refresh(ctx, pair.RefreshToken, AppRider); !errors.Is(err, ErrRefreshInvalid) {
		t.Fatalf("expected reuse rejection, got %v", err)
	}
	if _, err := svc.Refresh(ctx, pair2.RefreshToken, AppRider); !errors.Is(err, ErrRefreshInvalid) {
		t.Fatalf("family must be revoked after reuse, got %v", err)
	}
}

func TestOTPLockoutAfterMaxAttempts(t *testing.T) {
	svc, _ := newService(t)
	ctx := context.Background()
	phone := uniquePhone()
	if _, err := svc.RequestOTP(ctx, phone, AppDriver); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < OTPMaxAttempts; i++ {
		if _, err := svc.VerifyOTP(ctx, phone, "999999", AppDriver); !errors.Is(err, ErrOTPInvalid) {
			t.Fatalf("attempt %d: expected invalid, got %v", i+1, err)
		}
	}
	if _, err := svc.VerifyOTP(ctx, phone, "999999", AppDriver); !errors.Is(err, ErrOTPLocked) {
		t.Fatalf("expected lockout, got %v", err)
	}
}

func TestOTPExpires(t *testing.T) {
	svc, clock := newService(t)
	ctx := context.Background()
	phone := uniquePhone()
	code, err := svc.RequestOTP(ctx, phone, AppRider)
	if err != nil {
		t.Fatal(err)
	}
	clock.t = clock.t.Add(OTPTTL + time.Second)
	if _, err := svc.VerifyOTP(ctx, phone, code, AppRider); !errors.Is(err, ErrOTPInvalid) {
		t.Fatalf("expected expired code to be invalid, got %v", err)
	}
}
