package gateway

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisLimiter is a fixed-window rate limiter backed by Redis. It allows up to
// `limit` requests per `window` for a given key. Because it lives in Redis, the
// limit is enforced across every gateway replica, not per-process — important
// once the gateway is scaled horizontally.
type RedisLimiter struct {
	client *redis.Client
	limit  int64
	window time.Duration
}

// NewRedisLimiter builds a limiter allowing `limit` requests per `window`.
func NewRedisLimiter(client *redis.Client, limit int64, window time.Duration) *RedisLimiter {
	return &RedisLimiter{client: client, limit: limit, window: window}
}

// Allow increments the counter for key and reports whether it is within the
// limit. Only the first request in a window sets the expiry (when INCR returns
// 1), so each window is a clean fixed interval that resets when the key expires.
func (l *RedisLimiter) Allow(ctx context.Context, key string) (bool, error) {
	count, err := l.client.Incr(ctx, key).Result()
	if err != nil {
		return false, err
	}
	if count == 1 {
		// First hit in this window — start the countdown to reset.
		if err := l.client.Expire(ctx, key, l.window).Err(); err != nil {
			return false, err
		}
	}
	return count <= l.limit, nil
}
