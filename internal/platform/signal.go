package platform

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

// WaitForSignal blocks until the process receives an interrupt (Ctrl-C) or a
// SIGTERM (sent by Docker/Kubernetes when stopping a container). Long-running
// workers with no HTTP server use this to stay alive until asked to stop.
func WaitForSignal() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()
}

// ShutdownContext returns a context that is cancelled on interrupt/SIGTERM,
// plus its stop function. Servers use this to trigger graceful shutdown.
func ShutdownContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}
