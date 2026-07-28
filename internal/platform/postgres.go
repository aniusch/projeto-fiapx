package platform

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	pgxuuid "github.com/vgarvardt/pgx-google-uuid/v5"
)

// NewPostgresPool opens a connection pool to Postgres, verifying it with a ping.
// It retries until Postgres is reachable or ctx is cancelled, so a not-yet-ready
// database at startup doesn't crash the process.
func NewPostgresPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	var pool *pgxpool.Pool
	err := retryConnect(ctx, "postgres", func(ctx context.Context) error {
		p, err := openPostgresPool(ctx, dsn)
		if err != nil {
			return err
		}
		pool = p
		return nil
	})
	return pool, err
}

func openPostgresPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse postgres dsn: %w", err)
	}

	// Register the google/uuid codec on each new connection so repositories can
	// pass and scan uuid.UUID directly (pgx doesn't support it out of the box).
	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		pgxuuid.Register(conn.TypeMap())
		return nil
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return pool, nil
}
