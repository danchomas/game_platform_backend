package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"beeline-games/internal/core/apperr"
	"beeline-games/internal/core/domain"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

const experimentCols = `x.id, x.owner_id, x.title, x.prompt, x.status, coalesce(x.winner_id::text,''), x.winner_source,
	x.created_at, x.launched_at, x.finished_at,
	count(g.id), count(g.id) FILTER (WHERE g.status='ready' AND g.version>0)`

func experimentDest(x *domain.Experiment) []any {
	return []any{&x.ID, &x.OwnerID, &x.Title, &x.Prompt, &x.Status, &x.WinnerID, &x.WinnerSource,
		&x.CreatedAt, &x.LaunchedAt, &x.FinishedAt, &x.Variants, &x.Ready}
}

func (r *Repository) Create(ctx context.Context, ownerID, title, prompt string) (string, error) {
	id := uuid.NewString()
	_, err := r.pool.Exec(ctx,
		`INSERT INTO experiments(id,owner_id,title,prompt) VALUES($1,$2,$3,$4)`, id, ownerID, title, prompt)
	if err != nil {
		return "", fmt.Errorf("создание эксперимента: %w", err)
	}
	return id, nil
}

func (r *Repository) Get(ctx context.Context, id string) (domain.Experiment, error) {
	var x domain.Experiment
	err := r.pool.QueryRow(ctx,
		`SELECT `+experimentCols+` FROM experiments x LEFT JOIN games g ON g.experiment_id=x.id
		 WHERE x.id=$1 GROUP BY x.id`, id).Scan(experimentDest(&x)...)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return domain.Experiment{}, apperr.ErrNotFound
	case err != nil:
		return domain.Experiment{}, fmt.Errorf("чтение эксперимента: %w", err)
	}
	return x, nil
}

func (r *Repository) ListOwner(ctx context.Context, ownerID string, limit int) ([]domain.Experiment, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+experimentCols+` FROM experiments x LEFT JOIN games g ON g.experiment_id=x.id
		 WHERE x.owner_id=$1 GROUP BY x.id ORDER BY x.created_at DESC LIMIT $2`, ownerID, limit)
	if err != nil {
		return nil, fmt.Errorf("выборка экспериментов: %w", err)
	}
	defer rows.Close()

	var out []domain.Experiment
	for rows.Next() {
		var x domain.Experiment
		if err := rows.Scan(experimentDest(&x)...); err != nil {
			return nil, fmt.Errorf("разбор эксперимента: %w", err)
		}
		out = append(out, x)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("обход экспериментов: %w", err)
	}
	return out, nil
}

// Variants отдаёт варианты с итогом последней удачной сборки или правки: неудачная правка
// оставляет рабочую версию, поэтому на допуск к эксперименту она не влияет.
func (r *Repository) Variants(ctx context.Context, id string) ([]domain.Variant, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT g.id, g.variant_label, g.variant_direction, g.title, g.genre, g.status, g.status_detail,
		        g.version, g.published, coalesce(b.status,''), coalesce(b.fix_rounds,0), coalesce(c.cost,0)
		 FROM games g
		 LEFT JOIN LATERAL (
		   SELECT status, fix_rounds FROM generations
		   WHERE game_id=g.id AND kind IN ('build','edit') AND status IN ('ready','ready_with_issues')
		   ORDER BY id DESC LIMIT 1) b ON true
		 LEFT JOIN LATERAL (SELECT sum(cost_usd) AS cost FROM generations WHERE game_id=g.id) c ON true
		 WHERE g.experiment_id=$1 ORDER BY g.variant_label`, id)
	if err != nil {
		return nil, fmt.Errorf("выборка вариантов: %w", err)
	}
	defer rows.Close()

	var out []domain.Variant
	for rows.Next() {
		var v domain.Variant
		if err := rows.Scan(&v.GameID, &v.Label, &v.Direction, &v.Title, &v.Genre, &v.Status, &v.StatusDetail,
			&v.Version, &v.Published, &v.BuildStatus, &v.FixRounds, &v.CostUSD); err != nil {
			return nil, fmt.Errorf("разбор варианта: %w", err)
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("обход вариантов: %w", err)
	}
	return out, nil
}

// Launch переводит черновик в работу; false — эксперимент уже запущен или завершён.
func (r *Repository) Launch(ctx context.Context, id string) (bool, error) {
	tag, err := r.pool.Exec(ctx,
		`UPDATE experiments SET status='running', launched_at=now() WHERE id=$1 AND status='draft'`, id)
	if err != nil {
		return false, fmt.Errorf("запуск эксперимента: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// Finish фиксирует победителя. Автор может поменять выбор и после завершения,
// бандит — только завершить идущий эксперимент: решение автора он не перебивает.
func (r *Repository) Finish(ctx context.Context, id, winnerID, source string) (bool, error) {
	tag, err := r.pool.Exec(ctx,
		`UPDATE experiments SET status='finished', winner_id=$2, winner_source=$3, finished_at=now()
		 WHERE id=$1 AND (status='running' OR (status='finished' AND $3='manual'))`, id, winnerID, source)
	if err != nil {
		return false, fmt.Errorf("выбор победителя: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

func (r *Repository) Routing(ctx context.Context, id string) (domain.ExperimentRouting, error) {
	var rt domain.ExperimentRouting
	err := r.pool.QueryRow(ctx,
		`SELECT x.status, coalesce(x.winner_id::text,''),
		        coalesce(array_agg(g.id::text ORDER BY g.variant_label)
		                 FILTER (WHERE g.published AND g.version>0), '{}')
		 FROM experiments x LEFT JOIN games g ON g.experiment_id=x.id
		 WHERE x.id=$1 GROUP BY x.id`, id).Scan(&rt.Status, &rt.WinnerID, &rt.Live)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return domain.ExperimentRouting{}, apperr.ErrNotFound
	case err != nil:
		return domain.ExperimentRouting{}, fmt.Errorf("чтение маршрутизации эксперимента: %w", err)
	}
	return rt, nil
}

// Assignment — вариант, за которым закреплён игрок; пусто — игрок пришёл впервые.
func (r *Repository) Assignment(ctx context.Context, id, playerID string) (string, error) {
	var gameID string
	err := r.pool.QueryRow(ctx,
		`SELECT game_id::text FROM experiment_assignments WHERE experiment_id=$1 AND player_id=$2`,
		id, playerID).Scan(&gameID)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return "", nil
	case err != nil:
		return "", fmt.Errorf("чтение закрепления игрока: %w", err)
	}
	return gameID, nil
}

// Assign закрепляет игрока за вариантом; повторный вызов переназначает его
// (прежний вариант сняли с эксперимента).
func (r *Repository) Assign(ctx context.Context, id, playerID, gameID string) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO experiment_assignments(experiment_id,player_id,game_id) VALUES($1,$2,$3)
		 ON CONFLICT (experiment_id,player_id) DO UPDATE SET game_id=EXCLUDED.game_id, created_at=now()`,
		id, playerID, gameID)
	if err != nil {
		return fmt.Errorf("закрепление игрока: %w", err)
	}
	return nil
}
