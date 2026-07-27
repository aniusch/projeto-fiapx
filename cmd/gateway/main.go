// Command gateway is the public-facing API service. It handles authentication,
// accepts video uploads, enqueues processing jobs, and reports per-user status.
//
// In Phase 1 it only exposes a health check; the real routes arrive in Phase 3.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"

	"github.com/gabrielschuina/projeto-fiapx/internal/config"
	"github.com/gabrielschuina/projeto-fiapx/internal/platform"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		// The default logger is fine here: our custom one needs the config we
		// just failed to load.
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	logger := platform.NewLogger(cfg.Env, cfg.LogLevel)
	slog.SetDefault(logger) // so libraries using slog's default logger match our format

	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery()) // turns panics into 500s instead of crashing the process
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	srv := &http.Server{
		Addr:    ":" + cfg.HTTP.Port,
		Handler: router,
	}

	// Start the server in a goroutine so main can block on shutdown signals.
	go func() {
		logger.Info("gateway listening", "port", cfg.HTTP.Port, "env", cfg.Env)
		// ListenAndServe blocks until the server stops. A clean shutdown returns
		// http.ErrServerClosed, which is expected — anything else is a real error.
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Block until we get SIGINT/SIGTERM, then drain in-flight requests.
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
