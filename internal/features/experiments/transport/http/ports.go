package http

import (
	"context"

	"beeline-games/internal/core/domain"
)

type Experiments interface {
	Create(ctx context.Context, ownerID, prompt string, variants int) (domain.Experiment, []domain.Variant, error)
	Get(ctx context.Context, userID, id string) (domain.Experiment, []domain.Variant, error)
	List(ctx context.Context, userID string) ([]domain.Experiment, error)
	Launch(ctx context.Context, userID, id string) (domain.Experiment, []domain.Variant, error)
	ChooseWinner(ctx context.Context, userID, id, gameID string) (domain.Experiment, []domain.Variant, error)
	Report(ctx context.Context, userID, id string) (domain.ExperimentReport, error)
}
