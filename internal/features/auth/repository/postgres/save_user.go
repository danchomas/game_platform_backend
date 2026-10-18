package postgres

import (
	"beeline-games/internal/core/domain"
	"context"

	"github.com/google/uuid"
)

func (r *repository) SaveUser(ctx context.Context, name, role, accessKey string) (domain.User, error) {
	u := domain.User{
		ID:	uuid.NewString(),
		Name:	name,
		Role:	role,
	}

	err := r.pool.QueryRow(ctx,
		`INSERT INTO users(id,name,role,access_key) VALUES($1,$2,$3,$4) RETURNING created_at`,
		u.ID, name, role, accessKey).Scan(&u.CreatedAt)

	return u, err
}
