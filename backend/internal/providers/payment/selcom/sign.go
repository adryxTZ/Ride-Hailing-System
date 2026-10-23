package selcom

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"strings"
	"time"
)

// field is one signed request field; order matters for the signature.
type field struct{ key, value string }

// timestampLayout is ISO 8601 with a zone offset, as Selcom's examples show.
const timestampLayout = "2006-01-02T15:04:05-07:00"

// signingString joins the timestamp and the fields in signing order.
func signingString(ts string, fields []field) string {
	parts := make([]string, 0, len(fields)+1)
	parts = append(parts, "timestamp="+ts)
	for _, f := range fields {
		parts = append(parts, f.key+"="+f.value)
	}
	return strings.Join(parts, "&")
}

// digest is Base64(HMAC-SHA256(signingString, secret)).
func digest(secret, s string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(s))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// signHeaders sets Selcom's authentication headers for one request.
func signHeaders(h http.Header, apiKey, secret string, at time.Time, fields []field) {
	ts := at.Format(timestampLayout)
	names := make([]string, len(fields))
	for i, f := range fields {
		names[i] = f.key
	}
	h.Set("Authorization", "SELCOM "+base64.StdEncoding.EncodeToString([]byte(apiKey)))
	h.Set("Digest-Method", "HS256")
	h.Set("Digest", digest(secret, signingString(ts, fields)))
	h.Set("Timestamp", ts)
	h.Set("Signed-Fields", strings.Join(names, ","))
}
