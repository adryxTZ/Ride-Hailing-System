package receipts

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strconv"
	"time"
)

// LinkTTL is how long a PDF link works: long enough to open or share it right
// away, short enough that a forwarded link soon stops exposing the trip.
const LinkTTL = 15 * time.Minute

// signer signs and checks PDF links: HMAC over receipt id, language and expiry.
type signer struct{ key []byte }

func (s signer) sign(id, lang string, exp int64) string {
	mac := hmac.New(sha256.New, s.key)
	mac.Write([]byte(id + "|" + lang + "|" + strconv.FormatInt(exp, 10)))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// verify checks a link's signature (constant time) and expiry.
func (s signer) verify(id, lang, expRaw, sig string, now time.Time) error {
	exp, err := strconv.ParseInt(expRaw, 10, 64)
	if err != nil || now.Unix() > exp {
		return ErrLinkInvalid
	}
	if !hmac.Equal([]byte(s.sign(id, lang, exp)), []byte(sig)) {
		return ErrLinkInvalid
	}
	return nil
}
