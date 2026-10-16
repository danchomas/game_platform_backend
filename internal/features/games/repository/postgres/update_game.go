package postgres

import (
	"context"
)

func (r *repository) SetPublished(ctx context.Context, id string, published bool) error {
	_, err := r.pool.Exec(ctx, `UPDATE games SET published=$2, updated_at=now() WHERE id=$1`, id, published)
	return err
}

func (r *repository) SetTitle(ctx context.Context, id, title string) error {
	_, err := r.pool.Exec(ctx, `UPDATE games SET title=$2, updated_at=now() WHERE id=$1`, id, title)
	return err
}

func (r *repository) DeleteGame(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM games WHERE id=$1`, id)
	return err
}
