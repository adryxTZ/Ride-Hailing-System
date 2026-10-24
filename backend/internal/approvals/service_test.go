package approvals

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// fakeStaff gives each test user fixed grants.
type fakeStaff map[string]Grants

func (f fakeStaff) Grants(_ context.Context, id string) (Grants, error) { return f[id], nil }
func (f fakeStaff) Names(context.Context, []string) (map[string]string, error) {
	return map[string]string{}, nil
}

// fixture is a service with one registered test action whose executor counts runs.
type fixture struct {
	svc                        *Service
	pool                       *pgxpool.Pool
	action                     string
	runs                       *atomic.Int32
	requester, finance, finan2 string
}

func newFixture(t *testing.T) fixture {
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
	var brand string
	rows, err := pool.Query(context.Background(), `SELECT brand_id, id FROM users ORDER BY created_at LIMIT 3`)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&brand, &id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	rows.Close()
	if len(ids) < 3 {
		t.Skip("needs three users")
	}
	f := fixture{pool: pool, action: "test.refund." + uuid.NewString()[:8], runs: &atomic.Int32{},
		requester: ids[0], finance: ids[1], finan2: ids[2]}
	cat := NewCatalogue()
	cat.Register(Action{Key: f.action, Label: "Test refund", Condition: ConditionAmount, Scoped: true,
		Execute: func(_ context.Context, x Execution) (any, error) {
			f.runs.Add(1)
			return map[string]string{"reason": x.Reason}, nil
		}})
	staff := fakeStaff{f.requester: {Roles: []string{"Support"}}, f.finance: {Roles: []string{"Finance"}},
		f.finan2: {Roles: []string{"Finance", "Operations"}}}
	rules := func(context.Context) (Rules, error) {
		return Rules{Roles: []string{"Finance", "Operations", "Support"}, Permissions: []string{"trips.refund"}, Countries: []string{"TZ", "KE"}}, nil
	}
	f.svc = New(Deps{Pool: pool, BrandID: brand, Catalogue: cat, Staff: staff, Rules: rules})
	t.Cleanup(func() { f.cleanup(t) })
	return f
}

// cleanup removes this test's rows (triggers off: decisions and versions are append-only).
func (f fixture) cleanup(t *testing.T) {
	ctx := context.Background()
	conn, err := f.pool.Acquire(ctx)
	if err != nil {
		return
	}
	defer conn.Release()
	_, _ = conn.Exec(ctx, `SET session_replication_role = replica`)
	for _, sql := range []string{
		`DELETE FROM approval_decisions WHERE request_id IN (SELECT id FROM approval_requests WHERE action=$1)`,
		`DELETE FROM approval_requests WHERE action=$1`,
		`DELETE FROM approval_workflow_versions WHERE action=$1`,
		`DELETE FROM approval_workflows WHERE action=$1`,
		`DELETE FROM audit_log WHERE action LIKE 'approval.%' AND (data->>'action'=$1 OR target='approval_workflow:'||$1)`,
	} {
		if _, err := conn.Exec(ctx, sql, f.action); err != nil {
			t.Logf("cleanup: %v", err)
		}
	}
	_, _ = conn.Exec(ctx, `SET session_replication_role = origin`)
}

func (f fixture) enable(t *testing.T, steps ...Step) {
	t.Helper()
	_, err := f.svc.SaveWorkflow(context.Background(), f.finance, f.action, WorkflowInput{Enabled: true, Reason: "test",
		Definition: Definition{ExpireHours: 1, Conditions: Conditions{AmountMin: dec("10000"), Countries: []string{"TZ"}}, Steps: steps}})
	if err != nil {
		t.Fatal(err)
	}
}

