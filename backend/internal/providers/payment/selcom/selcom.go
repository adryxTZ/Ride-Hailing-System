// Package selcom is the payment.Gateway adapter for Selcom's Checkout API
// (Tanzania): USSD push collections and order-status queries.
//
// Files: selcom.go (client, endpoints, status mapping), sign.go (request signing).
// No live calls are made in tests; the HTTP shape is checked with httptest.
package selcom

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/providers/payment"
)

// Name is this adapter's key.
const Name = "selcom"

// Checkout API paths.
const (
	pathCreateOrder   = "/v1/checkout/create-order-minimal"
	pathWalletPayment = "/v1/checkout/wallet-payment"
	pathOrderStatus   = "/v1/checkout/order-status"
	requestTimeout    = 20 * time.Second
	resultOK          = "000" // Selcom's "accepted" result code
)

// minTZS is the smallest collection Selcom accepts.
var minTZS = decimal.NewFromInt(1000)

// Config is what the adapter needs; all values come from the environment.
type Config struct {
	BaseURL    string
	APIKey     string
	APISecret  string
	Vendor     string
	BuyerName  string // shown on the payer's prompt when no name is known (brand display name)
	BuyerEmail string // Selcom requires one; the brand's support address
}

// Client is the Selcom adapter.
type Client struct {
	cfg  Config
	http *http.Client
	now  func() time.Time
}

// New returns a client, or ErrNotConfigured when any credential is missing.
func New(cfg Config) (*Client, error) {
	if cfg.BaseURL == "" || cfg.APIKey == "" || cfg.APISecret == "" || cfg.Vendor == "" {
		return nil, payment.ErrNotConfigured
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	return &Client{cfg: cfg, http: &http.Client{Timeout: requestTimeout}, now: time.Now}, nil
}

// Name implements payment.Gateway.
func (c *Client) Name() string { return Name }

// MinAmount implements payment.Gateway.
func (c *Client) MinAmount(currency string) decimal.Decimal {
	if currency == "TZS" {
		return minTZS
	}
	return decimal.Zero
}

// Reference implements payment.Gateway: Selcom is queried by our own order id.
func (c *Client) Reference(r payment.CollectRequest) string { return r.OrderID }

// Collect creates the order, then pushes the USSD prompt. Selcom's "000" on the
// push only means the prompt was sent; COLLECTED comes from Status alone.
func (c *Client) Collect(ctx context.Context, r payment.CollectRequest) (payment.CollectResult, error) {
	msisdn := strings.TrimPrefix(r.Phone, "+") // Selcom wants 255…, not +255…
	order := []field{
		{"vendor", c.cfg.Vendor}, {"order_id", r.OrderID}, {"buyer_email", c.cfg.BuyerEmail}, {"buyer_name", c.cfg.BuyerName},
		{"buyer_phone", msisdn}, {"amount", r.Amount.StringFixed(0)}, {"currency", r.Currency}, {"no_of_items", "1"},
	}
	if r.CallbackURL != "" {
		order = append(order, field{"webhook", base64.StdEncoding.EncodeToString([]byte(r.CallbackURL))})
	}
	res, err := c.call(ctx, http.MethodPost, pathCreateOrder, order)
	if err != nil {
		return payment.CollectResult{}, err
	}
	if res.ResultCode != resultOK {
		return payment.CollectResult{}, fmt.Errorf("%w: create order %s %s", payment.ErrUnavailable, res.ResultCode, res.Message)
	}
	push := []field{{"transid", r.OrderID}, {"order_id", r.OrderID}, {"msisdn", msisdn}}
	if res, err = c.call(ctx, http.MethodPost, pathWalletPayment, push); err != nil {
		return payment.CollectResult{}, err
	}
	if res.ResultCode != resultOK {
		return payment.CollectResult{}, fmt.Errorf("%w: wallet payment %s %s", payment.ErrUnavailable, res.ResultCode, res.Message)
	}
	return payment.CollectResult{}, nil
}

// Status implements payment.Gateway by asking order-status for our order id.
func (c *Client) Status(ctx context.Context, ref string) (payment.StatusResult, error) {
	res, err := c.call(ctx, http.MethodGet, pathOrderStatus, []field{{"order_id", ref}})
	if err != nil {
		return payment.StatusResult{}, err
	}
	st := payment.StatusResult{State: mapPaymentStatus(res.paymentStatus())}
	if st.State == payment.StateFailed {
		st.FailureCode = mapFailure(res.paymentStatus())
	}
	if st.State == payment.StateCollected {
		st.ProviderTxnID = res.transID()
	}
	return st, nil
}

// mapPaymentStatus turns Selcom's order payment_status into our three states.
// Anything unknown stays PENDING (and is logged) so no money is ever assumed.
func mapPaymentStatus(s string) string {
	switch strings.ToUpper(s) {
	case "COMPLETED", "PAID", "SUCCESS":
		return payment.StateCollected
	case "CANCELLED", "CANCELED", "USERCANCELED", "USERCANCELLED", "REJECTED", "FAILED", "EXPIRED":
		return payment.StateFailed
	case "PENDING", "INPROGRESS", "USERCANCELED_PENDING", "":
		return payment.StatePending
	}
	slog.Warn("selcom: unknown payment_status treated as pending", "status", s)
	return payment.StatePending
}

// mapFailure picks our failure code for a failed order.
func mapFailure(s string) string {
	switch strings.ToUpper(s) {
	case "CANCELLED", "CANCELED", "USERCANCELED", "USERCANCELLED", "REJECTED":
		return payment.FailRejectedByUser
	}
	return payment.FailGateway
}

// envelope is the common shape of Selcom responses.
type envelope struct {
	ResultCode string            `json:"resultcode"`
	Message    string            `json:"message"`
	Result     string            `json:"result"`
	Data       []json.RawMessage `json:"data"`
}

func (e envelope) first() map[string]any {
	if len(e.Data) == 0 {
		return nil
	}
	var m map[string]any
	_ = json.Unmarshal(e.Data[0], &m)
	return m
}

func (e envelope) paymentStatus() string { return fmt.Sprint(e.first()["payment_status"]) }
func (e envelope) transID() string       { return fmt.Sprint(e.first()["transid"]) }

// call signs and sends one request. GET fields go in the query, POST fields as JSON.
func (c *Client) call(ctx context.Context, method, path string, fields []field) (envelope, error) {
	var body io.Reader
	target := c.cfg.BaseURL + path
	if method == http.MethodGet {
		q := url.Values{}
		for _, f := range fields {
			q.Set(f.key, f.value)
		}
		target += "?" + q.Encode()
	} else {
		m := map[string]string{}
		for _, f := range fields {
			m[f.key] = f.value
		}
		raw, _ := json.Marshal(m)
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return envelope{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	signHeaders(req.Header, c.cfg.APIKey, c.cfg.APISecret, c.now(), fields)
	resp, err := c.http.Do(req)
	if err != nil {
		return envelope{}, fmt.Errorf("%w: %v", payment.ErrUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()
	var out envelope
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return envelope{}, fmt.Errorf("%w: non-JSON response (HTTP %d)", payment.ErrUnavailable, resp.StatusCode)
	}
	return out, nil
}
