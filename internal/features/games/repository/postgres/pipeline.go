package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"beeline-games/internal/core/apperr"
	"beeline-games/internal/core/domain"

	"github.com/jackc/pgx/v5"
)

func (r *Repository) SetStatus(ctx context.Context, id, status, detail string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE games SET status=$2, status_detail=$3, updated_at=now() WHERE id=$1`, id, status, detail)
	if err != nil {
		return fmt.Errorf("смена статуса генерации: %w", err)
	}
	return nil
}

// TrySetStatus атомарно переводит игру в новый статус, только если текущий входит в from.
func (r *Repository) TrySetStatus(ctx context.Context, id, status string, from []string) (bool, error) {
	tag, err := r.pool.Exec(ctx,
		`UPDATE games SET status=$2, status_detail='', updated_at=now() WHERE id=$1 AND status = ANY($3)`,
		id, status, from)
	if err != nil {
		return false, fmt.Errorf("смена статуса генерации: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

func (r *Repository) SetSpec(ctx context.Context, id string, spec domain.GameSpec) error {
	b, err := json.Marshal(spec)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx,
		`UPDATE games SET spec=$2,
		   title    = CASE WHEN $3<>'' THEN $3 ELSE title END,
		   genre    = CASE WHEN $4<>'' THEN $4 ELSE genre END,
		   audience = CASE WHEN $5<>'' THEN $5 ELSE audience END,
		   updated_at=now() WHERE id=$1`,
		id, string(b), spec.Title, spec.Genre, spec.Audience)
	return err
}

func (r *Repository) AddMessage(ctx context.Context, gameID, role, content string) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO messages(game_id,role,content) VALUES($1,$2,$3)`, gameID, role, content)
	return err
}

// ResetInterrupted помечает игры, чья генерация оборвалась перезапуском сервера.
func (r *Repository) ResetInterrupted(ctx context.Context) (int64, error) {
	tag, err := r.pool.Exec(ctx,
		`UPDATE games SET
		   status = CASE WHEN status='planning' AND spec IS NOT NULL THEN 'awaiting_approval'
		                 WHEN version > 0 THEN 'ready' ELSE 'failed' END,
		   status_detail = 'генерация прервана перезапуском сервера', updated_at=now()
		 WHERE status IN ('planning','generating','validating','testing','fixing')`)
	if err != nil {
		return 0, err
	}
	_, err = r.pool.Exec(ctx,
		`UPDATE generations SET status='failed', error='interrupted', finished_at=now() WHERE status='running'`)
	return tag.RowsAffected(), err
}

func (r *Repository) StartGeneration(ctx context.Context, gameID, kind string) (int64, error) {
	var id int64
	err := r.pool.QueryRow(ctx,
		`INSERT INTO generations(game_id,kind) VALUES($1,$2) RETURNING id`, gameID, kind).Scan(&id)
	return id, err
}

