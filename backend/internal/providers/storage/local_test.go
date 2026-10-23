package storage

import (
	"bytes"
	"context"
	"io"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

const testKey = "documents/0190f0b6-1111-7000-8000-000000000001/0190f0b6-2222-7000-8000-000000000002.jpg"

func TestLocalSignedRoundTrip(t *testing.T) {
	l := NewLocal(t.TempDir(), "http://api/v1", []byte("secret"))
	raw, err := l.UploadURL(context.Background(), testKey, "image/jpeg", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(raw)
	exp, _ := strconv.ParseInt(u.Query().Get("exp"), 10, 64)
	if !l.Verify(testKey, "PUT", exp, u.Query().Get("sig")) {
		t.Fatal("valid upload signature rejected")
	}
	if l.Verify(testKey, "GET", exp, u.Query().Get("sig")) {
		t.Fatal("PUT signature accepted for GET")
	}
	if err := l.Write(testKey, strings.NewReader("jpeg-bytes"), MaxUploadBytes); err != nil {
		t.Fatal(err)
	}
	f, _, err := l.Open(testKey)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(f)
	_ = f.Close()
	if !bytes.Equal(got, []byte("jpeg-bytes")) {
		t.Fatalf("read back %q", got)
	}
}

func TestLocalRejectsTraversalAndExpiry(t *testing.T) {
	l := NewLocal(t.TempDir(), "http://api/v1", []byte("secret"))
	if _, err := l.UploadURL(context.Background(), "../../etc/passwd", "image/jpeg", time.Minute); err == nil {
		t.Fatal("traversal key accepted")
	}
	if l.Verify(testKey, "GET", time.Now().Add(-time.Minute).Unix(), l.sign(testKey, "GET", time.Now().Add(-time.Minute).Unix())) {
		t.Fatal("expired signature accepted")
	}
	if err := l.Write(testKey, strings.NewReader("12345"), 4); err == nil {
		t.Fatal("oversized upload accepted")
	}
}
