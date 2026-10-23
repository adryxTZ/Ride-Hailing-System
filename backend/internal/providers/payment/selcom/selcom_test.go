package selcom

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/providers/payment"
)

func TestNewRequiresCredentials(t *testing.T) {
	if _, err := New(Config{BaseURL: "https://x"}); !errors.Is(err, payment.ErrNotConfigured) {
		t.Fatalf("want ErrNotConfigured, got %v", err)
	}
}

func TestSigningHeaders(t *testing.T) {
	at := time.Date(2026, 10, 21, 9, 30, 46, 0, time.FixedZone("EAT", 3*3600))
	fields := []field{{"vendor", "V1"}, {"order_id", "o-1"}}
	h := http.Header{}
	signHeaders(h, "key", "secret", at, fields)
	mac := hmac.New(sha256.New, []byte("secret"))
	mac.Write([]byte("timestamp=2026-10-21T09:30:46+03:00&vendor=V1&order_id=o-1"))
	want := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	if h.Get("Digest") != want || h.Get("Timestamp") != "2026-10-21T09:30:46+03:00" || h.Get("Signed-Fields") != "vendor,order_id" ||
		h.Get("Authorization") != "SELCOM "+base64.StdEncoding.EncodeToString([]byte("key")) || h.Get("Digest-Method") != "HS256" {
		t.Fatalf("headers wrong: %v", h)
	}
}

func TestCollectAndStatus(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if r.Header.Get("Digest") == "" || r.Header.Get("Authorization") == "" {
			t.Errorf("unsigned request to %s", r.URL.Path)
		}
		switch r.URL.Path {
		case pathCreateOrder, pathWalletPayment:
			_ = json.NewEncoder(w).Encode(map[string]any{"resultcode": "000", "message": "SUCCESS"})
		case pathOrderStatus:
			_ = json.NewEncoder(w).Encode(map[string]any{"resultcode": "000", "data": []map[string]any{{"payment_status": r.URL.Query().Get("order_id"), "transid": "TX9"}}})
		}
	}))
	defer srv.Close()
	c, err := New(Config{BaseURL: srv.URL, APIKey: "k", APISecret: "s", Vendor: "v", BuyerEmail: "e@x", BuyerName: "n"})
	if err != nil {
		t.Fatal(err)
	}
	res, err := c.Collect(context.Background(), payment.CollectRequest{OrderID: "COMPLETED", Phone: "+255700000000", Amount: decimal.NewFromInt(1500), Currency: "TZS"})
	if err != nil || res.ProviderRef != "" || c.Reference(payment.CollectRequest{OrderID: "COMPLETED"}) != "COMPLETED" || len(paths) != 2 {
		t.Fatalf("collect: %+v %v paths=%v", res, err, paths)
	}
	cases := map[string]payment.StatusResult{
		"COMPLETED":    {State: payment.StateCollected, ProviderTxnID: "TX9"},
		"USERCANCELED": {State: payment.StateFailed, FailureCode: payment.FailRejectedByUser},
		"EXPIRED":      {State: payment.StateFailed, FailureCode: payment.FailGateway},
		"INPROGRESS":   {State: payment.StatePending},
		"WEIRD":        {State: payment.StatePending},
	}
	for ref, want := range cases {
		got, err := c.Status(context.Background(), ref)
		if err != nil || got != want {
			t.Fatalf("%s: got %+v %v, want %+v", ref, got, err, want)
		}
	}
}
