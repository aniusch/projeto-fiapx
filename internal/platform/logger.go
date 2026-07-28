// Package platform holds cross-cutting helpers shared by every service: logging,
// signal handling, and the datastore/broker connectors.
package platform

import (
	"log/slog"
	"os"
	"strings"
)

// NewLogger builds a structured logger: JSON in production, human-readable text
// otherwise, at the given level.
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
