package branding

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/theme"
)

// memFiles is an object store that knows which keys were "uploaded".
type memFiles struct{ sizes map[string]int64 }

func (m memFiles) UploadURL(context.Context, string, string, time.Duration) (string, error) {
	return "/put", nil
}
func (m memFiles) DownloadURL(_ context.Context, key string, _ time.Duration) (string, error) {
	return "/get/" + key, nil
}
func (m memFiles) Size(_ context.Context, key string) (int64, error) {
	if n, ok := m.sizes[key]; ok {
		return n, nil
	}
	return 0, errors.New("missing")
}

// newTestService runs against a throwaway brand row. Needs NEXA_TEST_DATABASE_URL.
func newTestService(t *testing.T, files memFiles) (*Service, string) {
	t.Helper()
	url := os.Getenv("NEXA_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("NEXA_TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	var brandID string
	code := fmt.Sprintf("test-%d", rand.IntN(1_000_000))
	if err := pool.QueryRow(ctx, `INSERT INTO brands (code, display_name, country, currency, timezone, default_locale)
		VALUES ($1, 'Test brand', 'ZZ', 'TZS', 'UTC', 'en') RETURNING id`, code).Scan(&brandID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM brand_themes WHERE brand_id=$1`, brandID)
		_, _ = pool.Exec(ctx, `DELETE FROM audit_log WHERE brand_id=$1`, brandID)
		_, _ = pool.Exec(ctx, `DELETE FROM brands WHERE id=$1`, brandID)
	})
	var staff string
	if err := pool.QueryRow(ctx, `SELECT id FROM users ORDER BY created_at LIMIT 1`).Scan(&staff); err != nil {
		t.Skip("no user to act as staff")
	}
	fallback := &theme.Theme{Version: 0, Palette: theme.Derive(map[string]string{"primary": "#5003C0"})}
	return New(pool, brandID, files, &theme.Presets{}, fallback), staff
}

func goodDraft() DraftInput {
	return DraftInput{Name: "Navy", Source: "CUSTOM", Roles: Roles{Light: map[string]string{
		"primary": "#0B2447", "accent": "#576CBC", "background": "#FFFFFF", "surface": "#F4F6FA"}},
		Typography: map[string]string{"body_font": "Figtree"}, Shape: map[string]int{"radius_md": 10}}
}

func TestPublishAndRollback(t *testing.T) {
	ctx := context.Background()
	s, staff := newTestService(t, memFiles{})
	live, err := s.Published(ctx)
	if err != nil || live.Theme.Version != 0 {
		t.Fatalf("before any publish the fallback serves: %v %+v", err, live.Theme)
	}
	v1, err := s.CreateDraft(ctx, staff, goodDraft())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Publish(ctx, staff, v1.ID, ""); !errors.Is(err, ErrReasonRequired) {
		t.Fatalf("publish without reason = %v", err)
	}
	if _, err := s.Publish(ctx, staff, v1.ID, "launch colours"); err != nil {
		t.Fatal(err)
	}
	second := goodDraft()
	second.Roles.Light["primary"] = "#5003C0"
	v2, _ := s.CreateDraft(ctx, staff, second)
	if _, err := s.Publish(ctx, staff, v2.ID, "new look"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateDraft(ctx, staff, v2.ID, goodDraft()); !errors.Is(err, ErrNotDraft) {
		t.Fatalf("a published version must never change, got %v", err)
	}
	if _, err := s.Publish(ctx, staff, v1.ID, "roll back"); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	got, _ := s.Get(ctx, v1.ID)
	if got.Status != StatusPublished {
		t.Fatalf("v1 status after rollback = %s", got.Status)
	}
	if again, _ := s.Get(ctx, v2.ID); again.Status != StatusArchived {
		t.Fatalf("v2 status after rollback = %s", again.Status)
	}
}

func TestPublishBlockedByContrast(t *testing.T) {
	ctx := context.Background()
	s, staff := newTestService(t, memFiles{})
	bad := goodDraft()
	bad.Roles.Light["accent"] = "#0C2548" // looks like the primary
	d, err := s.CreateDraft(ctx, staff, bad)
	if err != nil {
		t.Fatal(err)
	}
	if d.Report.Pass {
		t.Fatal("report must fail")
	}
	if _, err := s.Publish(ctx, staff, d.ID, "try"); !errors.Is(err, ErrContrastFails) {
		t.Fatalf("publish = %v, want contrast failure", err)
	}
}

func TestDraftValidationAndAssets(t *testing.T) {
	ctx := context.Background()
	key := "branding/x/logo.png"
	s, staff := newTestService(t, memFiles{sizes: map[string]int64{key: 1024}})
	bad := goodDraft()
	bad.Typography = map[string]string{"body_font": "Comic Sans"}
	if _, err := s.CreateDraft(ctx, staff, bad); err == nil {
		t.Fatal("a font outside the allow-list must be refused")
	}
	missing := goodDraft()
	missing.Assets = map[string]string{"logo_full": "branding/x/none.png"}
	if _, err := s.CreateDraft(ctx, staff, missing); !errors.Is(err, ErrAssetMissing) {
		t.Fatalf("missing upload = %v", err)
	}
	ok := goodDraft()
	ok.Assets = map[string]string{"logo_full": key}
	d, err := s.CreateDraft(ctx, staff, ok)
	if err != nil {
		t.Fatal(err)
	}
	if d.AssetURLs["logo_full"] != PublicPrefix+key {
		t.Fatalf("asset url = %q", d.AssetURLs["logo_full"])
	}
}

func TestIdentityEdit(t *testing.T) {
	ctx := context.Background()
	s, staff := newTestService(t, memFiles{})
	if _, err := s.SetIdentity(ctx, staff, IdentityInput{Name: "Ride Co", Reason: ""}); !errors.Is(err, ErrReasonRequired) {
		t.Fatalf("identity without reason = %v", err)
	}
	id, err := s.SetIdentity(ctx, staff, IdentityInput{Name: "Ride Co", SupportEmail: "help@ride.example", Reason: "rename"})
	if err != nil {
		t.Fatal(err)
	}
	if id.Name != "Ride Co" || id.EditedAt == nil {
		t.Fatalf("identity = %+v", id)
	}
}
