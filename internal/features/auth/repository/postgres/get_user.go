package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"beeline-games/internal/core/apperr"
	"beeline-games/internal/core/domain"
)

func (r *Repository) GetUserByAccessKey(ctx context.Context, key string) (domain.User, error) {
	return r.queryUser(ctx, `SELECT id,name,role,created_at FROM users WHERE access_key=$1`, key)
}

func (r *Repository) GetUserByID(ctx context.Context, id string) (domain.User, error) {
	return r.queryUser(ctx, `SELECT id,name,role,created_at FROM users WHERE id=$1`, id)
}

func (r *Repository) queryUser(ctx context.Context, query string, arg any) (domain.User, error) {
	var u domain.User
	err := r.pool.QueryRow(ctx, query, arg).
		Scan(&u.ID, &u.Name, &u.Role, &u.CreatedAt)

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return domain.User{}, apperr.ErrNotFound
	case err != nil:
		return domain.User{}, fmt.Errorf("чтение пользователя: %w", err)
	}
	return u, nil
}
