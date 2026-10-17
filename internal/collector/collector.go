// Package collector — буферизованный приём событий аналитики.
// HTTP-обработчик только кладёт событие в канал (O(1)), а пачки пишутся в Postgres через COPY.
package collector

import (
	"context"
	"database/sql"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/lib/pq"
)

type Event struct {
	GameID, SessionID, PlayerID, Type, Result string
	Score                                     *float64
	DurationMs                                *int64
	Payload                                   []byte
}

type Collector struct {
	db      *sql.DB
	ch      chan Event
	Dropped atomic.Int64
}

func New(db *sql.DB, buffer int) *Collector {
	return &Collector{db: db, ch: make(chan Event, buffer)}
}

// Add не блокируется: при переполнении события отбрасываются (и считаются).
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

// Run работает до отмены ctx, затем дописывает остаток.
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
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, pq.CopyIn("events",
		"game_id", "session_id", "player_id", "type", "result", "score", "duration_ms", "payload"))
	if err != nil {
		return err
	}
	plays := map[string]int64{}
	for _, e := range batch {
		var payload any
		if len(e.Payload) > 0 {
			payload = string(e.Payload)
		}
		if _, err := stmt.ExecContext(ctx, e.GameID, e.SessionID, e.PlayerID, e.Type, e.Result,
			e.Score, e.DurationMs, payload); err != nil {
			return err
		}
		if e.Type == "session_start" {
			plays[e.GameID]++
		}
	}
	if _, err := stmt.ExecContext(ctx); err != nil {
		return err
	}
	if err := stmt.Close(); err != nil {
		return err
	}
	if len(plays) > 0 {
		ids := make([]string, 0, len(plays))
		cnt := make([]int64, 0, len(plays))
		for id, n := range plays {
			ids, cnt = append(ids, id), append(cnt, n)
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE games g SET plays = g.plays + d.c
			 FROM (SELECT unnest($1::uuid[]) AS id, unnest($2::bigint[]) AS c) d WHERE g.id = d.id`,
			pq.Array(ids), pq.Array(cnt)); err != nil {
			return err
		}
	}
	return tx.Commit()
}
