package storage

import (
	"testing"

	"github.com/aniusch/projeto-fiapx/internal/config"
)

func TestNewWithStaticCredentials(t *testing.T) {
	c, err := New(config.StorageConfig{
		Endpoint:  "localhost:9000",
		Region:    "us-east-1",
		Bucket:    "fiapx-videos",
		AccessKey: "minioadmin",
		SecretKey: "minioadmin",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if c == nil {
		t.Fatal("expected a client")
	}
}

func TestNewWithIAMCredentials(t *testing.T) {
	// Empty access key selects the AWS default/IAM chain. Construction must not
	// require network — the IAM provider only contacts IMDS lazily, on first use.
	c, err := New(config.StorageConfig{
		Endpoint: "s3.us-east-1.amazonaws.com",
		Region:   "us-east-1",
		Bucket:   "fiapx-videos",
		UseSSL:   true,
	})
	if err != nil {
		t.Fatalf("New with IAM creds: %v", err)
	}
	if c == nil {
		t.Fatal("expected a client")
	}
}

func TestNewUsesSeparatePresignClientForPublicEndpoint(t *testing.T) {
	c, err := New(config.StorageConfig{
		Endpoint:       "minio:9000",
		PublicEndpoint: "localhost:9000",
		Region:         "us-east-1",
		Bucket:         "fiapx-videos",
		AccessKey:      "minioadmin",
		SecretKey:      "minioadmin",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if c.mc == c.presign {
		t.Fatal("expected a distinct presign client when PublicEndpoint differs")
	}
}
