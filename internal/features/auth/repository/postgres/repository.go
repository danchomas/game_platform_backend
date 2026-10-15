package postgres

import (
	"beeline-games/internal/core/domain"
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	SaveUser(ctx context.Context, name, role, accessKey string) (domain.User, error)
	GetUserByAccessKey(ctx context.Context, key string) (domain.User, error)
	GetUserByID(ctx context.Context, id string) (domain.User, error)
}

type repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) Repository {
	return &repository{pool: pool}
}
