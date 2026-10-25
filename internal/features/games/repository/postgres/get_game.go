package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"beeline-games/internal/core/domain"
	coreErrs "beeline-games/internal/core/errors"

	"github.com/jackc/pgx/v5"
)

const gameCols = `g.id, g.owner_id, u.name, g.title, g.genre, g.audience, g.prompt, g.version, g.published, g.plays,
	g.status, g.status_detail, coalesce(g.spec::text,''), g.created_at, g.updated_at`

func scanGame(row pgx.Row, extra ...any) (domain.Game, error) {
	var g domain.Game
	var spec string
	dest := append([]any{&g.ID, &g.OwnerID, &g.OwnerName, &g.Title, &g.Genre, &g.Audience, &g.Prompt,
		&g.Version, &g.Published, &g.Plays, &g.Status, &g.StatusDetail, &spec, &g.CreatedAt, &g.UpdatedAt}, extra...)
	if err := row.Scan(dest...); err != nil {
		return g, err
	}
	if spec != "" {
		g.Spec = json.RawMessage(spec)
	}
	return g, nil
}

func (r *repository) GetGame(ctx context.Context, id string) (domain.Game, error) {
	var html string
	g, err := scanGame(r.pool.QueryRow(ctx,
		`SELECT `+gameCols+`, g.html FROM games g JOIN users u ON u.id=g.owner_id WHERE g.id=$1`, id), &html)
	if errors.Is(err, pgx.ErrNoRows) {
		return g, coreErrs.ErrNotFound
	}
	g.HTML = html
	return g, err
}

func (r *repository) listGames(ctx context.Context, query string, args ...any) ([]domain.Game, error) {
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Game
	for rows.Next() {
		g, err := scanGame(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// ListOwnerGames возвращает и черновики (план без кода), чтобы пользователь видел статус.
func (r *repository) ListOwnerGames(ctx context.Context, ownerID string, limit, offset int) ([]domain.Game, error) {
	return r.listGames(ctx,
		`SELECT `+gameCols+` FROM games g JOIN users u ON u.id=g.owner_id
		 WHERE g.owner_id=$1 ORDER BY g.updated_at DESC LIMIT $2 OFFSET $3`, ownerID, limit, offset)
}

func (r *repository) ListPublished(ctx context.Context, sort, q string, limit, offset int) ([]domain.Game, error) {
	order := "g.created_at DESC"
	if sort == "popular" {
		order = "g.plays DESC, g.created_at DESC"
	}
	return r.listGames(ctx, `SELECT `+gameCols+` FROM games g JOIN users u ON u.id=g.owner_id
		 WHERE g.published AND ($1='' OR g.title ILIKE '%'||$1||'%' OR g.genre ILIKE '%'||$1||'%')
		 ORDER BY `+order+` LIMIT $2 OFFSET $3`, q, limit, offset)
}

func (r *repository) IsPublished(ctx context.Context, id string) (bool, error) {
	var p bool
	err := r.pool.QueryRow(ctx, `SELECT published FROM games WHERE id=$1`, id).Scan(&p)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return p, err
}
