package config

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Env != "development" {
		t.Errorf("Env = %q, want development", cfg.Env)
	}
	if cfg.HTTP.Port != "8080" {
		t.Errorf("HTTP.Port = %q, want 8080", cfg.HTTP.Port)
	}
	if cfg.JWT.TTL != 24*time.Hour {
		t.Errorf("JWT.TTL = %v, want 24h", cfg.JWT.TTL)
	}
	if cfg.Worker.FPS != 1 {
		t.Errorf("Worker.FPS = %d, want 1", cfg.Worker.FPS)
	}
	if cfg.Metrics.Addr != ":9101" {
		t.Errorf("Metrics.Addr = %q, want :9101", cfg.Metrics.Addr)
	}
}

func TestLoadOverridesFromEnv(t *testing.T) {
	t.Setenv("HTTP_PORT", "9999")
	t.Setenv("PROCESSING_FPS", "5")
	t.Setenv("JWT_TTL", "2h")
	t.Setenv("S3_USE_SSL", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.HTTP.Port != "9999" {
		t.Errorf("HTTP.Port = %q, want 9999", cfg.HTTP.Port)
	}
	if cfg.Worker.FPS != 5 {
		t.Errorf("Worker.FPS = %d, want 5", cfg.Worker.FPS)
	}
	if cfg.JWT.TTL != 2*time.Hour {
		t.Errorf("JWT.TTL = %v, want 2h", cfg.JWT.TTL)
	}
	if !cfg.Storage.UseSSL {
		t.Error("Storage.UseSSL = false, want true")
	}
}

func TestLoadRejectsMalformedValues(t *testing.T) {
	t.Setenv("PROCESSING_FPS", "not-a-number")
	if _, err := Load(); err == nil {
		t.Fatal("expected an error for a non-numeric PROCESSING_FPS")
	}
}
