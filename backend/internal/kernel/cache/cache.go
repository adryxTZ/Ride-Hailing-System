// Package cache is the read-mostly data cache the performance standard asks
// for: values live in process and are invalidated by a version number that
// writers bump in Postgres (config_versions) and publish through Redis, so a
// read costs one cheap version check at most every few seconds and never a
// full query unless something changed.
package cache

import (
	"context"
	"sync"
	"time"
)

// VersionSource returns the current version for a scope (e.g. "catalog").
type VersionSource interface {
	Version(ctx context.Context, scope string) (int64, error)
}

// Loader produces the value for the cache from the source of truth.
type Loader[T any] func(ctx context.Context) (T, error)

// Versioned caches one value and refreshes it only when the scope's version changes.
type Versioned[T any] struct {
	scope    string
	versions VersionSource
	load     Loader[T]
	checkTTL time.Duration // how long to trust the last version check

	mu        sync.RWMutex
	value     T
	version   int64
	checkedAt time.Time
	loaded    bool
}

// New builds a versioned cache. checkTTL bounds how stale a value can be after
// a write elsewhere (5 s is a good default for catalogue data).
func New[T any](scope string, versions VersionSource, load Loader[T], checkTTL time.Duration) *Versioned[T] {
	return &Versioned[T]{scope: scope, versions: versions, load: load, checkTTL: checkTTL}
}

// Get returns the cached value, reloading when the version moved. On a version
// lookup failure the last good value is served (availability over freshness).
func (c *Versioned[T]) Get(ctx context.Context) (T, int64, error) {
	c.mu.RLock()
	fresh := c.loaded && time.Since(c.checkedAt) < c.checkTTL
	v, ver := c.value, c.version
	c.mu.RUnlock()
	if fresh {
		return v, ver, nil
	}

	current, err := c.versions.Version(ctx, c.scope)
	if err != nil && c.loaded {
		return v, ver, nil
	}
	if err != nil {
		var zero T
		return zero, 0, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.loaded && current == c.version {
		c.checkedAt = time.Now()
		return c.value, c.version, nil
	}
	value, err := c.load(ctx)
	if err != nil {
		if c.loaded {
			return c.value, c.version, nil
		}
		var zero T
		return zero, 0, err
	}
	c.value, c.version, c.loaded, c.checkedAt = value, current, true, time.Now()
	return c.value, c.version, nil
}
