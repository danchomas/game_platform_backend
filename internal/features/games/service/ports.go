package service

import (
	"context"
	"time"

	"beeline-games/internal/core/agent"
	"beeline-games/internal/core/domain"
)

type GameRepository interface {
	CreateGame(ctx context.Context, ownerID, prompt string) (string, error)
	CreateVariant(ctx context.Context, in domain.VariantInput) (string, error)
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

	PipelineRepository
}

// PipelineRepository — состояние агентного конвейера и телеметрия генераций.
type PipelineRepository interface {
	SetStatus(ctx context.Context, id, status, detail string) error
	// TrySetStatus атомарно переводит игру в status, только если текущий статус входит в from.
	TrySetStatus(ctx context.Context, id, status string, from []string) (bool, error)
	SetSpec(ctx context.Context, id string, spec domain.GameSpec) error
	AddMessage(ctx context.Context, gameID, role, content string) error
	ResetInterrupted(ctx context.Context) (int64, error)

	StartGeneration(ctx context.Context, gameID, kind string) (int64, error)
	FinishGeneration(ctx context.Context, g domain.GenerationResult) error
	ListGenerations(ctx context.Context, gameID string, limit int) ([]domain.Generation, error)
	LatestScreenshot(ctx context.Context, gameID string) ([]byte, error)
	GenerationStats(ctx context.Context, from time.Time) (domain.GenerationStats, error)
}

// Agent — агентный конвейер из internal/core/agent: план, сборка, проверки, исправления.
type Agent interface {
	Plan(ctx context.Context, in agent.PlanInput, tr *agent.Trace) (domain.GameSpec, error)
	Build(ctx context.Context, in agent.BuildInput, stage agent.Stage, tr *agent.Trace) (agent.BuildOutput, error)
}

type InsightProvider interface {
	GetInsightsHint(ctx context.Context) string
}