func (f fixture) submit(t *testing.T, amount string) *Request {
	t.Helper()
	r, err := f.svc.Submit(context.Background(), Submission{Action: f.action, RequesterID: f.requester, Reason: "rider was overcharged",
		Country: "TZ", Amount: dec(amount), Target: "trip:x", Payload: map[string]string{"trip": uuid.NewString(), "amount": amount}})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

var financeStep = Step{Name: "Finance", Mode: ModeAny, Roles: []string{"Finance"}, Approvals: 1}

// TestGateOffAndConditions: off by default; on, only matching changes are held.
func TestGateOffAndConditions(t *testing.T) {
	f := newFixture(t)
	if r := f.submit(t, "50000"); r != nil || f.svc.Enabled(context.Background(), f.action) {
		t.Fatal("a never-saved workflow must not hold anything")
	}
	f.enable(t, financeStep)
	if !f.svc.Enabled(context.Background(), f.action) {
		t.Fatal("a saved, enabled workflow must read as on at once (cache marked stale)")
	}
	if r := f.submit(t, "9999"); r != nil {
		t.Fatal("below the amount condition applies at once")
	}
	if r, _ := f.svc.Submit(context.Background(), Submission{Action: f.action, RequesterID: f.requester, Reason: "x", Country: "KE",
		Amount: dec("50000"), Payload: 1}); r != nil {
		t.Fatal("another country applies at once")
	}
	r := f.submit(t, "10000")
	if r == nil || r.Status != StatusPending || r.WorkflowVersion != 1 || len(r.Workflow.Steps) != 1 {
		t.Fatalf("at the threshold the change must be held: %+v", r)
	}
	if _, err := f.svc.Submit(context.Background(), Submission{Action: f.action, RequesterID: f.requester, Country: "TZ",
		Amount: dec("20000"), Payload: 2}); !errors.Is(err, ErrReasonRequired) {
		t.Fatalf("a held change needs a reason, got %v", err)
	}
}

// TestSelfApprovalAndExecutesOnce: the requester cannot approve; the final
// approval runs the action once with the approvers in the reason.
func TestSelfApprovalAndExecutesOnce(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.enable(t, financeStep)
	r := f.submit(t, "15000")
	if _, err := f.svc.Approve(ctx, Viewer{ID: f.requester}, r.ID, ""); !errors.Is(err, ErrSelfApproval) {
		t.Fatalf("self approval must be refused, got %v", err)
	}
	if _, err := f.pool.Exec(ctx, `INSERT INTO approval_decisions (request_id, step, approver_id, decision) VALUES ($1, 0, $2, 'APPROVE')`,
		r.ID, f.requester); err == nil {
		t.Fatal("the database must refuse a requester's own decision")
	}
	if _, err := f.svc.Approve(ctx, Viewer{ID: f.finance, Countries: []string{"KE"}}, r.ID, ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("staff limited to another country must not see it, got %v", err)
	}
	done, err := f.svc.Approve(ctx, Viewer{ID: f.finance}, r.ID, "checked the trip")
	if err != nil {
		t.Fatal(err)
	}
	if done.Status != StatusExecuted || f.runs.Load() != 1 {
		t.Fatalf("status %s runs %d", done.Status, f.runs.Load())
	}
	var res map[string]string
	_ = json.Unmarshal(done.Result, &res)
	if want := "rider was overcharged (approved by"; len(res["reason"]) < len(want) || res["reason"][:len(want)] != want {
		t.Fatalf("executor reason %q", res["reason"])
	}
	f.svc.execute(ctx, r.ID, f.finance) // a second run (e.g. a retry) does nothing
	if f.runs.Load() != 1 {
		t.Fatal("the executor ran twice")
	}
	if _, err := f.svc.Approve(ctx, Viewer{ID: f.finan2}, r.ID, ""); !errors.Is(err, ErrNotPending) {
		t.Fatalf("a finished request takes no more decisions, got %v", err)
	}
}

// TestStepsInOrderAndReject: two steps, distinct people; ALL needs every role;
// a rejection ends the request without running it.
func TestStepsInOrderAndReject(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.enable(t, financeStep, Step{Name: "Ops and finance", Mode: ModeAll, Roles: []string{"Operations", "Finance"}, Approvals: 1})
	r := f.submit(t, "20000")
	if r2, _ := f.svc.Approve(ctx, Viewer{ID: f.finance}, r.ID, ""); r2.CurrentStep != 1 || r2.Status != StatusPending {
		t.Fatalf("after step 1: step %d status %s", r2.CurrentStep, r2.Status)
	}
	if _, err := f.svc.Approve(ctx, Viewer{ID: f.finance}, r.ID, ""); !errors.Is(err, ErrAlreadyDecided) {
		t.Fatalf("a person decides a request once, got %v", err)
	}
	r3, err := f.svc.Approve(ctx, Viewer{ID: f.finan2}, r.ID, "")
	if err != nil || r3.Status != StatusPending {
		t.Fatalf("ALL with one of two roles is still pending: %v %v", err, r3)
	}
	if f.runs.Load() != 0 {
		t.Fatal("nothing runs before the last step")
	}
	other := f.submit(t, "30000")
	if _, err := f.svc.Reject(ctx, Viewer{ID: f.finance}, other.ID, ""); !errors.Is(err, ErrNoteRequired) {
		t.Fatalf("a rejection needs a note, got %v", err)
	}
	rej, err := f.svc.Reject(ctx, Viewer{ID: f.finance}, other.ID, "not justified")
	if err != nil || rej.Status != StatusRejected || f.runs.Load() != 0 {
		t.Fatalf("rejected: %v %+v", err, rej)
	}
}

// TestKeepsDefinitionAndExpires: a later edit does not change a pending
// request's steps; an overdue request expires and cannot be approved.
func TestKeepsDefinitionAndExpires(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.enable(t, financeStep)
	r := f.submit(t, "12000")
	f.enable(t, financeStep, Step{Name: "Ops", Mode: ModeAny, Roles: []string{"Operations"}, Approvals: 1})
	got, _ := f.svc.Get(ctx, Viewer{ID: f.requester}, r.ID)
	if len(got.Workflow.Steps) != 1 || got.WorkflowVersion != 1 {
		t.Fatalf("the pending request must keep version 1: %+v", got.Workflow)
	}
	f.svc.now = func() time.Time { return time.Now().Add(2 * time.Hour) }
	if _, err := f.svc.Approve(ctx, Viewer{ID: f.finance}, r.ID, ""); !errors.Is(err, ErrExpired) {
		t.Fatalf("an overdue request must expire, got %v", err)
	}
	got, _ = f.svc.Get(ctx, Viewer{ID: f.requester}, r.ID)
	if got.Status != StatusExpired || f.runs.Load() != 0 {
		t.Fatalf("status %s", got.Status)
	}
}

// TestDuplicateAndCancel: the same pending change is one request; only the requester cancels.
func TestDuplicateAndCancel(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.enable(t, financeStep)
	sub := Submission{Action: f.action, RequesterID: f.requester, Reason: "r", Country: "TZ", Amount: dec("11000"), Payload: map[string]int{"n": 1}}
	a, _ := f.svc.Submit(ctx, sub)
	b, _ := f.svc.Submit(ctx, sub)
	if a == nil || b == nil || a.ID != b.ID {
		t.Fatal("a repeated pending change must return the same request")
	}
	if _, err := f.svc.Cancel(ctx, Viewer{ID: f.finance}, a.ID, ""); !errors.Is(err, ErrNotRequester) {
		t.Fatalf("only the requester cancels, got %v", err)
	}
	c, err := f.svc.Cancel(ctx, Viewer{ID: f.requester}, a.ID, "")
	if err != nil || c.Status != StatusCancelled {
		t.Fatalf("cancel: %v", err)
	}
	if n, _ := f.svc.Actionable(ctx, Viewer{ID: f.finance}); n < 0 {
		t.Fatal("count")
	}
	var amount decimal.Decimal
	_ = f.pool.QueryRow(ctx, `SELECT amount FROM approval_requests WHERE id=$1`, a.ID).Scan(&amount)
	if !amount.Equal(decimal.RequireFromString("11000")) {
		t.Fatalf("stored amount %s", amount)
	}
}
