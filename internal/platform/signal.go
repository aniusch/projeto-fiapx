package platform

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

// WaitForSignal blocks until the process receives SIGINT or SIGTERM.
func WaitForSignal() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()
}

// ShutdownContext returns a context cancelled on SIGINT/SIGTERM, plus its stop
// function, for triggering graceful shutdown.
func ShutdownContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}
