package service

import (
	"context"

	"beeline-games/internal/core/collector"
	"beeline-games/internal/core/domain"
)

type GamesProvider interface {
	GetGame(ctx context.Context, userID, gameID string) (domain.Game, error)
	IsPublished(ctx context.Context, gameID string) (bool, error)
}

type CollectorProvider interface {
	Add(e collector.Event)
}
