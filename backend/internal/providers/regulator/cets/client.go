package cets

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/kalen-limited/hauler-rider/backend/internal/providers/regulator"
)

// Name is the regulator key this adapter submits for.
const Name = "latra"

// Defaults the configuration loader applies (NEXA_REGULATOR_LATRA_*).
const (
	DefaultAudience = "cets"
	DefaultTimeout  = 15 * time.Second
)

// Endpoint paths, relative to Config.BaseURL (API doc v2.1.1).
const (
	pathVehicle = "rideVehicle/submitVehicle"
	pathDriver  = "rideDriver/submitDriver"
	pathTrip    = "rideHailing/submitTrip"
	pathReport  = "rideReport/submitReport"
)

// maxResponseBytes bounds how much of a CeTS answer is read.
const maxResponseBytes = 1 << 20

// Config is what the adapter needs; values come from NEXA_REGULATOR_LATRA_*.
type Config struct {
	BaseURL       string        // CeTS base URL; paths are appended to it
	Issuer        string        // our domain as registered with CeTS (iss and sub)
	Audience      string        // aud claim (DefaultAudience unless LATRA says otherwise)
	KID           string        // key id in the token header and the JWKS
	PrivateKeyPEM []byte        // RSA private key, PKCS#8 or PKCS#1 PEM
	JWKSURI       string        // public URL of our JWKS, served by discovery
	Timeout       time.Duration // per request; zero means DefaultTimeout
}

// Client is the CeTS adapter; safe for concurrent use.
type Client struct {
	baseURL string
	jwksURI string
	signer  signer
	http    *http.Client
	now     func() time.Time
}

// New returns a client, or regulator.ErrNotConfigured when a required value is
// missing or the private key cannot be used.
func New(cfg Config) (*Client, error) {
	if cfg.BaseURL == "" || cfg.Issuer == "" || cfg.Audience == "" || cfg.KID == "" ||
		len(cfg.PrivateKeyPEM) == 0 || cfg.JWKSURI == "" {
		return nil, regulator.ErrNotConfigured
	}
	key, err := parsePrivateKey(cfg.PrivateKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", regulator.ErrNotConfigured, err)
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	return &Client{
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
		jwksURI: cfg.JWKSURI,
		signer:  signer{key: key, kid: cfg.KID, issuer: cfg.Issuer, audience: cfg.Audience},
		http:    &http.Client{Timeout: timeout},
		now:     time.Now,
	}, nil
}

// Name implements regulator.Regulator.
func (c *Client) Name() string { return Name }

// JWKS returns our public key as a JWKS document ({"keys": [{kty, use, alg,
// kid, n, e}]}) for GET /v1/regulator/jwks.
func (c *Client) JWKS() map[string]any { return c.signer.jwks() }

// JWKSURI returns the public JWKS URL for /.well-known/tz-e-ticketing-server.
func (c *Client) JWKSURI() string { return c.jwksURI }

// RegisterDriver implements regulator.Regulator.
func (c *Client) RegisterDriver(ctx context.Context, d regulator.DriverRecord) (regulator.Ack, error) {
	return c.submit(ctx, pathDriver, driverPayload(d), false)
}

// RegisterVehicle implements regulator.Regulator.
func (c *Client) RegisterVehicle(ctx context.Context, v regulator.VehicleRecord) (regulator.Ack, error) {
	body, err := vehiclePayload(v)
	if err != nil {
		return regulator.Ack{}, err
	}
	return c.submit(ctx, pathVehicle, body, false)
}

// SubmitTrip implements regulator.Regulator.
func (c *Client) SubmitTrip(ctx context.Context, t regulator.TripRecord) (regulator.Ack, error) {
	body, err := tripPayload(t)
	if err != nil {
		return regulator.Ack{}, err
	}
	return c.submit(ctx, pathTrip, body, false)
}

// SubmitDailyReport implements regulator.Regulator.
func (c *Client) SubmitDailyReport(ctx context.Context, r regulator.DailyReport) (regulator.Ack, error) {
	body, err := reportPayload(r)
	if err != nil {
		return regulator.Ack{}, err
	}
	return c.submit(ctx, pathReport, body, true)
}

// submit posts payload to path once and maps the answer to the port's terms.
func (c *Client) submit(ctx context.Context, path string, payload any, report bool) (regulator.Ack, error) {
	raw, err := c.post(ctx, path, payload)
	if err != nil {
		return regulator.Ack{}, err
	}
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil || env.StatusCode == nil {
		return regulator.Ack{}, unavailable(path, "no CeTS envelope in the response")
	}
	if env.StatusCode.int() != codeSuccess {
		return regulator.Ack{}, rejection(env, raw, report)
	}
	return regulator.Ack{Reference: env.reference(), Code: strconv.Itoa(codeSuccess)}, nil
}

// post sends one signed request; any transport failure or non-2xx status is
// regulator.ErrUnavailable. It returns the raw body of a 2xx answer.
func (c *Client) post(ctx context.Context, path string, payload any) ([]byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("cets: encode %s: %w", path, err)
	}
	token, err := c.signer.token(c.now())
	if err != nil {
		return nil, fmt.Errorf("cets: sign %s: %w", path, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/"+path, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("cets: build %s: %w", path, err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "*/*")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, unavailable(path, err.Error())
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, unavailable(path, "unreadable body: "+err.Error())
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, unavailable(path, fmt.Sprintf("HTTP %d: %s", resp.StatusCode, truncate(string(raw), maxRawMessage)))
	}
	return raw, nil
}

// unavailable wraps regulator.ErrUnavailable with what went wrong.
func unavailable(path, detail string) error {
	return fmt.Errorf("%w: cets %s: %s", regulator.ErrUnavailable, path, detail)
}

// envelope is CeTS's response wrapper.
type envelope struct {
	Status     string          `json:"status"`
	StatusCode *codeValue      `json:"statusCode"`
	Message    string          `json:"message"`
	Data       json.RawMessage `json:"data"`
}

// reference reads data.latraReferenceNumber; a missing or odd data is "".
func (e envelope) reference() string {
	var data struct {
		Reference string `json:"latraReferenceNumber"`
	}
	_ = json.Unmarshal(e.Data, &data) // registrations may answer without a reference
	return data.Reference
}

// codeValue accepts statusCode as a JSON number or a numeric string.
type codeValue int

// UnmarshalJSON implements json.Unmarshaler.
func (v *codeValue) UnmarshalJSON(b []byte) error {
	n, err := strconv.Atoi(strings.Trim(string(b), `"`))
	if err != nil {
		return err
	}
	*v = codeValue(n)
	return nil
}

// int returns the code, zero for a nil value.
func (v *codeValue) int() int {
	if v == nil {
		return 0
	}
	return int(*v)
}
