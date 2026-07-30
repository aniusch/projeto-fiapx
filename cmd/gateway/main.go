// Command gateway is the public-facing API service: authentication, video
// uploads, per-user status listing, and downloads. It stores sources in object
// storage, records state in Postgres, and enqueues processing jobs on RabbitMQ.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/aniusch/projeto-fiapx/internal/auth"
	"github.com/aniusch/projeto-fiapx/internal/config"
	"github.com/aniusch/projeto-fiapx/internal/gateway"
	"github.com/aniusch/projeto-fiapx/internal/messaging"
	"github.com/aniusch/projeto-fiapx/internal/platform"
	"github.com/aniusch/projeto-fiapx/internal/repository/postgres"
	"github.com/aniusch/projeto-fiapx/internal/storage"
)

const (
	maxUploadBytes = 512 << 20 // 512 MiB
	presignTTL     = 15 * time.Minute
	rateLimit      = 60 // requests per window, per client IP
	rateWindow     = time.Minute
	statusPrefetch = 20 // concurrent status-event updates
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	logger := platform.NewLogger(cfg.Env, cfg.LogLevel)
	slog.SetDefault(logger)

	// Bound all startup connection work. Connectors retry within this window, so
	// a dependency that is still coming up doesn't crash the process.
	startupCtx, cancelStartup := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancelStartup()

	// --- Dependencies -----------------------------------------------------
	pool, err := platform.NewPostgresPool(startupCtx, cfg.Postgres.DSN)
	if err != nil {
		logger.Error("connect postgres", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	redisClient, err := platform.NewRedisClient(startupCtx, cfg.Redis)
	if err != nil {
		logger.Error("connect redis", "error", err)
		os.Exit(1)
	}
	defer redisClient.Close()

	rabbitConn, err := platform.NewRabbitConn(startupCtx, cfg.RabbitMQ.URL)
	if err != nil {
		logger.Error("connect rabbitmq", "error", err)
		os.Exit(1)
	}
	defer rabbitConn.Close()

	channel, err := rabbitConn.Channel()
	if err != nil {
		logger.Error("open rabbitmq channel", "error", err)
		os.Exit(1)
	}
	defer channel.Close()
	if err := messaging.DeclareTopology(channel); err != nil {
		logger.Error("declare topology", "error", err)
		os.Exit(1)
	}

	// A separate channel to consume worker status events (a channel is not safe
	// for concurrent publish + consume).
	statusCh, err := rabbitConn.Channel()
	if err != nil {
		logger.Error("open status channel", "error", err)
		os.Exit(1)
	}
	defer statusCh.Close()

	objectStore, err := storage.New(cfg.Storage)
	if err != nil {
		logger.Error("create object store client", "error", err)
		os.Exit(1)
	}
	if err := objectStore.EnsureBucket(startupCtx); err != nil {
		logger.Error("ensure bucket", "error", err)
		os.Exit(1)
	}

	logger.Info("all dependencies connected")

	// --- Wire the HTTP server --------------------------------------------
	videoRepo := postgres.NewVideoRepository(pool)
	server := gateway.NewServer(gateway.Deps{
		Users:     postgres.NewUserRepository(pool),
		Videos:    videoRepo,
		Objects:   objectStore,
		Publisher: messaging.NewPublisher(channel),
		Tokens:    auth.NewManager(cfg.JWT.Secret, cfg.JWT.TTL),
		Limiter:   gateway.NewRedisLimiter(redisClient, rateLimit, rateWindow),
		Config: gateway.Config{
			MaxUploadBytes: maxUploadBytes,
			PresignTTL:     presignTTL,
		},
	})

	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()
	router.Use(gin.Recovery())

	// Instrument every request, and expose the metrics for Prometheus to scrape.
	httpMetrics := gateway.NewHTTPMetrics(prometheus.DefaultRegisterer)
	router.Use(httpMetrics.Middleware())
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	router.GET("/readyz", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := pool.Ping(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable", "postgres": err.Error()})
			return
		}
		if err := redisClient.Ping(ctx).Err(); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable", "redis": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})

	server.RegisterRoutes(router)

	srv := &http.Server{Addr: ":" + cfg.HTTP.Port, Handler: router}

	go func() {
		logger.Info("gateway listening", "port", cfg.HTTP.Port, "env", cfg.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server failed", "error", err)
			os.Exit(1)
		}
	}()

	ctx, stop := platform.ShutdownContext()
	defer stop()

	// Consume worker status events and apply them to the videos table (the
	// gateway is the sole writer). Runs until the shutdown signal drains it.
	statusConsumer := gateway.NewStatusConsumer(videoRepo)
	go func() {
		if err := statusConsumer.Consume(ctx, statusCh, statusPrefetch); err != nil {
			logger.Error("status consumer stopped with error", "error", err)
		}
	}()

	<-ctx.Done()

	logger.Info("gateway shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
	}
	logger.Info("gateway stopped")
}
