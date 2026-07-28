package platform

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"

	"github.com/aniusch/projeto-fiapx/internal/config"
)

// NewRedisClient connects to Redis and verifies it with a ping. Like the
// Postgres pool, the returned client is safe for concurrent use and shared
// across the whole process.
func NewRedisClient(ctx context.Context, cfg config.RedisConfig) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}
	return client, nil
}
