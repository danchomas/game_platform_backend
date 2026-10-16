package postgres

import (
	"context"
	"errors"

	"beeline-games/internal/core/domain"
	coreErrs "beeline-games/internal/core/errors"

	"github.com/jackc/pgx/v5"
)

func (r *repository) Messages(ctx context.Context, gameID string) ([]domain.Message, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id,role,content,version,created_at FROM messages WHERE game_id=$1 ORDER BY id`, gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Message
	for rows.Next() {
		var m domain.Message
		var v *int32 // pgx работает с указателями для NULL
		if err := rows.Scan(&m.ID, &m.Role, &m.Content, &v, &m.CreatedAt); err != nil {
			return nil, err
		}
		if v != nil {
			val := int(*v)
			m.Version = &val
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *repository) Versions(ctx context.Context, gameID string) ([]domain.VersionInfo, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT version,instruction,created_at FROM game_versions WHERE game_id=$1 ORDER BY version DESC`, gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.VersionInfo
	for rows.Next() {
		var v domain.VersionInfo
		if err := rows.Scan(&v.Version, &v.Instruction, &v.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r *repository) VersionHTML(ctx context.Context, gameID string, v int) (string, error) {
	var h string
	err := r.pool.QueryRow(ctx,
		`SELECT html FROM game_versions WHERE game_id=$1 AND version=$2`, gameID, v).Scan(&h)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", coreErrs.ErrNotFound
	}
	return h, err
}
