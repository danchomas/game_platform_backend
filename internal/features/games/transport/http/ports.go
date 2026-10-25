package http

import (
	"context"

	"beeline-games/internal/core/domain"
)

type Games interface {
	CreateAndGenerate(ctx context.Context, ownerID, prompt string) (domain.Game, string, int, error)
	IterateGame(ctx context.Context, userID, gameID, instruction string) (domain.Game, string, int, error)

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
