package http

import (
	"context"

	"beeline-games/internal/features/play/service"
)

type Play interface {
	GetPlayableHTML(ctx context.Context, userID, gameID string) (string, bool, error)
	ProcessEvents(ctx context.Context, batch service.EventBatch) error
}
