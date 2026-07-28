// Command worker consumes video-processing jobs from RabbitMQ, extracts frames
// with ffmpeg, zips them, uploads the result to object storage, and updates the
// job status in Postgres. On failure it publishes an event for the notifier.
package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/aniusch/projeto-fiapx/internal/config"
	"github.com/aniusch/projeto-fiapx/internal/messaging"
	"github.com/aniusch/projeto-fiapx/internal/platform"
	"github.com/aniusch/projeto-fiapx/internal/repository/postgres"
	"github.com/aniusch/projeto-fiapx/internal/storage"
	"github.com/aniusch/projeto-fiapx/internal/worker"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	logger := platform.NewLogger(cfg.Env, cfg.LogLevel)
	slog.SetDefault(logger)

	startupCtx, cancelStartup := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelStartup()

	// --- Dependencies -----------------------------------------------------
	pool, err := platform.NewPostgresPool(startupCtx, cfg.Postgres.DSN)
	if err != nil {
		logger.Error("connect postgres", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	objectStore, err := storage.New(cfg.Storage)
	if err != nil {
		logger.Error("create object store client", "error", err)
		os.Exit(1)
	}

	rabbitConn, err := platform.NewRabbitConn(cfg.RabbitMQ.URL)
	if err != nil {
		logger.Error("connect rabbitmq", "error", err)
		os.Exit(1)
	}
	defer rabbitConn.Close()

	// One channel to consume jobs, a separate one to publish failure events.
	// A single channel is not safe to use concurrently for both.
	consumeCh, err := rabbitConn.Channel()
	if err != nil {
		logger.Error("open consume channel", "error", err)
		os.Exit(1)
	}
	defer consumeCh.Close()

	publishCh, err := rabbitConn.Channel()
	if err != nil {
		logger.Error("open publish channel", "error", err)
		os.Exit(1)
	}
	defer publishCh.Close()

	if err := messaging.DeclareTopology(consumeCh); err != nil {
		logger.Error("declare topology", "error", err)
		os.Exit(1)
	}

	// --- Build the worker -------------------------------------------------
	w := worker.New(worker.Deps{
		Videos:  postgres.NewVideoRepository(pool),
		Users:   postgres.NewUserRepository(pool),
		Objects: objectStore,
		Events:  messaging.NewPublisher(publishCh),
		Config: worker.Config{
			FFmpegPath: cfg.Worker.FFmpegPath,
			FPS:        cfg.Worker.FPS,
			WorkDir:    cfg.Worker.WorkDir,
			JobTimeout: cfg.Worker.JobTimeout,
		},
	})

	logger.Info("worker starting",
		"concurrency", cfg.Worker.Concurrency, "fps", cfg.Worker.FPS, "ffmpeg", cfg.Worker.FFmpegPath)

	// Consume returns when the shutdown signal fires (after in-flight jobs drain)
	// or if consumption fails to start.
	ctx, stop := platform.ShutdownContext()
	defer stop()

	done := make(chan error, 1)
	go func() { done <- w.Consume(ctx, consumeCh, cfg.Worker.Concurrency) }()

	if err := <-done; err != nil {
		logger.Error("consumer stopped with error", "error", err)
		os.Exit(1)
	}
	logger.Info("worker stopped")
}
