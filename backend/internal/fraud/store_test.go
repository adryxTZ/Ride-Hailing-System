package fraud

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testService(t *testing.T) (*Service, *pgxpool.Pool, string) {
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
	var brand, user string
	if err := pool.QueryRow(context.Background(), `SELECT brand_id, id FROM users WHERE full_name IS NOT NULL ORDER BY created_at LIMIT 1`).Scan(&brand, &user); err != nil {
		t.Skip("no users yet")
	}
	return New(pool, brand, []string{"TZ"}, nil), pool, user
}

// TestRaiseDedupesAndReviewFreezes: a repeat adds a hit; a review is audited and
// final; after a dismissal the same key opens nothing until the next period.
func TestRaiseDedupesAndReviewFreezes(t *testing.T) {
	ctx := context.Background()
	s, pool, user := testService(t)
	key := "TEST:" + uuid.NewString()
	f := Finding{Country: "TZ", Rule: RuleGPSSpoof, Severity: SeverityMedium, SubjectKind: SubjectDriver, SubjectID: user, DedupeKey: key, Evidence: map[string]any{"n": 1}}
	id, err := s.Raise(ctx, f)
	if err != nil || id == "" {
		t.Fatalf("raise: %v", err)
	}
	if again, err := s.Raise(ctx, f); err != nil || again != id {
		t.Fatalf("a repeat must hit the same flag: %v", err)
	}
	got, _ := s.Get(ctx, id)
	if got.Hits != 2 || got.Status != StatusOpen {
		t.Fatalf("hits %d status %s", got.Hits, got.Status)
	}
	if _, err := s.Review(ctx, user, id, DecisionDismiss, ""); err != ErrReasonRequired {
		t.Fatal("a review needs a note")
	}
	if _, err := s.Review(ctx, user, id, DecisionDismiss, "test: reviewed"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Review(ctx, user, id, DecisionConfirm, "again"); err != ErrFlagReviewed {
		t.Fatal("a reviewed flag must not be reviewed again")
	}
	if id2, err := s.Raise(ctx, f); err != nil || id2 != "" {
		t.Fatalf("after a dismissal the same period opens nothing (got %q, %v)", id2, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE fraud_flags SET hits=99 WHERE id=$1`, id); err == nil {
		t.Fatal("the database let a reviewed flag change")
	}
	var audited int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE target=$1 AND action='fraud.review'`, "fraud_flag:"+id).Scan(&audited)
	if audited != 1 {
		t.Fatalf("review audit rows: %d", audited)
	}
}

// TestCancelScanQueriesRun runs the scan's queries on the dev data (their SQL and shapes).
func TestCancelScanQueriesRun(t *testing.T) {
	ctx := context.Background()
	s, _, _ := testService(t)
	since := s.now().Add(-30 * 24 * time.Hour)
	ds, err := s.store.driverCounts(ctx, "TZ", since)
	if err != nil {
		t.Fatal(err)
	}
	rs, err := s.store.riderCounts(ctx, "TZ", since)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range append(ds, rs...) {
		if c.Cancels > c.Accepted && c.SubjectKind == SubjectRider {
			t.Fatalf("a rider cannot cancel more assigned trips than were assigned: %+v", c)
		}
	}
	t.Logf("drivers %d, riders %d", len(ds), len(rs))
}
