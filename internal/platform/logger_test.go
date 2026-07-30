package platform

import (
	"log/slog"
	"testing"
)

func TestNewLogger(t *testing.T) {
	if NewLogger("production", "info") == nil {
		t.Fatal("production logger is nil")
	}
	if NewLogger("development", "debug") == nil {
		t.Fatal("development logger is nil")
	}
}

func TestParseLevel(t *testing.T) {
	cases := map[string]slog.Level{
		"debug":    slog.LevelDebug,
		"warn":     slog.LevelWarn,
		"warning":  slog.LevelWarn,
		"error":    slog.LevelError,
		"info":     slog.LevelInfo,
		"":         slog.LevelInfo, // default
		"nonsense": slog.LevelInfo,
	}
	for in, want := range cases {
		if got := parseLevel(in); got != want {
			t.Errorf("parseLevel(%q) = %v, want %v", in, got, want)
		}
	}
}
