package receipts

import (
	"bytes"
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

var d = decimal.RequireFromString

func facts() Facts {
	dist := 3702
	return Facts{
		TripID: "trip-1", RiderID: "rider-1", DriverID: "driver-1", Country: "TZ", Currency: "TZS",
		Lines: []FactLine{
			{Code: "BASE_FARE", Amount: d("6411.23")},
			{Code: "ROUNDING", Amount: d("-11.23")},
			{Code: "BOOKING_FEE", Amount: d("0")},
			{Code: "WAITING", Amount: d("260")},
		},
		Total: d("6660"), VAT: d("97.63"), Levy: d("64"),
		Payment: Payment{Method: "MOBILE_MONEY", Status: "COLLECTED"},
		Trip:    TripInfo{Code: "9DCB88", Pickup: "Kariakoo", Dropoff: "Julius Nyerere International Airport, Terminal 3, Dar es Salaam", DistanceM: &dist, CompletedAt: time.Date(2026, 10, 23, 7, 30, 0, 0, time.UTC)},
		Driver:  DriverInfo{Name: "Driver Name", Plate: "T 123 ABC"},
	}
}

var issuer = Issuer{Name: "Platform", LegalEntity: "Platform Ltd", SupportPhone: "+255700000000", Country: "TZ", Timezone: "Africa/Dar_es_Salaam"}

func TestLinesMustAddUpToWhatWasPaid(t *testing.T) {
	r, err := Build(facts(), issuer)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Lines) != 3 { // the zero booking fee is left out
		t.Fatalf("want 3 lines, got %+v", r.Lines)
	}
	if r.Lines[2].Label[LangSW] != "Muda wa kusubiri" || r.Lines[0].Label[LangEN] != "Trip fare" {
		t.Fatalf("labels: %+v", r.Lines)
	}
	f := facts()
	f.Total = d("6400")
	if _, err := Build(f, issuer); !errors.Is(err, ErrLinesMismatch) {
		t.Fatalf("a receipt that disagrees with the money must not be built: %v", err)
	}
}

func TestNumbersAreYearAndSequence(t *testing.T) {
	if Number(2026, 123) != "2026-000123" {
		t.Fatal(Number(2026, 123))
	}
	// 23:30 UTC on 31 Dec is already the next year in East Africa.
	if yearIn(time.Date(2026, 12, 31, 23, 30, 0, 0, time.UTC), "Africa/Dar_es_Salaam") != 2027 {
		t.Fatal("the numbering year is the local year")
	}
}

func TestSignedLinks(t *testing.T) {
	s := signer{key: []byte("test-key")}
	now := time.Unix(1_800_000_000, 0)
	exp := now.Add(LinkTTL).Unix()
	sig := s.sign("receipt-1", "sw", exp)
	ok := func(id, lang, e, g string, at time.Time) error { return s.verify(id, lang, e, g, at) }
	e := itoa(exp)
	if err := ok("receipt-1", "sw", e, sig, now); err != nil {
		t.Fatalf("valid link refused: %v", err)
	}
	for name, err := range map[string]error{
		"other receipt":  ok("receipt-2", "sw", e, sig, now),
		"other language": ok("receipt-1", "en", e, sig, now),
		"longer expiry":  ok("receipt-1", "sw", itoa(exp+3600), sig, now),
		"expired":        ok("receipt-1", "sw", e, sig, now.Add(LinkTTL+time.Second)),
		"other key":      signer{key: []byte("other")}.verify("receipt-1", "sw", e, sig, now),
		"garbage expiry": ok("receipt-1", "sw", "soon", sig, now),
	} {
		if !errors.Is(err, ErrLinkInvalid) {
			t.Errorf("%s: want ErrLinkInvalid, got %v", name, err)
		}
	}
}

func itoa(n int64) string { return decimal.NewFromInt(n).String() }

func TestPDFIsDeterministicInBothLanguages(t *testing.T) {
	r, err := Build(facts(), issuer)
	if err != nil {
		t.Fatal(err)
	}
	r.Number, r.IssuedAt = "2026-000001", time.Date(2026, 10, 23, 7, 30, 0, 0, time.UTC)
	for _, lang := range []string{LangEN, LangSW} {
		a, err := Render(&r, lang)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := Render(&r, lang)
		if !bytes.HasPrefix(a, []byte("%PDF-1.4")) || !bytes.Equal(a, b) {
			t.Fatalf("%s: not a deterministic PDF", lang)
		}
		want := map[string]string{LangEN: "Total paid", LangSW: "Jumla iliyolipwa"}[lang]
		if !bytes.Contains(a, []byte(want)) || !bytes.Contains(a, []byte("TZS 6,660")) {
			t.Fatalf("%s: missing %q or the total", lang, want)
		}
		if out := os.Getenv("RECEIPT_SAMPLE_DIR"); out != "" {
			_ = os.WriteFile(out+"/receipt-"+lang+".pdf", a, 0o600)
		}
	}
}

func TestMoneyFormat(t *testing.T) {
	for in, want := range map[string]string{"6400": "TZS 6,400", "1234567.6": "TZS 1,234,568", "-11.23": "TZS -11", "0": "TZS 0"} {
		if got := money("TZS", d(in)); got != want {
			t.Errorf("%s: got %s want %s", in, got, want)
		}
	}
}

// TestNumbersAreGapless takes 20 numbers concurrently; with a rollback in the
// middle the sequence still has no hole. Needs NEXA_TEST_DATABASE_URL.
func TestNumbersAreGapless(t *testing.T) {
	url := os.Getenv("NEXA_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("NEXA_TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close) // registered first, so it runs after the counter cleanup below
	var brandID string
	if err := pool.QueryRow(ctx, `SELECT id FROM brands ORDER BY created_at LIMIT 1`).Scan(&brandID); err != nil {
		t.Skip("no seeded brand")
	}
	s := store{pool: pool, brandID: brandID}
	const year = 1999 // a year no real receipt uses
	reset := func() {
		_, _ = pool.Exec(ctx, `DELETE FROM receipt_counters WHERE brand_id=$1 AND year=$2`, brandID, year)
	}
	reset()
	t.Cleanup(reset)
	var mu sync.Mutex
	var got []int
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
				n, err := s.nextSeq(ctx, tx, year)
				if err != nil {
					return err
				}
				if i == 7 {
					return errors.New("rolled back") // its number goes to someone else
				}
				mu.Lock()
				got = append(got, n)
				mu.Unlock()
				return nil
			})
			if err != nil && i != 7 {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	seen := map[int]bool{}
	for _, n := range got {
		seen[n] = true
	}
	for n := 1; n <= 19; n++ {
		if !seen[n] {
			t.Fatalf("number %d missing from %v", n, got)
		}
	}
}
