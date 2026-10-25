package service

import (
	"context"

	"beeline-games/internal/core/domain"
)

type StatsRepository interface {
	Summary(ctx context.Context, f domain.Filter) (domain.Summary, error)
	Daily(ctx context.Context, f domain.Filter) ([]domain.DayPoint, error)
	PerGame(ctx context.Context, f domain.Filter) ([]domain.GameRow, error)
	CustomEvents(ctx context.Context, f domain.Filter) ([]domain.EventCount, error)
	DurationBuckets(ctx context.Context, f domain.Filter) ([]domain.Bucket, error)
	TopInsights(ctx context.Context, limit int) ([]domain.Insight, error)
}

type GameOwnership interface {
	CheckOwnership(ctx context.Context, userID, gameID string) (bool, error)
}
