package postgres

import (
	"beeline-games/internal/core/domain"
	coreErrs "beeline-games/internal/core/errors"
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

func (r *repository) GetUserByAccessKey(ctx context.Context, key string) (domain.User, error) {
	var u domain.User
	err := r.pool.QueryRow(ctx,
		`SELECT id,name,role,created_at FROM users WHERE access_key=$1`, key).
		Scan(&u.ID, &u.Name, &u.Role, &u.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return u, coreErrs.ErrNotFound
	}
	return u, err
}

func (r *repository) GetUserByID(ctx context.Context, id string) (domain.User, error) {
	var u domain.User
	err := r.pool.QueryRow(ctx,
		`SELECT id,name,role,created_at FROM users WHERE id=$1`, id).
		Scan(&u.ID, &u.Name, &u.Role, &u.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return u, coreErrs.ErrNotFound
	}
	return u, err
}
