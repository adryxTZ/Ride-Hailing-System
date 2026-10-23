package selcom

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/providers/payment"
)

// Business API paths (payouts). A separate Selcom product from checkout, with
// RSA-signed requests and its own credentials.
const (
	pathTransfer      = "/v1/transaction/process"
	pathTransferQuery = "/v1/transaction/query"
	// businessTimestamp is ISO 8601 UTC with milliseconds and a literal Z, as the Business API requires.
	businessTimestamp = "2006-01-02T15:04:05.000Z"
	transferPurpose   = "TRANSPORT"
	maxNameLen        = 100
	maxRemarksLen     = 140
)

// Business API result codes (anything else is a definite failure).
const (
	codePaid       = "000"
	codeProcessing = "111"
	codeQueued     = "927"
	codeAmbiguous  = "999" // outcome unknown: never treat as failed
	codeDuplicate  = "643" // transId already submitted: ask the query endpoint
)

// definiteFailure lists the result codes that mean the transfer was refused and
// will never be paid (bad wallet, invalid amount, insufficient float). Only
// these fail a payout, on submit or on query; anything else keeps it pending.
// To be confirmed against Selcom's Business API code list before launch.
var definiteFailure = map[string]bool{"301": true, "302": true, "303": true, "400": true, "403": true}

// minPayoutTZS is the smallest transfer the Business API sends.
var minPayoutTZS = decimal.NewFromInt(1000)

// walletByPrefix maps Tanzanian mobile prefixes to the Business API wallet code.
var walletByPrefix = map[string]string{
	"74": "MPESA", "75": "MPESA", "76": "MPESA",
	"68": "AIRTEL", "69": "AIRTEL", "78": "AIRTEL",
	"65": "TIGO", "67": "TIGO", "71": "TIGO", "77": "TIGO",
	"61": "HALOPESA", "62": "HALOPESA",
}

// PayoutConfig is what the payout client needs (from the environment).
type PayoutConfig struct {
	BaseURL       string
	APIKey        string
	PrivateKeyPEM []byte // RSA private key registered with Selcom
}

// Payouts is the Selcom Business API payout client.
type Payouts struct {
	cfg  PayoutConfig
	key  *rsa.PrivateKey
	http *http.Client
	now  func() time.Time
}

// NewPayouts returns the payout client, or ErrNotConfigured when credentials are missing.
func NewPayouts(cfg PayoutConfig) (*Payouts, error) {
	if cfg.BaseURL == "" || cfg.APIKey == "" || len(cfg.PrivateKeyPEM) == 0 {
		return nil, payment.ErrNotConfigured
	}
	key, err := parseRSA(cfg.PrivateKeyPEM)
	if err != nil {
		return nil, err
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	return &Payouts{cfg: cfg, key: key, http: &http.Client{Timeout: requestTimeout}, now: time.Now}, nil
}

func parseRSA(raw []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, fmt.Errorf("selcom payouts: private key is not PEM")
	}
	if k, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return k, nil
	}
	k, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("selcom payouts: %w", err)
	}
	rk, ok := k.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("selcom payouts: private key is not RSA")
	}
	return rk, nil
}

// Name implements payment.Payouter.
func (p *Payouts) Name() string { return Name }

// MinPayout implements payment.Payouter.
func (p *Payouts) MinPayout(currency string) decimal.Decimal {
	if currency == "TZS" {
		return minPayoutTZS
	}
	return decimal.Zero
}

// PayoutReference implements payment.Payouter: queried by our own transId.
func (p *Payouts) PayoutReference(r payment.DisburseRequest) string { return r.PayoutID }

// walletOf picks the wallet code from a +255 number.
func walletOf(phone string) (string, bool) {
	local := strings.TrimPrefix(phone, "+255")
	if len(local) != 9 || local == phone {
		return "", false
	}
	w, ok := walletByPrefix[local[:2]]
	return w, ok
}

