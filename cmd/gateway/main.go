// Command gateway is the public-facing API service. It handles authentication,
// accepts video uploads, enqueues processing jobs, and reports per-user status.
//
// As of Phase 2 it connects to Postgres and exposes liveness (/healthz) and
// readiness (/readyz) checks; the real routes arrive in Phase 3.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/gabrielschuina/projeto-fiapx/internal/config"
	"github.com/gabrielschuina/projeto-fiapx/internal/platform"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	logger := platform.NewLogger(cfg.Env, cfg.LogLevel)
	slog.SetDefault(logger)

	// Bound the startup work (DB connect) so we fail fast instead of hanging if
	// Postgres is unreachable.
	startupCtx, cancelStartup := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelStartup()

	pool, err := platform.NewPostgresPool(startupCtx, cfg.Postgres.DSN)
	if err != nil {
		logger.Error("failed to connect to postgres", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	logger.Info("connected to postgres")

	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())

	// Liveness: the process is up. Kubernetes restarts the pod if this fails.
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Readiness: dependencies are reachable. Kubernetes stops routing traffic
	// here (without restarting) when this fails, e.g. during a DB blip.
	router.GET("/readyz", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := pool.Ping(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable", "postgres": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})

	srv := &http.Server{
		Addr:    ":" + cfg.HTTP.Port,
		Handler: router,
	}

	go func() {
		logger.Info("gateway listening", "port", cfg.HTTP.Port, "env", cfg.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server failed", "error", err)
			os.Exit(1)
		}
	}()

	ctx, stop := platform.ShutdownContext()
	defer stop()
	<-ctx.Done()

	logger.Info("gateway shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
	}
	logger.Info("gateway stopped")
}
