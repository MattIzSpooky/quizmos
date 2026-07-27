package core

import (
	"context"
	"io"
)

// MediaStorage is the object storage dependency question-media upload
// needs — implemented by internal/storage.Client (MinIO/S3). Kept as a
// narrow interface here so domain packages don't need to import storage's
// concrete client type.
type MediaStorage interface {
	Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error
	Delete(ctx context.Context, key string) error
	PublicURL(key string) string
}
