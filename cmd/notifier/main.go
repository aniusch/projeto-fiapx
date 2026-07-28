// Command notifier consumes failure events from RabbitMQ and emails the affected
// user so they know a video could not be processed.
//
// In Phase 1 it only boots and waits; the consumer arrives in Phase 5.
package main

import (
	"log/slog"
	"os"

	"github.com/aniusch/projeto-fiapx/internal/config"
	"github.com/aniusch/projeto-fiapx/internal/platform"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	logger := platform.NewLogger(cfg.Env, cfg.LogLevel)
	slog.SetDefault(logger)

	logger.Info("notifier starting", "env", cfg.Env, "smtp", cfg.SMTP.Host)
	// Phase 5 wires the RabbitMQ consumer + SMTP sender here.
	logger.Info("notifier ready — waiting for events (consumer not yet implemented)")

	platform.WaitForSignal()
	logger.Info("notifier stopped")
}
