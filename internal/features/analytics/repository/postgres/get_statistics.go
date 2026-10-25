package postgres

import (
	"beeline-games/internal/core/domain"
	"context"
)

const reservedTypes = `'session_start','engaged','game_start','game_end','session_end'`

const aggCols = `
  count(DISTINCT e.session_id) FILTER (WHERE e.type='session_start'),
  count(DISTINCT NULLIF(e.player_id,'')),
  count(DISTINCT e.session_id) FILTER (WHERE e.type='engaged'),
  count(*) FILTER (WHERE e.type='game_start'),
  count(*) FILTER (WHERE e.type='game_end'),
  count(*) FILTER (WHERE e.type='game_end' AND e.result='win'),
  count(*) FILTER (WHERE e.type='game_end' AND e.result='lose'),
  coalesce(avg(e.duration_ms) FILTER (WHERE e.type='session_end')/1000.0,0),
  coalesce(avg(e.score) FILTER (WHERE e.type='game_end'),0),
  coalesce(max(e.score) FILTER (WHERE e.type='game_end'),0)`

const scope = `e.ts >= $1 AND e.ts < $2 AND ($3::uuid IS NULL OR g.owner_id = $3::uuid) AND ($4::uuid IS NULL OR e.game_id = $4::uuid)`

func scanAgg(sc interface{ Scan(...any) error }, pre ...any) (domain.Summary, error) {
	var m domain.Summary
	dest := append(pre, &m.Sessions, &m.Players, &m.Engaged, &m.Starts, &m.Ends, &m.Wins, &m.Losses,
		&m.AvgSessionSec, &m.AvgScore, &m.MaxScore)
	err := sc.Scan(dest...)
	m.Derive()
	return m, err
}

func (r *Repository) Summary(ctx context.Context, f domain.Filter) (domain.Summary, error) {
	m, err := scanAgg(r.pool.QueryRow(ctx,
		`SELECT `+aggCols+` FROM events e JOIN games g ON g.id=e.game_id WHERE `+scope,
		f.From, f.To, ns(f.OwnerID), ns(f.GameID)))
	if err != nil {
		return m, err
	}

	err = r.pool.QueryRow(ctx,
		`SELECT count(*) FROM (
		   SELECT e.player_id FROM events e JOIN games g ON g.id=e.game_id
		   WHERE e.type='session_start' AND e.player_id<>'' AND `+scope+`
		   GROUP BY e.player_id HAVING count(DISTINCT date_trunc('day',e.ts))>1) t`,
		f.From, f.To, ns(f.OwnerID), ns(f.GameID)).Scan(&m.ReturningPlayers)
	return m, err
}

func (r *Repository) Daily(ctx context.Context, f domain.Filter) ([]domain.DayPoint, error) {
	rows, err := r.pool.Query(ctx, `
	  WITH days AS (
	    SELECT generate_series(date_trunc('day',$1::timestamptz), date_trunc('day',$2::timestamptz - interval '1 microsecond'), interval '1 day') AS d),
	  agg AS (
	    SELECT date_trunc('day',e.ts) AS d,
	      count(DISTINCT e.session_id) FILTER (WHERE e.type='session_start') AS sessions,
	      count(*) FILTER (WHERE e.type='game_start') AS starts,
	      count(*) FILTER (WHERE e.type='game_end' AND e.result='win') AS wins,
	      count(*) FILTER (WHERE e.type='game_end' AND e.result='lose') AS losses
	    FROM events e JOIN games g ON g.id=e.game_id WHERE `+scope+` GROUP BY 1)
	  SELECT to_char(days.d,'YYYY-MM-DD'), coalesce(a.sessions,0), coalesce(a.starts,0), coalesce(a.wins,0), coalesce(a.losses,0)
	  FROM days LEFT JOIN agg a ON a.d=days.d ORDER BY days.d`,
		f.From, f.To, ns(f.OwnerID), ns(f.GameID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.DayPoint
	for rows.Next() {
		var p domain.DayPoint
		if err := rows.Scan(&p.Day, &p.Sessions, &p.Starts, &p.Wins, &p.Losses); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *Repository) PerGame(ctx context.Context, f domain.Filter) ([]domain.GameRow, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT g.id,g.title,g.genre,g.published,g.plays,`+aggCols+`
		 FROM games g LEFT JOIN events e ON e.game_id=g.id AND e.ts>=$1 AND e.ts<$2
		 WHERE g.version>0 AND ($3::uuid IS NULL OR g.owner_id=$3::uuid) AND (g.published OR $3::uuid IS NOT NULL)
		 GROUP BY g.id ORDER BY 6 DESC, g.created_at DESC LIMIT 200`,
		f.From, f.To, ns(f.OwnerID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.GameRow
	for rows.Next() {
		var row domain.GameRow
		m, err := scanAgg(rows, &row.ID, &row.Title, &row.Genre, &row.Published, &row.Plays)
		if err != nil {
			return nil, err
		}
		row.Summary = m
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *Repository) CustomEvents(ctx context.Context, f domain.Filter) ([]domain.EventCount, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT e.type, count(*), count(DISTINCT e.session_id)
		 FROM events e JOIN games g ON g.id=e.game_id
		 WHERE e.type NOT IN (`+reservedTypes+`) AND `+scope+`
		 GROUP BY e.type ORDER BY 2 DESC LIMIT 30`,
		f.From, f.To, ns(f.OwnerID), ns(f.GameID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.EventCount
	for rows.Next() {
		var c domain.EventCount
		if err := rows.Scan(&c.Type, &c.Count, &c.Sessions); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *Repository) DurationBuckets(ctx context.Context, f domain.Filter) ([]domain.Bucket, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT CASE WHEN e.duration_ms < 10000 THEN '0-10s'
		             WHEN e.duration_ms < 30000 THEN '10-30s'
		             WHEN e.duration_ms < 60000 THEN '30-60s'
		             WHEN e.duration_ms < 180000 THEN '1-3m'
		             WHEN e.duration_ms < 600000 THEN '3-10m'
		             ELSE '10m+' END AS b, count(*), min(e.duration_ms)
		 FROM events e JOIN games g ON g.id=e.game_id
		 WHERE e.type='session_end' AND e.duration_ms IS NOT NULL AND `+scope+`
		 GROUP BY 1 ORDER BY 3`,
		f.From, f.To, ns(f.OwnerID), ns(f.GameID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Bucket
	for rows.Next() {
		var b domain.Bucket
		var min int64
		if err := rows.Scan(&b.Label, &b.Count, &min); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (r *Repository) TopInsights(ctx context.Context, limit int) ([]domain.Insight, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT g.title,g.genre,g.audience,`+aggCols+`
		 FROM games g JOIN events e ON e.game_id=g.id
		 WHERE g.published AND e.ts > now() - interval '30 days'
		 GROUP BY g.id
		 HAVING count(DISTINCT e.session_id) FILTER (WHERE e.type='session_start') >= 3
		 ORDER BY 11 DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Insight
	for rows.Next() {
		var in domain.Insight
		m, err := scanAgg(rows, &in.Title, &in.Genre, &in.Audience)
		if err != nil {
			return nil, err
		}
		in.Sessions, in.AvgSessionSec, in.WinRate = m.Sessions, m.AvgSessionSec, m.WinRate
		out = append(out, in)
	}
	return out, rows.Err()
}
