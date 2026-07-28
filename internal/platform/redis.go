package platform

import (
	"context"

	"github.com/redis/go-redis/v9"

	"github.com/aniusch/projeto-fiapx/internal/config"
)

// NewRedisClient connects to Redis and verifies it with a ping. Like the
// Postgres pool, the returned client is safe for concurrent use and shared
// across the whole process. It retries until Redis is reachable or ctx is done.
func NewRedisClient(ctx context.Context, cfg config.RedisConfig) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	err := retryConnect(ctx, "redis", func(ctx context.Context) error {
		return client.Ping(ctx).Err()
	})
	if err != nil {
		_ = client.Close()
		return nil, err
	}
	return client, nil
}
