package http

import (
	"context"

	"beeline-games/internal/features/play/service"
)

type Play interface {
	PlayExperiment(ctx context.Context, experimentID, playerID string) (html, effectivePlayerID string, err error)
	GetPlayableHTML(ctx context.Context, userID, gameID string) (string, bool, error)
	ProcessEvents(ctx context.Context, batch service.EventBatch) error
}
