package platform

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRetryConnectSucceedsAfterFailures(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	attempts := 0
	err := retryConnect(ctx, "test", func(context.Context) error {
		attempts++
		if attempts < 3 {
			return errors.New("not ready")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

func TestRetryConnectGivesUpWhenContextExpires(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	err := retryConnect(ctx, "test", func(context.Context) error {
		return errors.New("always failing")
	})
	if err == nil {
		t.Fatal("expected an error once the context expired")
	}
}
