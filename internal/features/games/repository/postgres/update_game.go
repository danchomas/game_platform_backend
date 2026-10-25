package postgres

import (
	"context"
	"fmt"
)

func (r *Repository) SetPublished(ctx context.Context, id string, published bool) error {
	_, err := r.pool.Exec(ctx, `UPDATE games SET published=$2, updated_at=now() WHERE id=$1`, id, published)
	if err != nil {
		return fmt.Errorf("смена статуса публикации: %w", err)
	}
	return nil
}

func (r *Repository) SetTitle(ctx context.Context, id, title string) error {
	_, err := r.pool.Exec(ctx, `UPDATE games SET title=$2, updated_at=now() WHERE id=$1`, id, title)
	if err != nil {
		return fmt.Errorf("переименование игры: %w", err)
	}
	return nil
}

func (r *Repository) DeleteGame(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM games WHERE id=$1`, id)
	if err != nil {
		return fmt.Errorf("удаление игры: %w", err)
	}
	return nil
}
