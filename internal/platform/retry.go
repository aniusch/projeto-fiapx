package platform

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// retryConnect calls fn until it succeeds or ctx is cancelled, backing off
// exponentially between attempts (capped). This lets a service tolerate a
// dependency that isn't ready yet at startup — instead of exiting and relying on
// the orchestrator to restart it (a crash-loop), it waits, bounded by the
// startup context's deadline.
func retryConnect(ctx context.Context, dependency string, fn func(context.Context) error) error {
	const maxDelay = 5 * time.Second
	delay := 500 * time.Millisecond

	for attempt := 1; ; attempt++ {
		err := fn(ctx)
		if err == nil {
			return nil
		}
		// If the context is already done, don't keep trying.
		if ctx.Err() != nil {
			return fmt.Errorf("connect %s: gave up after %d attempt(s): %w", dependency, attempt, err)
		}

		slog.Warn("dependency not ready, retrying",
			"dependency", dependency, "attempt", attempt, "retry_in", delay, "error", err)

		select {
		case <-ctx.Done():
			return fmt.Errorf("connect %s: %w (last error: %v)", dependency, ctx.Err(), err)
		case <-time.After(delay):
		}

		if delay < maxDelay {
			if delay *= 2; delay > maxDelay {
				delay = maxDelay
			}
		}
	}
}
