package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// AccessTTL is short on purpose; refresh tokens carry the long session.
const AccessTTL = 15 * time.Minute

// Claims is the access-token payload. Role and brand are explicit so no
// endpoint has to infer who the caller is from database joins.
type Claims struct {
	jwt.RegisteredClaims
	Brand   string   `json:"brand"`
	Roles   []string `json:"roles"`
	App     string   `json:"app"`           // rider | driver | admin
	Session string   `json:"sid,omitempty"` // the sign-in session (refresh family) this token belongs to
}

// Issue signs an access token for the given subject and sign-in session.
func (k *KeySet) Issue(subject, brand, app, session string, roles []string, now time.Time) (string, error) {
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   subject,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(AccessTTL)),
		},
		Brand:   brand,
		Roles:   roles,
		App:     app,
		Session: session,
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	tok.Header["kid"] = k.ActiveKID
	return tok.SignedString(k.private)
}

// Verify parses and validates a token, resolving the key by kid.
func (k *KeySet) Verify(raw string) (*Claims, error) {
	var claims Claims
	_, err := jwt.ParseWithClaims(raw, &claims, func(t *jwt.Token) (any, error) {
		if t.Method.Alg() != jwt.SigningMethodEdDSA.Alg() {
			return nil, fmt.Errorf("unexpected alg %s", t.Method.Alg())
		}
		kid, _ := t.Header["kid"].(string)
		pub, ok := k.public[kid]
		if !ok {
			return nil, errors.New("unknown kid")
		}
		return pub, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodEdDSA.Alg()}))
	if err != nil {
		return nil, err
	}
	return &claims, nil
}