// Disburse implements payment.Payouter. Only a clear refusal is ErrPayoutRejected;
// a timeout, a 999 or anything unreadable is ambiguous and left to PayoutStatus.
func (p *Payouts) Disburse(ctx context.Context, r payment.DisburseRequest) error {
	wallet, ok := walletOf(r.Phone)
	if !ok {
		return fmt.Errorf("%w: %s", payment.ErrPayoutRejected, payment.FailUnsupported)
	}
	fields := []field{
		{"transId", r.PayoutID}, {"recipientFiCode", wallet}, {"recipientAccount", strings.TrimPrefix(r.Phone, "+")},
		{"recipientName", truncate(r.RecipientName, maxNameLen)}, {"amount", r.Amount.StringFixed(0)}, {"purpose", transferPurpose},
	}
	if r.Note != "" {
		fields = append(fields, field{"remarks", truncate(r.Note, maxRemarksLen)})
	}
	res, err := p.call(ctx, http.MethodPost, pathTransfer, fields)
	if err != nil {
		return err
	}
	if definiteFailure[res.ResultCode] {
		return fmt.Errorf("%w: %s %s", payment.ErrPayoutRejected, res.ResultCode, res.Message)
	}
	return nil // sent, possibly sent or unclear: PayoutStatus decides
}

// PayoutStatus implements payment.Payouter by querying our transId.
func (p *Payouts) PayoutStatus(ctx context.Context, ref string) (payment.PayoutResult, error) {
	res, err := p.call(ctx, http.MethodGet, pathTransferQuery, []field{{"transId", ref}})
	if err != nil {
		return payment.PayoutResult{}, err
	}
	switch res.ResultCode {
	case codePaid:
		return payment.PayoutResult{State: payment.PayoutPaid, ProviderTxnID: res.reference()}, nil
	}
	if definiteFailure[res.ResultCode] {
		return payment.PayoutResult{State: payment.PayoutFailed, FailureCode: payment.FailPayoutRefused}, nil
	}
	// Processing, ambiguous, not-found-yet, throttled or anything unknown: the
	// money may be in flight, so it stays pending (stuck after a while, for Finance).
	return payment.PayoutResult{State: payment.PayoutPending}, nil
}

// reference is Selcom's own id for a transfer, when it gives one.
func (e envelope) reference() string {
	for _, k := range []string{"reference", "transid", "transId"} {
		if v, ok := e.first()[k]; ok && v != nil {
			return fmt.Sprint(v)
		}
	}
	return ""
}

// call signs (RSA-SHA256) and sends one Business API request.
func (p *Payouts) call(ctx context.Context, method, path string, fields []field) (envelope, error) {
	req, err := p.request(ctx, method, path, fields)
	if err != nil {
		return envelope{}, err
	}
	resp, err := p.http.Do(req)
	if err != nil {
		return envelope{}, fmt.Errorf("%w: %v", payment.ErrUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= http.StatusInternalServerError || resp.StatusCode == http.StatusTooManyRequests {
		return envelope{}, fmt.Errorf("%w: HTTP %d", payment.ErrUnavailable, resp.StatusCode) // never read as a refusal
	}
	var out envelope
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return envelope{}, fmt.Errorf("%w: non-JSON response (HTTP %d)", payment.ErrUnavailable, resp.StatusCode)
	}
	return out, nil
}

func (p *Payouts) request(ctx context.Context, method, path string, fields []field) (*http.Request, error) {
	target := p.cfg.BaseURL + path
	var body io.Reader
	if method == http.MethodGet {
		q := url.Values{}
		for _, f := range fields {
			q.Set(f.key, f.value)
		}
		target += "?" + q.Encode()
	} else {
		body = bytes.NewReader(orderedJSON(fields))
	}
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return nil, err
	}
	ts := p.now().UTC().Format(businessTimestamp)
	sig, err := p.sign(signingString(ts, fields))
	if err != nil {
		return nil, err
	}
	names := make([]string, len(fields))
	for i, f := range fields {
		names[i] = f.key
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("api-key", p.cfg.APIKey)
	req.Header.Set("timestamp", ts)
	req.Header.Set("digest", sig)
	req.Header.Set("signed-fields", strings.Join(names, ","))
	return req, nil
}

// sign is Base64(RSA-PKCS1v15(SHA-256(s))).
func (p *Payouts) sign(s string) (string, error) {
	sum := sha256.Sum256([]byte(s))
	sig, err := rsa.SignPKCS1v15(rand.Reader, p.key, crypto.SHA256, sum[:])
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}

// orderedJSON writes the fields as a JSON object in signing order (a Go map would reorder them).
func orderedJSON(fields []field) []byte {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, f := range fields {
		if i > 0 {
			b.WriteByte(',')
		}
		k, _ := json.Marshal(f.key)
		v, _ := json.Marshal(f.value)
		b.Write(k)
		b.WriteByte(':')
		b.Write(v)
	}
	b.WriteByte('}')
	return b.Bytes()
}

func truncate(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}
