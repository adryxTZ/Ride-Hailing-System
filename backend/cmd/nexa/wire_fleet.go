package main

import (
	"context"
	"crypto/rand"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/kalen-limited/hauler-rider/backend/internal/catalog"
	"github.com/kalen-limited/hauler-rider/backend/internal/fleet"
	"github.com/kalen-limited/hauler-rider/backend/internal/presence"
	"github.com/kalen-limited/hauler-rider/backend/internal/providers/storage"
)

// fileSecretBytes is the length of the URL-signing secret for the local store.
const fileSecretBytes = 32

// fileStore is the local disk store (NEXA_UPLOAD_DIR, default ./data/uploads).
// Signed URLs are relative to the API ("/v1/files/…") unless NEXA_PUBLIC_API_URL
// is set, so phones on the LAN and the dashboard resolve them against their own API base.
func fileStore(bs *bootstrap) *storage.Local {
	dir := os.Getenv("NEXA_UPLOAD_DIR")
	if dir == "" {
		dir = "./data/uploads"
	}
	base := os.Getenv("NEXA_PUBLIC_API_URL")
	if base == "" {
		base = "/v1"
	}
	return storage.NewLocal(dir, base, fileSecret(bs.cfg.JWTKeysDir))
}

// fileSecret reads the signing secret next to the JWT keys, creating it once.
func fileSecret(dir string) []byte {
	path := filepath.Join(dir, "files.secret")
	if b, err := os.ReadFile(path); err == nil && len(b) == fileSecretBytes {
		return b
	}
	b := make([]byte, fileSecretBytes)
	_, _ = rand.Read(b)
	if err := os.MkdirAll(dir, 0o700); err == nil {
		if err := os.WriteFile(path, b, 0o600); err != nil {
			slog.Warn("file secret not persisted; upload links break on restart", "err", err)
		}
	}
	return b
}

// startFleet wires driver onboarding and review.
func startFleet(bs *bootstrap, brandID string, cat *catalog.Catalog, files *storage.Local, p *presence.Service) *fleet.Service {
	var off fleet.Offline
	if p != nil {
		off = func(ctx context.Context, driverID string) error { return p.Offline(ctx, driverID) }
	}
	f := fleet.New(bs.pool, brandID, cat, files, off)
	f.SetCountries(bs.brand.CountryByCode)
	return f
}
