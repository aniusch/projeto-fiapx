// Command worker consumes video-processing jobs from RabbitMQ, extracts frames
// with ffmpeg, zips them, uploads the result to object storage, and updates the
// job status in Postgres. On failure it publishes an event the notifier picks up.
//
// In Phase 1 it only boots and waits; the consumer arrives in Phase 4.
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

	logger.Info("worker starting", "env", cfg.Env, "rabbitmq", cfg.RabbitMQ.URL)
	// Phase 4 wires the RabbitMQ consumer loop here.
	logger.Info("worker ready — waiting for jobs (consumer not yet implemented)")

	platform.WaitForSignal()
	logger.Info("worker stopped")
}
