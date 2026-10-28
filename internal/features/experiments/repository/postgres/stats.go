package postgres

import (
	"context"
	"fmt"
	"time"

	"beeline-games/internal/core/domain"
)

// VariantStats считает воронку только по сессиям, начатым показом по ссылке эксперимента:
// прямые заходы на /g/{id} варианта в сравнение не попадают. Показ и события SDK
// связаны session_id, который сервер выдаёт при показе.
func (r *Repository) VariantStats(ctx context.Context, id string, since time.Time) ([]domain.VariantStats, error) {
	rows, err := r.pool.Query(ctx, `
	  WITH shown AS (
	    SELECT DISTINCT e.game_id, e.session_id FROM events e
	    JOIN games g ON g.id=e.game_id
	    WHERE g.experiment_id=$1 AND e.type='exposure' AND e.ts >= $2)
	  SELECT g.id,
	    count(DISTINCT s.session_id),
	    count(DISTINCT e.session_id) FILTER (WHERE e.type='session_start'),
	    count(DISTINCT e.session_id) FILTER (WHERE e.type='engaged'),
	    count(DISTINCT e.session_id) FILTER (WHERE e.type='session_end' AND e.duration_ms >= $3),
	    count(DISTINCT e.session_id) FILTER (WHERE e.type='js_error'),
	    count(DISTINCT NULLIF(e.player_id,'')),
	    count(*) FILTER (WHERE e.type='game_start'),
	    count(*) FILTER (WHERE e.type='game_end'),
	    count(*) FILTER (WHERE e.type='game_end' AND e.result='win'),
	    coalesce(avg(e.duration_ms) FILTER (WHERE e.type='session_end')/1000.0, 0)
	  FROM games g
	  LEFT JOIN shown s ON s.game_id=g.id
	  LEFT JOIN events e ON e.game_id=s.game_id AND e.session_id=s.session_id AND e.ts >= $2
	  WHERE g.experiment_id=$1
	  GROUP BY g.id, g.variant_label ORDER BY g.variant_label`,
		id, since, int64(domain.RetainedSeconds*1000))
	if err != nil {
		return nil, fmt.Errorf("статистика вариантов: %w", err)
	}
	defer rows.Close()

	var out []domain.VariantStats
	for rows.Next() {
		var st domain.VariantStats
		if err := rows.Scan(&st.GameID, &st.Exposures, &st.Sessions, &st.Engaged, &st.Retained,
			&st.ErrorSessions, &st.Players, &st.Starts, &st.Ends, &st.Wins, &st.AvgSessionSec); err != nil {
			return nil, fmt.Errorf("разбор статистики варианта: %w", err)
		}
		st.Derive()
		out = append(out, st)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("обход статистики вариантов: %w", err)
	}
	return out, nil
}

// Arms — данные для бандита: показы до before и сколько из них удержали игрока.
// Свежие показы в выборку не входят: их сессия ещё идёт, и засчитать её неудачей
// значило бы наказать вариант за то, что в него сейчас играют. Показы бывают только
// у запущенного эксперимента, поэтому нижняя граница по времени не нужна.
func (r *Repository) Arms(ctx context.Context, id string, before time.Time) ([]domain.ArmStats, error) {
	rows, err := r.pool.Query(ctx, `
	  WITH shown AS (
	    SELECT DISTINCT e.game_id, e.session_id FROM events e
	    JOIN games g ON g.id=e.game_id
	    WHERE g.experiment_id=$1 AND e.type='exposure' AND e.ts < $2)
	  SELECT s.game_id::text, count(DISTINCT s.session_id),
	    count(DISTINCT e.session_id) FILTER (WHERE e.duration_ms >= $3)
	  FROM shown s
	  LEFT JOIN events e ON e.game_id=s.game_id AND e.session_id=s.session_id AND e.type='session_end'
	  GROUP BY s.game_id`,
		id, before, int64(domain.RetainedSeconds*1000))
	if err != nil {
		return nil, fmt.Errorf("данные бандита: %w", err)
	}
	defer rows.Close()

	var out []domain.ArmStats
	for rows.Next() {
		var a domain.ArmStats
		if err := rows.Scan(&a.GameID, &a.Trials, &a.Successes); err != nil {
			return nil, fmt.Errorf("разбор данных бандита: %w", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("обход данных бандита: %w", err)
	}
	return out, nil
}
