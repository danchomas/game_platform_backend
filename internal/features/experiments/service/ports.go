package service

import (
	"context"
	"time"

	"beeline-games/internal/core/domain"
)

type ExperimentRepository interface {
	Create(ctx context.Context, ownerID, title, prompt string) (string, error)
	Get(ctx context.Context, id string) (domain.Experiment, error)
	ListOwner(ctx context.Context, ownerID string, limit int) ([]domain.Experiment, error)
	Variants(ctx context.Context, id string) ([]domain.Variant, error)
	// Launch и Finish атомарно меняют статус; false — эксперимент не в том статусе.
	Launch(ctx context.Context, id string) (bool, error)
	Finish(ctx context.Context, id, winnerID string) (bool, error)
	Routing(ctx context.Context, id string) (domain.ExperimentRouting, error)
	VariantStats(ctx context.Context, id string, since time.Time) ([]domain.VariantStats, error)
}

// VariantGames — то, что эксперименту нужно от каталога игр: варианты — обычные игры.
type VariantGames interface {
	StartVariant(ctx context.Context, in domain.VariantInput) (string, error)
	PublishGame(ctx context.Context, userID, gameID string, publish bool) (domain.Game, error)
}
