package cache

import (
	"context"
	"testing"
	"time"
)

type fakeVersions struct{ v int64 }

func (f *fakeVersions) Version(context.Context, string) (int64, error) { return f.v, nil }

func TestReloadsOnlyWhenVersionChanges(t *testing.T) {
	versions := &fakeVersions{v: 1}
	loads := 0
	c := New("x", versions, func(context.Context) (int, error) { loads++; return loads * 10, nil }, 0)
	ctx := context.Background()
	if v, _, _ := c.Get(ctx); v != 10 || loads != 1 {
		t.Fatalf("first get: v=%d loads=%d", v, loads)
	}
	if v, _, _ := c.Get(ctx); v != 10 || loads != 1 {
		t.Fatalf("same version must not reload: v=%d loads=%d", v, loads)
	}
	versions.v = 2
	if v, ver, _ := c.Get(ctx); v != 20 || ver != 2 || loads != 2 {
		t.Fatalf("version bump must reload: v=%d ver=%d loads=%d", v, ver, loads)
	}
}

func TestTrustsRecentCheck(t *testing.T) {
	versions := &fakeVersions{v: 1}
	loads := 0
	c := New("x", versions, func(context.Context) (int, error) { loads++; return 1, nil }, time.Minute)
	_, _, _ = c.Get(context.Background())
	versions.v = 2
	_, ver, _ := c.Get(context.Background())
	if ver != 1 || loads != 1 {
		t.Fatalf("within checkTTL the old value is served: ver=%d loads=%d", ver, loads)
	}
}
