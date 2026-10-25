package collector

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Event struct {
	GameID     string
	SessionID  string
	PlayerID   string
	Type       string
	Result     string
	Score      *float64
	DurationMs *int64
	Payload    []byte
}

type Collector struct {
	pool    *pgxpool.Pool
	ch      chan Event
	Dropped atomic.Int64
}

func New(pool *pgxpool.Pool, buffer int) *Collector {
	return &Collector{pool: pool, ch: make(chan Event, buffer)}
}

func (c *Collector) Add(e Event) {
	select {
	case c.ch <- e:
	default:
		c.Dropped.Add(1)
	}
}

const (
	maxBatch = 1000
	interval = 500 * time.Millisecond
)

func (c *Collector) Run(ctx context.Context) {
	batch := make([]Event, 0, maxBatch)
	t := time.NewTicker(interval)
	defer t.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}
		fctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := c.write(fctx, batch); err != nil {
			slog.Error("collector flush", "err", err, "events", len(batch))
		}
		batch = batch[:0]
	}

	for {
		select {
		case e := <-c.ch:
			batch = append(batch, e)
			if len(batch) >= maxBatch {
				flush()
			}
		case <-t.C:
			flush()
		case <-ctx.Done():

			for {
				select {
				case e := <-c.ch:
					batch = append(batch, e)
				default:
					flush()
					return
				}
			}
		}
	}
}

func (c *Collector) write(ctx context.Context, batch []Event) error {
	tx, err := c.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	rows := make([][]any, 0, len(batch))
	plays := map[string]int64{}

	for _, e := range batch {
		var payload any
		if len(e.Payload) > 0 {
			payload = string(e.Payload)
		}
		rows = append(rows, []any{
			e.GameID, e.SessionID, e.PlayerID, e.Type, e.Result, e.Score, e.DurationMs, payload,
		})
		if e.Type == "session_start" {
			plays[e.GameID]++
		}
	}

	_, err = tx.CopyFrom(ctx,
		pgx.Identifier{"events"},
		[]string{"game_id", "session_id", "player_id", "type", "result", "score", "duration_ms", "payload"},
		pgx.CopyFromRows(rows),
	)
	if err != nil {
		return err
	}

	if len(plays) > 0 {
		ids := make([]string, 0, len(plays))
		cnt := make([]int64, 0, len(plays))
		for id, n := range plays {
			ids = append(ids, id)
			cnt = append(cnt, n)
		}
		_, err = tx.Exec(ctx,
			`UPDATE games g SET plays = g.plays + d.c
			 FROM (SELECT unnest($1::uuid[]) AS id, unnest($2::bigint[]) AS c) d WHERE g.id = d.id`,
			ids, cnt)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}
