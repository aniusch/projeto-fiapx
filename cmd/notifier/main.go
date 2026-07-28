// Command notifier consumes failure events from RabbitMQ and emails the affected
// user so they know a video could not be processed.
package main

import (
	"log/slog"
	"os"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/aniusch/projeto-fiapx/internal/config"
	"github.com/aniusch/projeto-fiapx/internal/mail"
	"github.com/aniusch/projeto-fiapx/internal/messaging"
	"github.com/aniusch/projeto-fiapx/internal/notifier"
	"github.com/aniusch/projeto-fiapx/internal/observability"
	"github.com/aniusch/projeto-fiapx/internal/platform"
)

// prefetch bounds how many notifications are sent concurrently.
const prefetch = 10

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	logger := platform.NewLogger(cfg.Env, cfg.LogLevel)
	slog.SetDefault(logger)

	rabbitConn, err := platform.NewRabbitConn(cfg.RabbitMQ.URL)
	if err != nil {
		logger.Error("connect rabbitmq", "error", err)
		os.Exit(1)
	}
	defer rabbitConn.Close()

	channel, err := rabbitConn.Channel()
	if err != nil {
		logger.Error("open channel", "error", err)
		os.Exit(1)
	}
	defer channel.Close()

	if err := messaging.DeclareTopology(channel); err != nil {
		logger.Error("declare topology", "error", err)
		os.Exit(1)
	}

	metrics := observability.NewNotifierMetrics(prometheus.DefaultRegisterer)
	metricsSrv := observability.StartMetricsServer(cfg.Metrics.Addr, logger)
	defer observability.Shutdown(metricsSrv)

	n := notifier.New(mail.NewSMTPMailer(cfg.SMTP), metrics)

	logger.Info("notifier starting", "smtp", cfg.SMTP.Host, "from", cfg.SMTP.From)

	ctx, stop := platform.ShutdownContext()
	defer stop()

	done := make(chan error, 1)
	go func() { done <- n.Consume(ctx, channel, prefetch) }()

	if err := <-done; err != nil {
		logger.Error("consumer stopped with error", "error", err)
		os.Exit(1)
	}
	logger.Info("notifier stopped")
}
