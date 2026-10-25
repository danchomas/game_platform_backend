package http

import (
	"context"

	"beeline-games/internal/core/domain"
)

type Analytics interface {
	GetOverview(ctx context.Context, userID, role, scope string, days int) (domain.Dashboard, error)
	GetGameAnalytics(ctx context.Context, userID, role, gameID string, days int) (domain.Dashboard, error)
}
