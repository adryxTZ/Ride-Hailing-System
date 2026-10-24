package storage

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// Local stores files under Dir and signs URLs with Secret; the API serves them
// at BaseURL + "/files/{key}?m=<method>&exp=<unix>&sig=<hmac>".
type Local struct {
	Dir     string
	BaseURL string // e.g. http://192.168.1.10:8080/v1
	Secret  []byte
	now     func() time.Time
}

// NewLocal returns a disk store.
func NewLocal(dir, baseURL string, secret []byte) *Local {
	return &Local{Dir: dir, BaseURL: baseURL, Secret: secret, now: time.Now}
}

func (l *Local) sign(key, method string, exp int64) string {
	m := hmac.New(sha256.New, l.Secret)
	_, _ = fmt.Fprintf(m, "%s\n%s\n%d", method, key, exp)
	return hex.EncodeToString(m.Sum(nil))
}

func (l *Local) signed(key, method string, expires time.Duration) string {
	exp := l.now().Add(expires).Unix()
	q := url.Values{"m": {method}, "exp": {strconv.FormatInt(exp, 10)}, "sig": {l.sign(key, method, exp)}}
	return l.BaseURL + "/files/" + key + "?" + q.Encode()
}

// UploadURL implements Store.
func (l *Local) UploadURL(_ context.Context, key, _ string, expires time.Duration) (string, error) {
	if !ValidKey(key) {
		return "", ErrBadKey
	}
	return l.signed(key, "PUT", expires), nil
}

// DownloadURL implements Store.
func (l *Local) DownloadURL(_ context.Context, key string, expires time.Duration) (string, error) {
	if !ValidKey(key) {
		return "", ErrBadKey
	}
	return l.signed(key, "GET", expires), nil
}

// Size implements Store.
func (l *Local) Size(_ context.Context, key string) (int64, error) {
	if !ValidKey(key) {
		return 0, ErrBadKey
	}
	fi, err := os.Stat(filepath.Join(l.Dir, filepath.FromSlash(key)))
	if errors.Is(err, os.ErrNotExist) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, err
	}
	return fi.Size(), nil
}

// Verify implements Server: right method, not expired, signature matches.
func (l *Local) Verify(key, method string, exp int64, sig string) bool {
	if !ValidKey(key) || l.now().Unix() > exp {
		return false
	}
	return hmac.Equal([]byte(sig), []byte(l.sign(key, method, exp)))
}

// Write implements Server; refuses bodies over maxBytes.
func (l *Local) Write(key string, r io.Reader, maxBytes int64) error {
	path := filepath.Join(l.Dir, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	tmp := path + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	n, err := io.Copy(f, io.LimitReader(r, maxBytes+1))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil && n > maxBytes {
		err = errors.New("storage: file too large")
	}
	if err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

// Open implements Server.
func (l *Local) Open(key string) (io.ReadCloser, string, error) {
	f, err := os.Open(filepath.Join(l.Dir, filepath.FromSlash(key)))
	if err != nil {
		return nil, "", err
	}
	return f, mime.TypeByExtension(filepath.Ext(key)), nil
}

// Put implements Writer (server-side files such as privacy exports).
func (l *Local) Put(_ context.Context, key string, r io.Reader, maxBytes int64) error {
	if !ValidKey(key) {
		return ErrBadKey
	}
	return l.Write(key, r, maxBytes)
}

// Delete implements Writer; a file already gone is not an error.
func (l *Local) Delete(_ context.Context, key string) error {
	if !ValidKey(key) {
		return ErrBadKey
	}
	err := os.Remove(filepath.Join(l.Dir, filepath.FromSlash(key)))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
