package postgres

import (
	"beeline-games/internal/core/domain"
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	CreateGame(ctx context.Context, ownerID, prompt string) (string, error)
	GetGame(ctx context.Context, id string) (domain.Game, error)
	ListOwnerGames(ctx context.Context, ownerID string, limit, offset int) ([]domain.Game, error)
	ListPublished(ctx context.Context, sort, q string, limit, offset int) ([]domain.Game, error)

	AddVersion(ctx context.Context, in domain.VersionInput) (int, error)
	Messages(ctx context.Context, gameID string) ([]domain.Message, error)
	Versions(ctx context.Context, gameID string) ([]domain.VersionInfo, error)
	VersionHTML(ctx context.Context, gameID string, version int) (string, error)

	SetStatus(ctx context.Context, id, status, detail string) error
	TrySetStatus(ctx context.Context, id, status string, from []string) (bool, error)
	SetSpec(ctx context.Context, id string, spec domain.GameSpec) error
	AddMessage(ctx context.Context, gameID, role, content string) error
	ResetInterrupted(ctx context.Context) (int64, error)

	StartGeneration(ctx context.Context, gameID, kind string) (int64, error)
	FinishGeneration(ctx context.Context, g domain.GenerationResult) error
	ListGenerations(ctx context.Context, gameID string, limit int) ([]domain.Generation, error)
	LatestScreenshot(ctx context.Context, gameID string) ([]byte, error)
	GenerationStats(ctx context.Context, from time.Time) (domain.GenerationStats, error)

	SetPublished(ctx context.Context, id string, published bool) error
	SetTitle(ctx context.Context, id, title string) error
	DeleteGame(ctx context.Context, id string) error
	IsPublished(ctx context.Context, id string) (bool, error)
}

type repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) Repository {
	return &repository{pool: pool}
}
