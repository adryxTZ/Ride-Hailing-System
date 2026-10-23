package selcom

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/providers/payment"
)

// testPayouts is a client against a fake Selcom that checks every signature
// and answers transfers with code and queries with query.
func testPayouts(t *testing.T, code, query string) (*Payouts, *[]map[string]string) {
	t.Helper()
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	var seen []map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fields := map[string]string{}
		if r.Method == http.MethodPost {
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &fields)
		} else {
			for k := range r.URL.Query() {
				fields[k] = r.URL.Query().Get(k)
			}
		}
		parts := []string{"timestamp=" + r.Header.Get("timestamp")}
		for _, n := range strings.Split(r.Header.Get("signed-fields"), ",") {
			parts = append(parts, n+"="+fields[n])
		}
		sum := sha256.Sum256([]byte(strings.Join(parts, "&")))
		sig, _ := base64.StdEncoding.DecodeString(r.Header.Get("digest"))
		if rsa.VerifyPKCS1v15(&key.PublicKey, crypto.SHA256, sum[:], sig) != nil {
			http.Error(w, `{"resultcode":"401","message":"bad signature"}`, http.StatusUnauthorized)
			return
		}
		seen = append(seen, fields)
		answer := code
		if r.Method == http.MethodGet {
			answer = query
		}
		_, _ = w.Write([]byte(`{"resultcode":"` + answer + `","message":"x","data":[{"reference":"SEL123"}]}`))
	}))
	t.Cleanup(srv.Close)
	der := x509.MarshalPKCS1PrivateKey(key)
	p, err := NewPayouts(PayoutConfig{BaseURL: srv.URL, APIKey: "k", PrivateKeyPEM: pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: der})})
	if err != nil {
		t.Fatal(err)
	}
	return p, &seen
}

func req(phone string) payment.DisburseRequest {
	return payment.DisburseRequest{PayoutID: "po-1", Phone: phone, RecipientName: "Driver", Amount: decimal.NewFromInt(5000), Currency: "TZS"}
}

func TestPayoutSignedAndRoutedToTheRightWallet(t *testing.T) {
	p, seen := testPayouts(t, codeProcessing, codePaid)
	ctx := context.Background()
	if err := p.Disburse(ctx, req("+255754000111")); err != nil {
		t.Fatal(err)
	}
	if got := (*seen)[0]; got["recipientFiCode"] != "MPESA" || got["recipientAccount"] != "255754000111" || got["transId"] != "po-1" || got["amount"] != "5000" {
		t.Fatalf("transfer fields: %+v", got)
	}
	st, err := p.PayoutStatus(ctx, "po-1")
	if err != nil || st.State != payment.PayoutPaid || st.ProviderTxnID != "SEL123" {
		t.Fatalf("status: %+v %v", st, err)
	}
}

func TestPayoutAmbiguityNeverLooksFailed(t *testing.T) {
	for _, code := range []string{codeAmbiguous, codeDuplicate, codeQueued} {
		p, _ := testPayouts(t, code, codeAmbiguous)
		if err := p.Disburse(context.Background(), req("+255688000111")); err != nil {
			t.Fatalf("%s must be treated as possibly sent: %v", code, err)
		}
		if st, _ := p.PayoutStatus(context.Background(), "po-1"); st.State != payment.PayoutPending {
			t.Fatalf("%s query must stay pending: %+v", code, st)
		}
	}
}

func TestPayoutRefusals(t *testing.T) {
	p, seen := testPayouts(t, "302", "302")
	if err := p.Disburse(context.Background(), req("+255712000111")); !errors.Is(err, payment.ErrPayoutRejected) {
		t.Fatalf("a refusal code is definite: %v", err)
	}
	unknown, _ := testPayouts(t, "577", "577")
	if err := unknown.Disburse(context.Background(), req("+255712000111")); err != nil {
		t.Fatalf("an unknown submit code is not a refusal: %v", err)
	}
	if err := p.Disburse(context.Background(), req("+254712000111")); !errors.Is(err, payment.ErrPayoutRejected) || len(*seen) != 1 {
		t.Fatalf("a number with no supported wallet is refused before sending: %v", err)
	}
	if st, _ := p.PayoutStatus(context.Background(), "po-1"); st.State != payment.PayoutFailed {
		t.Fatalf("query refusal: %+v", st)
	}
}

func TestUnknownQueryCodeStaysPending(t *testing.T) {
	p, _ := testPayouts(t, codeProcessing, "504")
	if st, _ := p.PayoutStatus(context.Background(), "po-1"); st.State != payment.PayoutPending {
		t.Fatalf("an unknown code may mean the money is in flight: %+v", st)
	}
}
