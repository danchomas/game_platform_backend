package http

import (
	"context"

	"beeline-games/internal/core/domain"
)

type Games interface {
	// агентный конвейер: план → правка плана → подтверждение → фоновая сборка → правки в чате
	CreateGame(ctx context.Context, ownerID, prompt string, autoApprove bool) (domain.Game, domain.Plan, error)
	RevisePlan(ctx context.Context, userID, gameID, feedback string) (domain.Game, domain.Plan, error)
	ApprovePlan(ctx context.Context, userID, gameID string, spec *domain.GameSpec) (domain.Game, error)
	IterateGame(ctx context.Context, userID, gameID, instruction string) (domain.Game, error)
	GetStatus(ctx context.Context, userID, gameID string) (domain.Game, error)
	Subscribe(ctx context.Context, userID, gameID string) (domain.Game, <-chan domain.StatusEvent, func(), error)
	ListGenerations(ctx context.Context, userID, gameID string) ([]domain.Generation, error)
	Screenshot(ctx context.Context, userID, gameID string) ([]byte, error)
	GenerationStats(ctx context.Context, role string, days int) (domain.GenerationStats, error)

	GetGame(ctx context.Context, userID, gameID string) (domain.Game, error)
	GetMyGames(ctx context.Context, userID string, limit, offset int) ([]domain.Game, error)
	GetGallery(ctx context.Context, sort, search string, limit, offset int) ([]domain.Game, error)

	PatchGame(ctx context.Context, userID, gameID, title string) (domain.Game, error)
	DeleteGame(ctx context.Context, userID, gameID string) error
	PublishGame(ctx context.Context, userID, gameID string, publish bool) (domain.Game, error)

	GetMessages(ctx context.Context, userID, gameID string) ([]domain.Message, error)
	GetVersions(ctx context.Context, userID, gameID string) ([]domain.VersionInfo, error)
	RestoreVersion(ctx context.Context, userID, gameID string, version int) (domain.Game, string, int, error)
}
