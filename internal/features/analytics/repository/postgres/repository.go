package postgres

import (
	"beeline-games/internal/core/domain"
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	Summary(ctx context.Context, f domain.Filter) (domain.Summary, error)
	Daily(ctx context.Context, f domain.Filter) ([]domain.DayPoint, error)
	PerGame(ctx context.Context, f domain.Filter) ([]domain.GameRow, error)
	CustomEvents(ctx context.Context, f domain.Filter) ([]domain.EventCount, error)
	DurationBuckets(ctx context.Context, f domain.Filter) ([]domain.Bucket, error)
	TopInsights(ctx context.Context, limit int) ([]domain.Insight, error)
}

type repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) Repository {
	return &repository{pool: pool}
}

// Хелпер для pgx, чтобы передавать NULL, если строка пустая
func ns(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
