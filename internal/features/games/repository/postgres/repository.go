package postgres

import (
	"beeline-games/internal/core/domain"
	"context"

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
