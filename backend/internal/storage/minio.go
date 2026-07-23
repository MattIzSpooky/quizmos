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
)

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
func (c *Client) EnsureBucket(ctx context.Context) error {
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
func (c *Client) Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error {
	_, err := c.mc.PutObject(ctx, c.bucket, key, r, size, minio.PutObjectOptions{ContentType: contentType})
	return err
}

func (c *Client) Delete(ctx context.Context, key string) error {
	return c.mc.RemoveObject(ctx, c.bucket, key, minio.RemoveObjectOptions{})
}

// PublicURL returns the URL a browser can fetch key from directly.
func (c *Client) PublicURL(key string) string {
	return fmt.Sprintf("%s/%s/%s", c.publicURL, c.bucket, key)
}
