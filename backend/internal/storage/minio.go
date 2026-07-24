// Package storage wraps the MinIO/S3 client used to store question
// media (images, audio fragments). It knows nothing about quizzes or
// questions — just objects, bucket setup, and public URLs.
package storage

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// tracer is safe to use before telemetry.Setup runs: otel.Tracer returns a
// lazily-delegating wrapper that resolves the real TracerProvider at each
// Start call, not at the time this var is initialized.
var tracer = otel.Tracer("quizmos/storage")

// endSpan records err (if any) as a span error before ending it. Unlike
// the service package's sentinel errors (ErrNotFound and friends, which
// are expected business outcomes), every error this package's methods can
// return reflects an actual MinIO/S3 failure, so it's always worth
// flagging the span.
func endSpan(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
}

type Config struct {
	// Endpoint is host:port with no scheme — where the backend itself
	// reaches MinIO (e.g. "minio:9000" inside docker compose).
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	UseSSL    bool
	// PublicURL is the scheme+host (e.g. "http://localhost:9000") a
	// browser uses to fetch objects directly — deliberately separate
	// from Endpoint, since it's very often a different address than the
	// one the backend dials internally.
	PublicURL string
}

type Client struct {
	mc        *minio.Client
	bucket    string
	publicURL string
}

func New(cfg Config) (*Client, error) {
	mc, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("create minio client: %w", err)
	}
	return &Client{
		mc:        mc,
		bucket:    cfg.Bucket,
		publicURL: strings.TrimSuffix(cfg.PublicURL, "/"),
	}, nil
}

// EnsureBucket creates the bucket if it doesn't already exist and sets
// an anonymous-read policy on it. Question media has to be fetchable
// directly by players' browsers with no auth and no backend round trip,
// so the bucket (not just individual objects) is public-read.
func (c *Client) EnsureBucket(ctx context.Context) (err error) {
	ctx, span := tracer.Start(ctx, "storage.EnsureBucket", trace.WithAttributes(attribute.String("s3.bucket", c.bucket)))
	defer func() { endSpan(span, err) }()

	exists, err := c.mc.BucketExists(ctx, c.bucket)
	if err != nil {
		return fmt.Errorf("check bucket %q exists: %w", c.bucket, err)
	}
	if !exists {
		if err := c.mc.MakeBucket(ctx, c.bucket, minio.MakeBucketOptions{}); err != nil {
			return fmt.Errorf("create bucket %q: %w", c.bucket, err)
		}
	}
	policy := fmt.Sprintf(`{
		"Version": "2012-10-17",
		"Statement": [{
			"Effect": "Allow",
			"Principal": {"AWS": ["*"]},
			"Action": ["s3:GetObject"],
			"Resource": ["arn:aws:s3:::%s/*"]
		}]
	}`, c.bucket)
	if err := c.mc.SetBucketPolicy(ctx, c.bucket, policy); err != nil {
		return fmt.Errorf("set public-read policy on bucket %q: %w", c.bucket, err)
	}
	return nil
}

// Put uploads r as key, streaming rather than buffering — media files
// can be tens of megabytes and there's no need to hold them in memory.
func (c *Client) Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) (err error) {
	ctx, span := tracer.Start(ctx, "storage.Put", trace.WithAttributes(
		attribute.String("s3.bucket", c.bucket),
		attribute.String("s3.key", key),
		attribute.Int64("s3.size_bytes", size),
	))
	defer func() { endSpan(span, err) }()

	_, err = c.mc.PutObject(ctx, c.bucket, key, r, size, minio.PutObjectOptions{ContentType: contentType})
	return err
}

func (c *Client) Delete(ctx context.Context, key string) (err error) {
	ctx, span := tracer.Start(ctx, "storage.Delete", trace.WithAttributes(
		attribute.String("s3.bucket", c.bucket),
		attribute.String("s3.key", key),
	))
	defer func() { endSpan(span, err) }()

	return c.mc.RemoveObject(ctx, c.bucket, key, minio.RemoveObjectOptions{})
}

// PublicURL returns the URL a browser can fetch key from directly.
func (c *Client) PublicURL(key string) string {
	return fmt.Sprintf("%s/%s/%s", c.publicURL, c.bucket, key)
}
