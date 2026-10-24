// Package storage is the object-store port: documents, photos and receipts
// are uploaded straight from the apps with short-lived signed URLs (no file
// bodies through JSON endpoints), and read back the same way.
//
//   - storage.go  the Store port and key rules
//   - local.go    disk implementation for development; the API serves its
//     signed URLs at /v1/files/{key}
//
// Production swaps in an S3-compatible store (DO Spaces) with native presigned URLs.
package storage

import (
	"context"
	"errors"
	"io"
	"regexp"
	"time"
)

// Store keeps files by key.
type Store interface {
	// UploadURL returns a URL the client PUTs the bytes to before `expires`.
	UploadURL(ctx context.Context, key, contentType string, expires time.Duration) (string, error)
	// DownloadURL returns a URL that serves the file until `expires`.
	DownloadURL(ctx context.Context, key string, expires time.Duration) (string, error)
	// Size returns the size in bytes of an upload that landed, or ErrNotFound.
	Size(ctx context.Context, key string) (int64, error)
}

// Writer is implemented by stores the server itself writes to and deletes
// from: privacy exports, and files removed by the retention job.
type Writer interface {
	Put(ctx context.Context, key string, r io.Reader, maxBytes int64) error
	Delete(ctx context.Context, key string) error
}

// Server is implemented by stores whose signed URLs the API itself serves (local disk).
type Server interface {
	Verify(key, method string, expiresUnix int64, sig string) bool
	Write(key string, r io.Reader, maxBytes int64) error
	Open(key string) (io.ReadCloser, string, error)
}

// Allowed upload types and size.
var AllowedTypes = map[string]string{"image/jpeg": "jpg", "image/png": "png", "image/webp": "webp", "application/pdf": "pdf"}

// MaxUploadBytes bounds one file.
const MaxUploadBytes = 10 << 20

var keyPattern = regexp.MustCompile(`^[a-z]+/[0-9a-f-]{36}/[0-9a-f-]{36}\.(jpg|png|webp|pdf|json)$`)

// ErrBadKey rejects keys that do not follow <purpose>/<owner uuid>/<file uuid>.<ext>.
// (.json is only ever written by the server: uploads are limited to AllowedTypes.)
var ErrBadKey = errors.New("storage: bad key")

// ErrNotFound is returned for a key with no file behind it.
var ErrNotFound = errors.New("storage: not found")

// ValidKey tells whether a key has the platform's shape (no traversal possible).
func ValidKey(key string) bool { return keyPattern.MatchString(key) }
