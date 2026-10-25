package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"beeline-games/internal/core/apperr"
	"beeline-games/internal/core/domain"
)

const gameCols = `g.id, g.owner_id, u.name, g.title, g.genre, g.audience, g.prompt, g.version, g.published, g.plays,
	g.status, g.status_detail, coalesce(g.spec::text,''), g.created_at, g.updated_at`

// gameDest — поля domain.Game в порядке gameCols.
func gameDest(g *domain.Game, spec *string) []any {
	return []any{&g.ID, &g.OwnerID, &g.OwnerName, &g.Title, &g.Genre, &g.Audience, &g.Prompt,
		&g.Version, &g.Published, &g.Plays, &g.Status, &g.StatusDetail, spec, &g.CreatedAt, &g.UpdatedAt}
}

func (r *Repository) GetGame(ctx context.Context, id string) (domain.Game, error) {
	var g domain.Game
	var spec string
	err := r.pool.QueryRow(ctx,
		`SELECT `+gameCols+`, g.html FROM games g JOIN users u ON u.id=g.owner_id WHERE g.id=$1`, id).
		Scan(append(gameDest(&g, &spec), &g.HTML)...)
	if spec != "" {
		g.Spec = []byte(spec)
	}

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return domain.Game{}, apperr.ErrNotFound
	case err != nil:
		return domain.Game{}, fmt.Errorf("чтение игры: %w", err)
	}
	return g, nil
}

// ListOwnerGames отдаёт и черновики без кода: план ждёт подтверждения или идёт сборка.
func (r *Repository) ListOwnerGames(ctx context.Context, ownerID string, q domain.GameQuery) ([]domain.Game, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+gameCols+` FROM games g JOIN users u ON u.id=g.owner_id
		 WHERE g.owner_id=$1 ORDER BY g.updated_at DESC LIMIT $2 OFFSET $3`,
		ownerID, q.Limit, q.Offset)
	if err != nil {
		return nil, fmt.Errorf("выборка игр владельца: %w", err)
	}
	return scanGames(rows)
}

func (r *Repository) ListPublished(ctx context.Context, q domain.GameQuery) ([]domain.Game, error) {
	order := "g.created_at DESC"
	if q.Sort == domain.SortPopular {
		order = "g.plays DESC, g.created_at DESC"
	}

	rows, err := r.pool.Query(ctx,
		`SELECT `+gameCols+` FROM games g JOIN users u ON u.id=g.owner_id
		 WHERE g.published AND ($1='' OR g.title ILIKE '%'||$1||'%' OR g.genre ILIKE '%'||$1||'%')
		 ORDER BY `+order+` LIMIT $2 OFFSET $3`,
		q.Search, q.Limit, q.Offset)
	if err != nil {
		return nil, fmt.Errorf("выборка витрины: %w", err)
	}
	return scanGames(rows)
}

func (r *Repository) IsPublished(ctx context.Context, id string) (bool, error) {
	var p bool
	err := r.pool.QueryRow(ctx, `SELECT published FROM games WHERE id=$1`, id).Scan(&p)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return false, nil
	case err != nil:
		return false, fmt.Errorf("чтение статуса публикации: %w", err)
	}
	return p, nil
}

func scanGames(rows pgx.Rows) ([]domain.Game, error) {
	defer rows.Close()

	var out []domain.Game
	for rows.Next() {
		var g domain.Game
		var spec string
		if err := rows.Scan(gameDest(&g, &spec)...); err != nil {
			return nil, fmt.Errorf("разбор строки игры: %w", err)
		}
		if spec != "" {
			g.Spec = []byte(spec)
		}
		out = append(out, g)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("обход выборки игр: %w", err)
	}
	return out, nil
}
