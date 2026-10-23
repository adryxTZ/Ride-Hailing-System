package cets

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"math/big"
	"strings"
	"time"
)

// Token shape (CeTS API doc v2.1.1, §1–2).
const (
	jwtAlgorithm = "RS256"
	jwtType      = "JWT"
	tokenTTL     = 300 * time.Second // exp = iat + 300 s
	jtiBytes     = 16
	minKeyBits   = 2048
)

// errBadKey is returned for a private key that cannot be used.
var errBadKey = errors.New("cets: private key is not a usable RSA PEM key")

// jwtHeader is the JOSE header.
type jwtHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
	Kid string `json:"kid"`
}

// jwtClaims are the claims CeTS checks; iss and sub are both our issuer.
type jwtClaims struct {
	Iss string `json:"iss"`
	Sub string `json:"sub"`
	Aud string `json:"aud"`
	Iat int64  `json:"iat"`
	Exp int64  `json:"exp"`
	Jti string `json:"jti"`
}

// signer mints one fresh token per request; nothing is cached.
type signer struct {
	key      *rsa.PrivateKey
	kid      string
	issuer   string
	audience string
}

// token returns a signed compact JWT issued at now.
func (s signer) token(now time.Time) (string, error) {
	jti := make([]byte, jtiBytes)
	if _, err := rand.Read(jti); err != nil {
		return "", err
	}
	header, err := json.Marshal(jwtHeader{Alg: jwtAlgorithm, Typ: jwtType, Kid: s.kid})
	if err != nil {
		return "", err
	}
	claims, err := json.Marshal(jwtClaims{
		Iss: s.issuer, Sub: s.issuer, Aud: s.audience,
		Iat: now.Unix(), Exp: now.Add(tokenTTL).Unix(), Jti: hex.EncodeToString(jti),
	})
	if err != nil {
		return "", err
	}
	signingInput := b64(header) + "." + b64(claims)
	digest := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, s.key, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	return signingInput + "." + b64(sig), nil
}

// jwks is the public JWKS document for the discovery route.
func (s signer) jwks() map[string]any {
	pub := s.key.PublicKey
	return map[string]any{"keys": []any{map[string]any{
		"kty": "RSA",
		"use": "sig",
		"alg": jwtAlgorithm,
		"kid": s.kid,
		"n":   b64(pub.N.Bytes()),
		"e":   b64(big.NewInt(int64(pub.E)).Bytes()),
	}}}
}

// parsePrivateKey reads a PKCS#8 or PKCS#1 RSA key of at least minKeyBits.
// Literal "\n" sequences (PEMs pasted into env vars) are restored first.
func parsePrivateKey(pemBytes []byte) (*rsa.PrivateKey, error) {
	text := string(pemBytes)
	if strings.Contains(text, `\n`) && !strings.Contains(text, "\n") {
		text = strings.ReplaceAll(text, `\n`, "\n")
	}
	block, _ := pem.Decode([]byte(strings.TrimSpace(text)))
	if block == nil {
		return nil, errBadKey
	}
	key, err := decodeRSA(block.Bytes)
	if err != nil || key.N.BitLen() < minKeyBits {
		return nil, errBadKey
	}
	return key, nil
}

// decodeRSA tries PKCS#8 then PKCS#1.
func decodeRSA(der []byte) (*rsa.PrivateKey, error) {
	if k, err := x509.ParsePKCS8PrivateKey(der); err == nil {
		if rk, ok := k.(*rsa.PrivateKey); ok {
			return rk, nil
		}
		return nil, errBadKey
	}
	return x509.ParsePKCS1PrivateKey(der)
}

// b64 is unpadded base64url, as JOSE requires.
func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }
