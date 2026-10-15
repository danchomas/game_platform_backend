package pgx

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool создает пул соединений с базой данных (с ретраями для Docker)
func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	cfg.MaxConns = 30
	cfg.MinConns = 10
	cfg.MaxConnLifetime = 30 * time.Minute

	var pool *pgxpool.Pool
	var lastErr error

	for i := 0; i < 30; i++ {
		pool, lastErr = pgxpool.NewWithConfig(ctx, cfg)
		if lastErr == nil {
			if lastErr = pool.Ping(ctx); lastErr == nil {
				return pool, nil
			}
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return nil, fmt.Errorf("postgres is unavailable: %w", lastErr)
}
