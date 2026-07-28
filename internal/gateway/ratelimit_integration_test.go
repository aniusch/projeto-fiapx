//go:build integration

// Integration test for the Redis-backed rate limiter. Requires a running Redis
// (from docker-compose). Run with:
//
//	go test -tags=integration ./internal/gateway/...
//
// Override the address with TEST_REDIS_ADDR.
package gateway

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func testRedisAddr() string {
	if v := os.Getenv("TEST_REDIS_ADDR"); v != "" {
		return v
	}
	return "localhost:6379"
}

func TestRedisLimiterAllowsThenBlocks(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client := redis.NewClient(&redis.Options{Addr: testRedisAddr()})
	defer client.Close()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("connect redis: %v", err)
	}

	// Unique key per run so repeated test runs don't interfere.
	key := fmt.Sprintf("ratelimit:test:%d", time.Now().UnixNano())
	const limit = 3
	limiter := NewRedisLimiter(client, limit, time.Minute)

	// The first `limit` calls are allowed; the next is not.
	for i := 1; i <= limit; i++ {
		allowed, err := limiter.Allow(ctx, key)
		if err != nil {
			t.Fatalf("Allow #%d: %v", i, err)
		}
		if !allowed {
			t.Fatalf("call #%d should be allowed (limit=%d)", i, limit)
		}
	}
	allowed, err := limiter.Allow(ctx, key)
	if err != nil {
		t.Fatalf("Allow over limit: %v", err)
	}
	if allowed {
		t.Fatalf("call #%d should be blocked (limit=%d)", limit+1, limit)
	}
}
