package cets

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"math/big"
	"strings"
	"testing"

	"github.com/kalen-limited/hauler-rider/backend/internal/providers/regulator"
)

// verifyJWT checks the RS256 signature with pub and decodes header and claims.
func verifyJWT(t *testing.T, token string, pub *rsa.PublicKey) (jwtHeader, jwtClaims) {
	t.Helper()
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("not a compact JWT: %q", token)
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(pub, crypto.SHA256, digest[:], sig); err != nil {
		t.Fatalf("signature does not verify: %v", err)
	}
	var h jwtHeader
	var c jwtClaims
	decodeSegment(t, parts[0], &h)
	decodeSegment(t, parts[1], &c)
	return h, c
}

func decodeSegment(t *testing.T, seg string, v any) {
	t.Helper()
	b, err := base64.RawURLEncoding.DecodeString(seg)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatal(err)
	}
}

func TestTokenVerifiesWithPublicKey(t *testing.T) {
	c := newClient(t, "https://cets.test")
	tok, err := c.signer.token(fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	h, cl := verifyJWT(t, tok, &key(t).PublicKey)
	if h.Alg != "RS256" || h.Kid != "kid-1" || h.Typ != "JWT" {
		t.Fatalf("header: %+v", h)
	}
	if cl.Iss != "ops.example.test" || cl.Sub != cl.Iss || cl.Aud != "cets" || cl.Iat != fixedNow.Unix() ||
		cl.Exp-cl.Iat != 300 || len(cl.Jti) != 2*jtiBytes {
		t.Fatalf("claims: %+v", cl)
	}
	again, _ := c.signer.token(fixedNow)
	if _, cl2 := verifyJWT(t, again, &key(t).PublicKey); cl2.Jti == cl.Jti {
		t.Fatal("jti must differ per token")
	}
}

func TestJWKSExposesPublicKey(t *testing.T) {
	c := newClient(t, "https://cets.test")
	keys, ok := c.JWKS()["keys"].([]any)
	if !ok || len(keys) != 1 {
		t.Fatalf("jwks: %v", c.JWKS())
	}
	jwk := keys[0].(map[string]any)
	n, _ := base64.RawURLEncoding.DecodeString(jwk["n"].(string))
	e, _ := base64.RawURLEncoding.DecodeString(jwk["e"].(string))
	pub := &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(new(big.Int).SetBytes(e).Int64())}
	if jwk["kid"] != "kid-1" || jwk["alg"] != "RS256" || jwk["kty"] != "RSA" || !pub.Equal(&key(t).PublicKey) {
		t.Fatalf("jwk: %v", jwk)
	}
	if c.JWKSURI() != "https://ops.example.test/v1/regulator/jwks" {
		t.Fatalf("jwks uri: %s", c.JWKSURI())
	}
}

func TestNewNotConfigured(t *testing.T) {
	full := testConfig(t, "https://cets.test")
	cases := map[string]func(*Config){
		"base url": func(c *Config) { c.BaseURL = "" },
		"issuer":   func(c *Config) { c.Issuer = "" },
		"audience": func(c *Config) { c.Audience = "" },
		"kid":      func(c *Config) { c.KID = "" },
		"key":      func(c *Config) { c.PrivateKeyPEM = nil },
		"jwks uri": func(c *Config) { c.JWKSURI = "" },
		"bad key": func(c *Config) {
			c.PrivateKeyPEM = []byte("-----BEGIN PRIVATE KEY-----\nnope\n-----END PRIVATE KEY-----")
		},
	}
	for name, mutate := range cases {
		cfg := full
		mutate(&cfg)
		if _, err := New(cfg); !errors.Is(err, regulator.ErrNotConfigured) {
			t.Errorf("%s: want ErrNotConfigured, got %v", name, err)
		}
	}
}

func TestKeyFormats(t *testing.T) {
	pkcs1 := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key(t))})
	escaped := []byte(strings.ReplaceAll(string(pkcs8PEM(t)), "\n", `\n`))
	for name, p := range map[string][]byte{"pkcs1": pkcs1, "escaped newlines": escaped} {
		if _, err := parsePrivateKey(p); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}