func (r *Repository) FinishGeneration(ctx context.Context, g domain.GenerationResult) error {
	issues, err := json.Marshal(g.Issues)
	if err != nil {
		return err
	}
	if g.Issues == nil {
		issues = []byte("[]")
	}
	var test any
	if len(g.TestReport) > 0 {
		test = string(g.TestReport)
	}
	var pt, ct int
	var cost float64
	for _, c := range g.Calls {
		pt += c.PromptTokens
		ct += c.CompletionTokens
		cost += c.CostUSD
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx,
		`UPDATE generations SET status=$2, version=$3, prompt_tokens=$4, completion_tokens=$5, cost_usd=$6,
		   duration_ms=$7, fix_rounds=$8, issues=$9, test_report=$10, screenshot=$11, error=$12, finished_at=now()
		 WHERE id=$1`,
		g.ID, g.Status, g.Version, pt, ct, cost, g.DurationMs, g.FixRounds, string(issues), test, g.Screenshot, g.Error)
	if err != nil {
		return err
	}
	if len(g.Calls) > 0 {
		rows := make([][]any, 0, len(g.Calls))
		for _, c := range g.Calls {
			rows = append(rows, []any{g.ID, c.Role, c.Model, c.PromptTokens, c.CompletionTokens, c.CostUSD, c.LatencyMs, c.FinishReason, c.Error})
		}
		_, err = tx.CopyFrom(ctx, pgx.Identifier{"llm_calls"},
			[]string{"generation_id", "role", "model", "prompt_tokens", "completion_tokens", "cost_usd", "latency_ms", "finish_reason", "error"},
			pgx.CopyFromRows(rows))
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *Repository) ListGenerations(ctx context.Context, gameID string, limit int) ([]domain.Generation, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, game_id, kind, status, version, prompt_tokens, completion_tokens, cost_usd, duration_ms,
		        fix_rounds, issues::text, coalesce(test_report::text,''), error, screenshot IS NOT NULL, created_at, finished_at
		 FROM generations WHERE game_id=$1 ORDER BY id DESC LIMIT $2`, gameID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Generation
	for rows.Next() {
		var g domain.Generation
		var v *int32
		var issues, test string
		if err := rows.Scan(&g.ID, &g.GameID, &g.Kind, &g.Status, &v, &g.PromptTokens, &g.CompletionTokens,
			&g.CostUSD, &g.DurationMs, &g.FixRounds, &issues, &test, &g.Error, &g.HasScreenshot,
			&g.CreatedAt, &g.FinishedAt); err != nil {
			return nil, err
		}
		if v != nil {
			n := int(*v)
			g.Version = &n
		}
		_ = json.Unmarshal([]byte(issues), &g.Issues)
		if test != "" {
			g.TestReport = []byte(test)
		}
		out = append(out, g)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(out) > 0 {
		ids := make([]int64, len(out))
		idx := make(map[int64]int, len(out))
		for i, g := range out {
			ids[i], idx[g.ID] = g.ID, i
		}
		crows, err := r.pool.Query(ctx,
			`SELECT generation_id, role, model, prompt_tokens, completion_tokens, cost_usd, latency_ms, finish_reason, error
			 FROM llm_calls WHERE generation_id = ANY($1) ORDER BY id`, ids)
		if err != nil {
			return nil, err
		}
		defer crows.Close()
		for crows.Next() {
			var gid int64
			var c domain.LLMCall
			if err := crows.Scan(&gid, &c.Role, &c.Model, &c.PromptTokens, &c.CompletionTokens, &c.CostUSD,
				&c.LatencyMs, &c.FinishReason, &c.Error); err != nil {
				return nil, err
			}
			out[idx[gid]].Calls = append(out[idx[gid]].Calls, c)
		}
		if err := crows.Err(); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (r *Repository) LatestScreenshot(ctx context.Context, gameID string) ([]byte, error) {
	var b []byte
	err := r.pool.QueryRow(ctx,
		`SELECT screenshot FROM generations WHERE game_id=$1 AND screenshot IS NOT NULL ORDER BY id DESC LIMIT 1`,
		gameID).Scan(&b)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.ErrNotFound
	}
	return b, err
}

func (r *Repository) GenerationStats(ctx context.Context, from time.Time) (domain.GenerationStats, error) {
	var s domain.GenerationStats
	err := r.pool.QueryRow(ctx,
		`SELECT count(*) FILTER (WHERE kind IN ('build','edit')),
		        count(*) FILTER (WHERE kind IN ('build','edit') AND status='ready'),
		        count(*) FILTER (WHERE kind IN ('build','edit') AND status='ready_with_issues'),
		        count(*) FILTER (WHERE kind IN ('build','edit') AND status='failed'),
		        coalesce(sum(cost_usd),0),
		        coalesce(avg(duration_ms) FILTER (WHERE kind IN ('build','edit'))/1000.0,0),
		        coalesce(avg(fix_rounds) FILTER (WHERE kind IN ('build','edit')),0)
		 FROM generations WHERE created_at >= $1 AND status <> 'running'`, from).
		Scan(&s.Total, &s.Ready, &s.WithIssues, &s.Failed, &s.TotalCostUSD, &s.AvgDurationSec, &s.AvgFixRounds)
	if err != nil {
		return s, err
	}
	// success = без блокирующих проблем; стоимость на сборку включает и планирование
	if s.Total > 0 {
		s.SuccessRate = float64(s.Ready) / float64(s.Total)
		s.AvgCostUSD = s.TotalCostUSD / float64(s.Total)
	}

	rows, err := r.pool.Query(ctx,
		`SELECT i->>'code', count(*) FROM generations g, jsonb_array_elements(g.issues) i
		 WHERE g.created_at >= $1 GROUP BY 1 ORDER BY 2 DESC LIMIT 10`, from)
	if err != nil {
		return s, err
	}
	for rows.Next() {
		var b domain.Bucket
		if err := rows.Scan(&b.Label, &b.Count); err != nil {
			rows.Close()
			return s, err
		}
		s.TopIssues = append(s.TopIssues, b)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return s, err
	}

	rows, err = r.pool.Query(ctx,
		`SELECT c.role, c.model, count(*), coalesce(sum(c.cost_usd),0), coalesce(avg(c.latency_ms),0),
		        count(*) FILTER (WHERE c.finish_reason IN ('length','max_tokens')),
		        count(*) FILTER (WHERE c.error <> '')
		 FROM llm_calls c WHERE c.created_at >= $1 GROUP BY 1,2 ORDER BY 4 DESC`, from)
	if err != nil {
		return s, err
	}
	defer rows.Close()
	for rows.Next() {
		var m domain.ModelStat
		if err := rows.Scan(&m.Role, &m.Model, &m.Calls, &m.CostUSD, &m.AvgLatencyMs, &m.Truncated, &m.Errors); err != nil {
			return s, err
		}
		s.ByModel = append(s.ByModel, m)
	}
	return s, rows.Err()
}
