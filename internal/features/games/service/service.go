package service

import (
	"context"
	"sync"
	"time"

	"beeline-games/internal/core/domain"
	"beeline-games/internal/core/llm"
	"beeline-games/internal/features/games/repository/postgres"
)

type LLMProvider interface {
	Chat(ctx context.Context, msgs []llm.Msg) (string, error)
}

type InsightProvider interface {
	GetInsightsHint(ctx context.Context) string
}

type Service interface {
	CreateAndGenerate(ctx context.Context, ownerID, prompt string) (domain.Game, string, int, error)
	IterateGame(ctx context.Context, userID, gameID, instruction string) (domain.Game, string, int, error)

	GetGame(ctx context.Context, userID, gameID string) (domain.Game, error)
	GetMyGames(ctx context.Context, userID string, limit, offset int) ([]domain.Game, error)
	GetGallery(ctx context.Context, sort, q string, limit, offset int) ([]domain.Game, error)

	PatchGame(ctx context.Context, userID, gameID, title string) (domain.Game, error)
	DeleteGame(ctx context.Context, userID, gameID string) error
	PublishGame(ctx context.Context, userID, gameID string, publish bool) (domain.Game, error)

	GetMessages(ctx context.Context, userID, gameID string) ([]domain.Message, error)
	GetVersions(ctx context.Context, userID, gameID string) ([]domain.VersionInfo, error)
	RestoreVersion(ctx context.Context, userID, gameID string, version int) (domain.Game, string, int, error)

	IsPublished(ctx context.Context, gameID string) (bool, error)
	CheckOwnership(ctx context.Context, userID, gameID string) (bool, error)
}

type service struct {
	repo       postgres.Repository
	llm        LLMProvider
	insights   InsightProvider
	llmTimeout time.Duration

	genSem   chan struct{}
	inflight sync.Map
}

func NewService(repo postgres.Repository, llmProv LLMProvider, insights InsightProvider, maxConcurrent int, llmTimeout time.Duration) Service {
	return &service{
		repo:       repo,
		llm:        llmProv,
		insights:   insights,
		llmTimeout: llmTimeout,
		genSem:     make(chan struct{}, maxConcurrent),
	}
}
