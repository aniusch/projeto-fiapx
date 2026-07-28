// Package storage wraps an S3-compatible object store (MinIO in dev, AWS S3 in
// prod). It exposes just the operations the services need, hiding the minio-go
// client behind a small surface that is easy to fake in tests.
package storage

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/aniusch/projeto-fiapx/internal/config"
)

// Client talks to a single bucket.
type Client struct {
	mc      *minio.Client // internal client for uploads/downloads
	presign *minio.Client // client whose endpoint is the one download URLs point at
	bucket  string
}

// New builds a storage client from configuration. The same code path serves
// MinIO and S3 — only the endpoint, credentials, and TLS flag differ.
//
// When PublicEndpoint differs from Endpoint, a second client is built for
// signing download URLs: presigned URLs embed and are signed over the endpoint
// host, so they must be signed with the host the *client* will actually call.
func New(cfg config.StorageConfig) (*Client, error) {
	mc, err := newMinioClient(cfg.Endpoint, cfg)
	if err != nil {
		return nil, err
	}

	presign := mc
	if cfg.PublicEndpoint != "" && cfg.PublicEndpoint != cfg.Endpoint {
		presign, err = newMinioClient(cfg.PublicEndpoint, cfg)
		if err != nil {
			return nil, err
		}
	}

	return &Client{mc: mc, presign: presign, bucket: cfg.Bucket}, nil
}

func newMinioClient(endpoint string, cfg config.StorageConfig) (*minio.Client, error) {
	c, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("create minio client for %q: %w", endpoint, err)
	}
	return c, nil
}

// EnsureBucket creates the bucket if it does not already exist. Handy when
// pointing at a fresh object store; a no-op otherwise.
func (c *Client) EnsureBucket(ctx context.Context) error {
	exists, err := c.mc.BucketExists(ctx, c.bucket)
	if err != nil {
		return fmt.Errorf("check bucket: %w", err)
	}
	if exists {
		return nil
	}
	if err := c.mc.MakeBucket(ctx, c.bucket, minio.MakeBucketOptions{}); err != nil {
		return fmt.Errorf("make bucket: %w", err)
	}
	return nil
}

// Put streams an object of known size into the bucket under key. Passing the
// size lets the client stream directly rather than buffering the whole object in
// memory — important for large video uploads.
func (c *Client) Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error {
	_, err := c.mc.PutObject(ctx, c.bucket, key, r, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return fmt.Errorf("put object %q: %w", key, err)
	}
	return nil
}

// Get opens an object for reading. The caller must close the returned reader.
func (c *Client) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	obj, err := c.mc.GetObject(ctx, c.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("get object %q: %w", key, err)
	}
	return obj, nil
}

// PresignGet returns a time-limited URL that lets the holder download the object
// directly from object storage, without proxying the bytes through our service.
func (c *Client) PresignGet(ctx context.Context, key string, expiry time.Duration) (string, error) {
	u, err := c.presign.PresignedGetObject(ctx, c.bucket, key, expiry, nil)
	if err != nil {
		return "", fmt.Errorf("presign object %q: %w", key, err)
	}
	return u.String(), nil
}
