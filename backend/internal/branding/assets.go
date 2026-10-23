package branding

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
)

// uploadTTL is how long an upload ticket stays valid.
const uploadTTL = 10 * time.Minute

// assetTypes are the image types accepted for logos and icons (no PDFs; SVG is
// not accepted because the apps and PDFs cannot render it safely without a rasteriser).
var assetTypes = map[string]string{"image/png": "png", "image/webp": "webp", "image/jpeg": "jpg"}

// PublicPrefix is where the API serves branding assets to anyone (apps, receipts, emails).
const PublicPrefix = "/v1/brand/assets/"

// UploadTicket is where the admin PUTs a logo or icon before attaching its key to a draft.
type UploadTicket struct {
	Key       string    `json:"key"`
	UploadURL string    `json:"upload_url"`
	ExpiresAt time.Time `json:"expires_at"`
}

// UploadURL issues an upload ticket for one image.
func (s *Service) UploadURL(ctx context.Context, contentType string) (*UploadTicket, error) {
	ext, ok := assetTypes[contentType]
	if !ok {
		return nil, ErrAssetType
	}
	key := assetKeyPrefix + s.store.brandID + "/" + uuid.NewString() + "." + ext
	url, err := s.files.UploadURL(ctx, key, contentType, uploadTTL)
	if err != nil {
		return nil, err
	}
	return &UploadTicket{Key: key, UploadURL: url, ExpiresAt: time.Now().Add(uploadTTL)}, nil
}

// checkAssets confirms each attached file was uploaded and is small enough.
func (s *Service) checkAssets(ctx context.Context, assets map[string]string) error {
	for slot, key := range assets {
		size, err := s.files.Size(ctx, key)
		if err != nil || size > MaxAssetBytes {
			return ErrAssetMissing.WithDetails(map[string]any{"slot": slot})
		}
	}
	return nil
}

// AssetLink returns a short-lived download link for a public branding asset.
func (s *Service) AssetLink(ctx context.Context, key string) (string, error) {
	if !strings.HasPrefix(key, assetKeyPrefix) {
		return "", ErrAssetMissing
	}
	return s.files.DownloadURL(ctx, key, time.Hour)
}

// assetURLs maps slots to the API's public asset paths.
func assetURLs(keys map[string]string) map[string]string {
	out := map[string]string{}
	for slot, key := range keys {
		out[slot] = PublicPrefix + key
	}
	return out
}
