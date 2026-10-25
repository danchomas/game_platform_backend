package service

import (
	"context"

	"beeline-games/internal/core/domain"
	"beeline-games/internal/core/llm"
)

type GameRepository interface {
	CreateGame(ctx context.Context, ownerID, prompt string) (string, error)
	GetGame(ctx context.Context, id string) (domain.Game, error)
	ListOwnerGames(ctx context.Context, ownerID string, q domain.GameQuery) ([]domain.Game, error)
	ListPublished(ctx context.Context, q domain.GameQuery) ([]domain.Game, error)

	AddVersion(ctx context.Context, in domain.VersionInput) (int, error)
	Messages(ctx context.Context, gameID string) ([]domain.Message, error)
	Versions(ctx context.Context, gameID string) ([]domain.VersionInfo, error)
	VersionHTML(ctx context.Context, gameID string, version int) (string, error)

	SetPublished(ctx context.Context, id string, published bool) error
	SetTitle(ctx context.Context, id, title string) error
	DeleteGame(ctx context.Context, id string) error
	IsPublished(ctx context.Context, id string) (bool, error)
}

type LLMProvider interface {
	Chat(ctx context.Context, msgs []llm.Msg) (string, error)
}

type InsightProvider interface {
	GetInsightsHint(ctx context.Context) string
}
