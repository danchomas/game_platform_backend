package store

import (
	"context"
	"database/sql"
	"time"
)

// Зарезервированные типы событий, которые шлёт SDK автоматически.
const reservedTypes = `'session_start','engaged','game_start','game_end','session_end'`

type Filter struct {
	OwnerID string // "" = без фильтра по владельцу
	GameID  string // "" = все игры
	From    time.Time
	To      time.Time
}

func ns(s string) sql.NullString { return sql.NullString{String: s, Valid: s != ""} }

type Summary struct {
	Sessions         int64   `json:"sessions"`
	Players          int64   `json:"players"`
	ReturningPlayers int64   `json:"returning_players"`
	Engaged          int64   `json:"engaged_sessions"`
	Starts           int64   `json:"game_starts"`
	Ends             int64   `json:"game_ends"`
	Wins             int64   `json:"wins"`
	Losses           int64   `json:"losses"`
	AvgSessionSec    float64 `json:"avg_session_sec"`
	AvgScore         float64 `json:"avg_score"`
	MaxScore         float64 `json:"max_score"`
	EngagementRate   float64 `json:"engagement_rate"` // доля сессий с реальным взаимодействием
	CompletionRate   float64 `json:"completion_rate"` // доля начатых партий, дошедших до конца
	WinRate          float64 `json:"win_rate"`
}

func (s *Summary) derive() {
	if s.Sessions > 0 {
		s.EngagementRate = float64(s.Engaged) / float64(s.Sessions)
	}
	if s.Starts > 0 {
		s.CompletionRate = float64(s.Ends) / float64(s.Starts)
	}
	if s.Ends > 0 {
		s.WinRate = float64(s.Wins) / float64(s.Ends)
	}
}

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

func scanAgg(sc interface{ Scan(...any) error }, pre ...any) (Summary, error) {
	var m Summary
	dest := append(pre, &m.Sessions, &m.Players, &m.Engaged, &m.Starts, &m.Ends, &m.Wins, &m.Losses,
		&m.AvgSessionSec, &m.AvgScore, &m.MaxScore)
	err := sc.Scan(dest...)
	m.derive()
	return m, err
}

const scope = `e.ts >= $1 AND e.ts < $2 AND ($3::uuid IS NULL OR g.owner_id = $3::uuid) AND ($4::uuid IS NULL OR e.game_id = $4::uuid)`

func (s *Store) Summary(ctx context.Context, f Filter) (Summary, error) {
	m, err := scanAgg(s.DB.QueryRowContext(ctx,
		`SELECT `+aggCols+` FROM events e JOIN games g ON g.id=e.game_id WHERE `+scope,
		f.From, f.To, ns(f.OwnerID), ns(f.GameID)))
	if err != nil {
		return m, err
	}
	err = s.DB.QueryRowContext(ctx,
		`SELECT count(*) FROM (
		   SELECT e.player_id FROM events e JOIN games g ON g.id=e.game_id
		   WHERE e.type='session_start' AND e.player_id<>'' AND `+scope+`
		   GROUP BY e.player_id HAVING count(DISTINCT date_trunc('day',e.ts))>1) t`,
		f.From, f.To, ns(f.OwnerID), ns(f.GameID)).Scan(&m.ReturningPlayers)
	return m, err
}

type DayPoint struct {
	Day      string `json:"day"`
	Sessions int64  `json:"sessions"`
	Starts   int64  `json:"game_starts"`
	Wins     int64  `json:"wins"`
	Losses   int64  `json:"losses"`
}

func (s *Store) Daily(ctx context.Context, f Filter) ([]DayPoint, error) {
	rows, err := s.DB.QueryContext(ctx, `
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
	out := []DayPoint{}
	for rows.Next() {
		var p DayPoint
		if err := rows.Scan(&p.Day, &p.Sessions, &p.Starts, &p.Wins, &p.Losses); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

type GameRow struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Genre     string `json:"genre"`
	Published bool   `json:"published"`
	Plays     int64  `json:"plays_total"`
	Summary
}

// PerGame — таблица «игры × метрики» для дашборда.
func (s *Store) PerGame(ctx context.Context, f Filter) ([]GameRow, error) {
	rows, err := s.DB.QueryContext(ctx,
		`SELECT g.id,g.title,g.genre,g.published,g.plays,`+aggCols+`
		 FROM games g LEFT JOIN events e ON e.game_id=g.id AND e.ts>=$1 AND e.ts<$2
		 WHERE g.version>0 AND ($3::uuid IS NULL OR g.owner_id=$3::uuid) AND (g.published OR $3::uuid IS NOT NULL)
		 GROUP BY g.id ORDER BY 6 DESC, g.created_at DESC LIMIT 200`,
		f.From, f.To, ns(f.OwnerID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []GameRow{}
	for rows.Next() {
		var r GameRow
		m, err := scanAgg(rows, &r.ID, &r.Title, &r.Genre, &r.Published, &r.Plays)
		if err != nil {
			return nil, err
		}
		r.Summary = m
		out = append(out, r)
	}
	return out, rows.Err()
}

type EventCount struct {
	Type     string `json:"type"`
	Count    int64  `json:"count"`
	Sessions int64  `json:"sessions"`
}

// CustomEvents — кастомные события, которые игра шлёт через GameSDK.event(...)
func (s *Store) CustomEvents(ctx context.Context, f Filter) ([]EventCount, error) {
	rows, err := s.DB.QueryContext(ctx,
		`SELECT e.type, count(*), count(DISTINCT e.session_id)
		 FROM events e JOIN games g ON g.id=e.game_id
		 WHERE e.type NOT IN (`+reservedTypes+`) AND `+scope+`
		 GROUP BY e.type ORDER BY 2 DESC LIMIT 30`,
		f.From, f.To, ns(f.OwnerID), ns(f.GameID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []EventCount{}
	for rows.Next() {
		var c EventCount
		if err := rows.Scan(&c.Type, &c.Count, &c.Sessions); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// DurationBuckets — гистограмма длительности сессий (сек).
type Bucket struct {
	Label string `json:"label"`
	Count int64  `json:"count"`
}

func (s *Store) DurationBuckets(ctx context.Context, f Filter) ([]Bucket, error) {
	rows, err := s.DB.QueryContext(ctx,
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
	out := []Bucket{}
	for rows.Next() {
		var b Bucket
		var min int64
		if err := rows.Scan(&b.Label, &b.Count, &min); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// ---------- инсайты для промпта LLM ----------

type Insight struct {
	Title, Genre, Audience string
	Sessions               int64
	AvgSessionSec          float64
	WinRate                float64
}

// TopInsights — самые «залипательные» опубликованные игры за 30 дней.
func (s *Store) TopInsights(ctx context.Context, limit int) ([]Insight, error) {
	rows, err := s.DB.QueryContext(ctx,
		`SELECT g.title,g.genre,g.audience,`+aggCols+`
		 FROM games g JOIN events e ON e.game_id=g.id
		 WHERE g.published AND e.ts > now() - interval '30 days'
		 GROUP BY g.id
		 HAVING count(DISTINCT e.session_id) FILTER (WHERE e.type='session_start') >= 3
		 ORDER BY 11 DESC LIMIT $1`, limit) // 11-я колонка = avg_session_sec
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Insight{}
	for rows.Next() {
		var in Insight
		m, err := scanAgg(rows, &in.Title, &in.Genre, &in.Audience)
		if err != nil {
			return nil, err
		}
		in.Sessions, in.AvgSessionSec, in.WinRate = m.Sessions, m.AvgSessionSec, m.WinRate
		out = append(out, in)
	}
	return out, rows.Err()
}
