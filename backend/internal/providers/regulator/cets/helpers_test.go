package cets

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"sync"
	"testing"
	"time"
)

var (
	keyOnce sync.Once
	testKey *rsa.PrivateKey
)

// key returns one RSA key shared by the package's tests.
func key(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	keyOnce.Do(func() {
		k, err := rsa.GenerateKey(rand.Reader, minKeyBits)
		if err != nil {
			panic(err)
		}
		testKey = k
	})
	return testKey
}

// pkcs8PEM encodes the test key as PKCS#8 PEM.
func pkcs8PEM(t *testing.T) []byte {
	der, err := x509.MarshalPKCS8PrivateKey(key(t))
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
}

// testConfig is a complete configuration against baseURL.
func testConfig(t *testing.T, baseURL string) Config {
	return Config{
		BaseURL: baseURL, Issuer: "ops.example.test", Audience: DefaultAudience, KID: "kid-1",
		PrivateKeyPEM: pkcs8PEM(t), JWKSURI: "https://ops.example.test/v1/regulator/jwks",
		Timeout: 2 * time.Second,
	}
}

// newClient builds a client against baseURL with a fixed clock.
func newClient(t *testing.T, baseURL string) *Client {
	t.Helper()
	c, err := New(testConfig(t, baseURL))
	if err != nil {
		t.Fatal(err)
	}
	c.now = func() time.Time { return fixedNow }
	return c
}

// fixedNow is the clock the client tests sign with.
var fixedNow = time.Date(2026, 10, 23, 9, 0, 0, 0, time.UTC)
