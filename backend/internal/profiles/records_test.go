package profiles

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// liveRecord finds a sealed record made at booking, without changing anything. Needs NEXA_TEST_DATABASE_URL.
func liveRecord(t *testing.T) (*pgxpool.Pool, string, string) {
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
	var trip, brand string
	if err := pool.QueryRow(context.Background(), `SELECT trip_id, brand_id FROM trip_pricing_records WHERE stage='FINAL' AND source='LIVE' ORDER BY finalized_at DESC LIMIT 1`).Scan(&trip, &brand); err != nil {
		t.Skip("no sealed live record yet (run an e2e ride first)")
	}
	return pool, trip, brand
}

func TestSealedRecordCannotChange(t *testing.T) {
	ctx := context.Background()
	pool, trip, _ := liveRecord(t)
	for name, sql := range map[string]string{
		"update terms":    `UPDATE trip_pricing_records SET terms='{}' WHERE trip_id=$1`,
		"re-seal":         `UPDATE trip_pricing_records SET record_hash='x', finalized_at=now() WHERE trip_id=$1`,
		"move to a brand": `UPDATE trip_pricing_records SET brand_id=gen_random_uuid() WHERE trip_id=$1`,
		"delete":          `DELETE FROM trip_pricing_records WHERE trip_id=$1`,
	} {
		if _, err := pool.Exec(ctx, sql, trip); err == nil {
			t.Fatalf("%s was allowed on a sealed record", name)
		}
	}
}

func TestSealVerifiesAndFinaliseIsOnce(t *testing.T) {
	ctx := context.Background()
	pool, trip, brand := liveRecord(t)
	s := New(pool, brand, nil, nil)
	r, err := s.Record(ctx, trip)
	if err != nil {
		t.Fatal(err)
	}
	if r.SealValid == nil || !*r.SealValid {
		t.Fatal("a sealed record's hash must verify against its stored columns")
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := s.FinalizeTx(ctx, tx, trip); err != nil {
		t.Fatalf("finalising a sealed record again must be a no-op, got %v", err)
	}
	var hash string
	if err := tx.QueryRow(ctx, `SELECT record_hash FROM trip_pricing_records WHERE trip_id=$1`, trip).Scan(&hash); err != nil || hash != *r.RecordHash {
		t.Fatal("a second finalise changed the seal")
	}
}

func TestVersionsAndAssignmentsAreAppendOnly(t *testing.T) {
	ctx := context.Background()
	pool, _, _ := liveRecord(t)
	for name, sql := range map[string]string{
		"edit a version":       `UPDATE pricing_profile_versions SET commission_rate=0.5 WHERE id=(SELECT id FROM pricing_profile_versions LIMIT 1)`,
		"delete a version":     `DELETE FROM pricing_profile_versions WHERE id=(SELECT id FROM pricing_profile_versions LIMIT 1)`,
		"move an assignment":   `UPDATE pricing_profile_assignments SET priority=priority+1 WHERE id=(SELECT id FROM pricing_profile_assignments LIMIT 1)`,
		"delete an assignment": `DELETE FROM pricing_profile_assignments WHERE id=(SELECT id FROM pricing_profile_assignments LIMIT 1)`,
		"add a charge later":   `INSERT INTO pricing_profile_charges (version_id, code, label, category, amount_type, amount, recipient, applies_when) SELECT id, 'LATE', '{"en":"x","sw":"x"}', 'OTHER', 'FLAT', 1, 'DRIVER', 'PICKUP' FROM pricing_profile_versions LIMIT 1`,
	} {
		if _, err := pool.Exec(ctx, sql); err == nil {
			t.Fatalf("%s was allowed", name)
		}
	}
}
