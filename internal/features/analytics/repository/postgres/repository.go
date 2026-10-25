package postgres

import (
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func ns(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
