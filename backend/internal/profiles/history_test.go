package profiles

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

func TestRewindUndoesLaterChanges(t *testing.T) {
	from := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	now := SnapProfile{ID: "p", Name: "New name", Status: StatusInactive}
	later := []Event{ // newest first
		{Kind: EventDeactivated, Data: map[string]any{"from": StatusActive}},
		{Kind: EventRenamed, Data: map[string]any{"from": "Old name", "to": "New name"}},
		{Kind: EventWindowChanged, Data: map[string]any{"old": map[string]any{"from": from.Format(time.RFC3339), "until": nil}}},
		{Kind: EventActivated, Data: map[string]any{"from": StatusDraft}},
	}
	got := rewind(now, later)
	if got.Status != StatusDraft || got.Name != "Old name" || got.ActiveFrom == nil || !got.ActiveFrom.Equal(from) || got.ActiveUntil != nil {
		t.Fatalf("rewind gave %+v", got)
	}
	if rewind(now, nil) != now {
		t.Fatal("no later events must leave the profile as it is")
	}
}

func TestCheckRange(t *testing.T) {
	t0 := time.Now()
	if checkRange(t0, t0.Add(-time.Hour), 31) != ErrRangeInvalid {
		t.Fatal("a reversed range must be refused")
	}
	if checkRange(t0, t0.Add(32*24*time.Hour), 31) != ErrRangeTooLarge {
		t.Fatal("over 31 days must be refused")
	}
	if checkRange(t0, t0.Add(24*time.Hour), 31) != nil {
		t.Fatal("one day is fine")
	}
}

// fakeArea makes any area id a region of TZ (the test never touches real regions).
type fakeArea struct{}

func (fakeArea) AreaInfo(context.Context, string) (string, string, bool, error) {
	return "TZ", "Test region", true, nil
}

func dbClock(t *testing.T, pool *pgxpool.Pool) time.Time {
	t.Helper()
	time.Sleep(100 * time.Millisecond) // the API and database clocks may differ by a few ms
	var at time.Time
	if err := pool.QueryRow(context.Background(), `SELECT clock_timestamp()`).Scan(&at); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	return at
}

// TestAsOfReplaysHistory builds a history (assign + v1, publish v2, switch off)
// and checks that "as of" each point names what applied then.
func TestAsOfReplaysHistory(t *testing.T) {
	ctx := context.Background()
	pool, _, brand := liveRecord(t)
	s := New(pool, brand, nil, fakeArea{})
	area := uuid.NewString()
	t0 := dbClock(t, pool)
	p, err := s.Create(ctx, "", CreateInput{Country: "TZ", Name: "As-of test " + area[:8], Reason: "test"})
	if err != nil {
		t.Fatal(err)
	}
	rate := func(v string) *decimal.Decimal { d := decimal.RequireFromString(v); return &d }
	if _, err := s.Publish(ctx, "", p.ID, VersionInput{Terms: Terms{CommissionRate: rate("0.11")}, Reason: "v1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetStatus(ctx, "", p.ID, StatusActive, "test"); err != nil {
		t.Fatal(err)
	}
	sc, err := s.Assign(ctx, "", p.ID, AssignInput{Kind: ScopeArea, ScopeID: area, Reason: "test"})
	if err != nil {
		t.Fatal(err)
	}
	t1 := dbClock(t, pool)
	if _, err := s.Publish(ctx, "", p.ID, VersionInput{Terms: Terms{CommissionRate: rate("0.12")}, Reason: "v2"}); err != nil {
		t.Fatal(err)
	}
	t2 := dbClock(t, pool)
	if _, err := s.SetStatus(ctx, "", p.ID, StatusInactive, "test"); err != nil {
		t.Fatal(err)
	}
	t3 := dbClock(t, pool)
	t.Cleanup(func() { _ = s.Unassign(ctx, "", p.ID, sc.AssignmentID, "test cleanup") })

	for _, c := range []struct {
		name    string
		at      time.Time
		version int // 0 = the country default wins
	}{{"before the profile existed", t0, 0}, {"after assigning v1", t1, 1}, {"after publishing v2", t2, 2}, {"after switching off", t3, 0}} {
		res, err := s.AsOf(ctx, Input{Country: "TZ", AreaID: area, AreaName: "Test region", At: c.at})
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		won := res.Profile.ID == p.ID
		if (c.version == 0 && won) || (c.version != 0 && (!won || res.Profile.Version != c.version)) {
			t.Fatalf("%s: got %s v%d (%s) trace %+v", c.name, res.Profile.Name, res.Profile.Version, res.Scope.Kind, res.Trace.Candidates)
		}
	}
	if _, err := s.AsOf(ctx, Input{Country: "TZ", At: time.Now().Add(time.Hour)}); err != ErrAsOfInFuture {
		t.Fatal("a future time must be refused")
	}
}

// TestPreviewSwitchesOnInResponseOnly checks a preview of an inactive profile
// wins in the answer and changes nothing stored.
func TestPreviewSwitchesOnInResponseOnly(t *testing.T) {
	ctx := context.Background()
	pool, _, brand := liveRecord(t)
	s := New(pool, brand, nil, fakeArea{})
	area := uuid.NewString()
	p, err := s.Create(ctx, "", CreateInput{Country: "TZ", Name: "Preview test " + area[:8], Reason: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Publish(ctx, "", p.ID, VersionInput{Terms: Terms{}, Reason: "v1"}); err != nil {
		t.Fatal(err)
	}
	sc, err := s.Assign(ctx, "", p.ID, AssignInput{Kind: ScopeArea, ScopeID: area, Reason: "test"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Unassign(ctx, "", p.ID, sc.AssignmentID, "test cleanup") })
	in := Input{Country: "TZ", AreaID: area, AreaName: "Test region"}
	if res, err := s.Preview(ctx, in, ""); err != nil || res.Profile.ID == p.ID {
		t.Fatalf("a draft must not price without a preview (err %v)", err)
	}
	res, err := s.Preview(ctx, in, p.ID)
	if err != nil || res.Profile.ID != p.ID {
		t.Fatalf("the previewed draft must win (err %v)", err)
	}
	if got, _ := s.Get(ctx, p.ID); got.Status != StatusDraft {
		t.Fatal("a preview changed the stored status")
	}
	if _, err := s.Preview(ctx, Input{Country: "TZ", At: time.Now().Add(8 * 24 * time.Hour)}, ""); err != ErrSimulateAtRange {
		t.Fatal("more than 7 days ahead must be refused")
	}
}
