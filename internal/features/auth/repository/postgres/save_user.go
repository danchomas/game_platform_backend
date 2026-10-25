package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"beeline-games/internal/core/domain"
)

func (r *Repository) SaveUser(ctx context.Context, name, role, accessKey string) (domain.User, error) {
	u := domain.User{
		ID:   uuid.NewString(),
		Name: name,
		Role: role,
	}

	err := r.pool.QueryRow(ctx,
		`INSERT INTO users(id,name,role,access_key) VALUES($1,$2,$3,$4) RETURNING created_at`,
		u.ID, name, role, accessKey).Scan(&u.CreatedAt)
	if err != nil {
		return domain.User{}, fmt.Errorf("сохранение пользователя: %w", err)
	}

	return u, nil
}
