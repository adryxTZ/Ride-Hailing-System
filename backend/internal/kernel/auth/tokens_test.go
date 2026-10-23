package auth

import (
	"testing"
	"time"
)

func TestIssueVerifyRoundTrip(t *testing.T) {
	ks, err := LoadOrGenerate(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	tok, err := ks.Issue("user-1", "demo", "rider", "", []string{"RIDER"}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	c, err := ks.Verify(tok)
	if err != nil {
		t.Fatal(err)
	}
	if c.Subject != "user-1" || c.Brand != "demo" || c.App != "rider" || len(c.Roles) != 1 {
		t.Fatalf("claims mismatch: %+v", c)
	}
	if len(ks.JWKS()["keys"].([]map[string]any)) != 1 {
		t.Fatal("jwks should expose one key")
	}
}

func TestExpiredTokenRejected(t *testing.T) {
	ks, _ := LoadOrGenerate(t.TempDir())
	tok, _ := ks.Issue("u", "b", "rider", "", nil, time.Now().Add(-2*AccessTTL))
	if _, err := ks.Verify(tok); err == nil {
		t.Fatal("expected expiry error")
	}
}
