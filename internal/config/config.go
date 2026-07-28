// Package config assembles the application's configuration from environment
// variables. Every service loads the same Config struct but only reads the
// sections it needs — the gateway ignores SMTP, the notifier ignores Storage,
// and so on. Centralizing this keeps env-var names defined in exactly one place.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config is the full, service-agnostic configuration tree.
type Config struct {
	Env      string // "development" | "production"
	LogLevel string // "debug" | "info" | "warn" | "error"

	HTTP     HTTPConfig
	Postgres PostgresConfig
	Redis    RedisConfig
	RabbitMQ RabbitMQConfig
	Storage  StorageConfig
	JWT      JWTConfig
	SMTP     SMTPConfig
	Worker   WorkerConfig
	Metrics  MetricsConfig
}

// MetricsConfig configures the Prometheus metrics endpoint for services that
// don't otherwise run an HTTP server (worker, notifier).
type MetricsConfig struct {
	Addr string
}

// WorkerConfig configures the video-processing worker.
type WorkerConfig struct {
	FFmpegPath  string        // path/name of the ffmpeg binary
	FPS         int           // frames to extract per second of video
	WorkDir     string        // scratch directory for downloads/frames/zips
	JobTimeout  time.Duration // max time to spend processing a single video
	Concurrency int           // max videos processed in parallel per worker
}

// HTTPConfig configures the gateway's HTTP server.
type HTTPConfig struct {
	Port            string
	ShutdownTimeout time.Duration
}

// PostgresConfig holds the connection string for the primary datastore.
type PostgresConfig struct {
	DSN string
}

// RedisConfig configures the cache / token store.
type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

// RabbitMQConfig configures the message broker connection.
type RabbitMQConfig struct {
	URL string
}

// StorageConfig configures the S3-compatible object store (MinIO in dev, S3 in prod).
type StorageConfig struct {
	Endpoint string // internal endpoint used for uploads/downloads by the services
	// PublicEndpoint is the host clients can reach, used when signing download
	// URLs. It differs from Endpoint when the service talks to storage over an
	// internal network name (e.g. "minio:9000") but browsers use "localhost:9000".
	// Empty means "same as Endpoint".
	PublicEndpoint string
	Region         string
	Bucket         string
	AccessKey      string
	SecretKey      string
	UseSSL         bool
}

// JWTConfig configures token signing and lifetime.
type JWTConfig struct {
	Secret string
	TTL    time.Duration
}

// SMTPConfig configures the outbound mail server used by the notifier. In dev
// this points at Mailpit (no auth, no TLS); in prod, at a real relay with
// credentials and STARTTLS.
type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	StartTLS bool
}

// Load reads configuration from the environment, applying development-friendly
// defaults so the services can boot locally without a fully populated .env.
// It returns an error only if a value that should be numeric/boolean/duration
// is present but malformed.
func Load() (Config, error) {
	l := &loader{}

	cfg := Config{
		Env:      l.str("APP_ENV", "development"),
		LogLevel: l.str("LOG_LEVEL", "info"),

		HTTP: HTTPConfig{
			Port:            l.str("HTTP_PORT", "8080"),
			ShutdownTimeout: l.duration("HTTP_SHUTDOWN_TIMEOUT", 10*time.Second),
		},
		Postgres: PostgresConfig{
			DSN: l.str("POSTGRES_DSN", "postgres://fiapx:fiapx@localhost:5432/fiapx?sslmode=disable"),
		},
		Redis: RedisConfig{
			Addr:     l.str("REDIS_ADDR", "localhost:6379"),
			Password: l.str("REDIS_PASSWORD", ""),
			DB:       l.int("REDIS_DB", 0),
		},
		RabbitMQ: RabbitMQConfig{
			URL: l.str("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
		},
		Storage: StorageConfig{
			Endpoint:       l.str("S3_ENDPOINT", "localhost:9000"),
			PublicEndpoint: l.str("S3_PUBLIC_ENDPOINT", ""),
			Region:         l.str("S3_REGION", "us-east-1"),
			Bucket:         l.str("S3_BUCKET", "fiapx-videos"),
			AccessKey:      l.str("S3_ACCESS_KEY", "minioadmin"),
			SecretKey:      l.str("S3_SECRET_KEY", "minioadmin"),
			UseSSL:         l.boolean("S3_USE_SSL", false),
		},
		JWT: JWTConfig{
			Secret: l.str("JWT_SECRET", "dev-insecure-secret-change-me"),
			TTL:    l.duration("JWT_TTL", 24*time.Hour),
		},
		SMTP: SMTPConfig{
			Host:     l.str("SMTP_HOST", "localhost"),
			Port:     l.int("SMTP_PORT", 1025),
			Username: l.str("SMTP_USERNAME", ""),
			Password: l.str("SMTP_PASSWORD", ""),
			From:     l.str("SMTP_FROM", "no-reply@fiapx.local"),
			StartTLS: l.boolean("SMTP_STARTTLS", false),
		},
		Worker: WorkerConfig{
			FFmpegPath:  l.str("FFMPEG_PATH", "ffmpeg"),
			FPS:         l.int("PROCESSING_FPS", 1),
			WorkDir:     l.str("WORK_DIR", os.TempDir()),
			JobTimeout:  l.duration("JOB_TIMEOUT", 10*time.Minute),
			Concurrency: l.int("WORKER_CONCURRENCY", 3),
		},
		Metrics: MetricsConfig{
			Addr: l.str("METRICS_ADDR", ":9101"),
		},
	}

	return cfg, l.err
}

// loader accumulates the first parse error encountered while reading env vars.
// This lets Load build the whole Config in one expression and report a single
// error at the end, instead of checking err after every field.
type loader struct {
	err error
}

func (l *loader) str(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func (l *loader) int(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		l.record(key, err)
		return def
	}
	return n
}

func (l *loader) boolean(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		l.record(key, err)
		return def
	}
	return b
}

func (l *loader) duration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		l.record(key, err)
		return def
	}
	return d
}

// record keeps only the first error so the caller sees the earliest problem.
func (l *loader) record(key string, err error) {
	if l.err == nil {
		l.err = fmt.Errorf("config: invalid value for %s: %w", key, err)
	}
}
