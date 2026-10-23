package branding

import (
	"context"
	"time"
)

// Files is the object store for logos and icons (providers/storage).
type Files interface {
	UploadURL(ctx context.Context, key, contentType string, expires time.Duration) (string, error)
	DownloadURL(ctx context.Context, key string, expires time.Duration) (string, error)
	Size(ctx context.Context, key string) (int64, error)
}
