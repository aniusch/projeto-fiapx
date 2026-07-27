// Package platform holds small cross-cutting helpers shared by every service:
// logging setup, signal handling, and (in later phases) metrics.
package platform

import (
	"log/slog"
	"os"
	"strings"
)

// NewLogger builds a structured logger. In production it emits JSON (easy for
// log aggregators like the ELK stack to parse); in development it emits
// human-readable text. The level string controls verbosity.
func NewLogger(env, level string) *slog.Logger {
	opts := &slog.HandlerOptions{Level: parseLevel(level)}

	var handler slog.Handler
	if strings.EqualFold(env, "production") {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
