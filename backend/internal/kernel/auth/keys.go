package auth

import (
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

// KeySet holds the active signing key plus any still-valid verification keys
// (so a rotation does not log everyone out).
type KeySet struct {
	ActiveKID string
	private   ed25519.PrivateKey
	public    map[string]ed25519.PublicKey // kid → key
}

// LoadOrGenerate reads <dir>/active.pem. When the directory is empty it
// generates a keypair: persisted if dir is writable, otherwise ephemeral with a
// loud warning (fine for `make run`, never for production).
func LoadOrGenerate(dir string) (*KeySet, error) {
	path := filepath.Join(dir, "active.pem")
	if raw, err := os.ReadFile(path); err == nil {
		return fromPEM(raw)
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, err
	}
	block := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	if err := os.MkdirAll(dir, 0o700); err == nil {
		if err := os.WriteFile(path, block, 0o600); err == nil {
			slog.Warn("generated new JWT signing key", "path", path)
		}
	} else {
		slog.Warn("JWT signing key is ephemeral; tokens will not survive a restart", "dir", dir)
	}
	return newKeySet(pub, priv), nil
}

func fromPEM(raw []byte) (*KeySet, error) {
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, fmt.Errorf("active.pem: not PEM")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("active.pem: %w", err)
	}
	priv, ok := key.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("active.pem: not an Ed25519 key")
	}
	return newKeySet(priv.Public().(ed25519.PublicKey), priv), nil
}

func newKeySet(pub ed25519.PublicKey, priv ed25519.PrivateKey) *KeySet {
	kid := keyID(pub)
	return &KeySet{ActiveKID: kid, private: priv, public: map[string]ed25519.PublicKey{kid: pub}}
}

// keyID derives a stable identifier from the public key bytes.
func keyID(pub ed25519.PublicKey) string {
	sum := sha256.Sum256(pub)
	return base64.RawURLEncoding.EncodeToString(sum[:8])
}

// JWKS returns the public keys in the JSON Web Key Set shape served at
// /v1/auth/jwks so other services (and the admin) can verify tokens.
func (k *KeySet) JWKS() map[string]any {
	keys := make([]map[string]any, 0, len(k.public))
	for kid, pub := range k.public {
		keys = append(keys, map[string]any{
			"kty": "OKP", "crv": "Ed25519", "alg": "EdDSA", "use": "sig",
			"kid": kid, "x": base64.RawURLEncoding.EncodeToString(pub),
		})
	}
	return map[string]any{"keys": keys}
}

// Derive returns a secret for another purpose (e.g. signing receipt links),
// derived from the active signing key so no extra secret has to be managed.
// Different purposes get unrelated secrets; rotating the JWT key rotates them too.
func (k *KeySet) Derive(purpose string) []byte {
	mac := hmac.New(sha256.New, k.private.Seed())
	mac.Write([]byte("derive|" + purpose))
	return mac.Sum(nil)
}
