package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"beeline-games/internal/core/apperr"
	"beeline-games/internal/core/collector"
	"beeline-games/internal/core/domain"
)

type fakeGames struct {
	game     domain.Game
	getErr   error
	pubErr   error
	pubCalls int
}

func (f *fakeGames) GetGame(context.Context, string, string) (domain.Game, error) {
	if f.getErr != nil {
		return domain.Game{}, f.getErr
	}
	return f.game, nil
}

func (f *fakeGames) IsPublished(context.Context, string) (bool, error) {
	f.pubCalls++
	if f.pubErr != nil {
		return false, f.pubErr
	}
	return f.game.Published, nil
}

type recordingCollector struct {
	mu     sync.Mutex
	events []collector.Event
}

func (c *recordingCollector) Add(e collector.Event) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, e)
}

func (c *recordingCollector) all() []collector.Event {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]collector.Event(nil), c.events...)
}

const gameHTML = `<!doctype html><html><head><meta charset="utf-8"></head><body><canvas></canvas></body></html>`

// fakeRouter отдаёт заданный вариант и запоминает, для кого его выбирали.
type fakeRouter struct {
	gameID string
	err    error
	player string
}

func (r *fakeRouter) PickVariant(_ context.Context, _, playerID string) (string, error) {
	r.player = playerID
	return r.gameID, r.err
}

func newTestService(games GamesProvider, col CollectorProvider) *Service {
	return NewService(games, &fakeRouter{err: apperr.ErrNotFound}, col, "http://localhost:8080")
}

func TestGetPlayableHTMLInjectsSDKBeforeGameCode(t *testing.T) {
	games := &fakeGames{game: domain.Game{
		ID: "g1", OwnerID: "owner-1", Published: true, HTML: gameHTML,
	}}
	svc := newTestService(games, &recordingCollector{})

	html, published, err := svc.GetPlayableHTML(context.Background(), "", "g1")
	if err != nil {
		t.Fatalf("игра не отдана: %v", err)
	}
	if !published {
		t.Error("признак публикации потерян")
	}

	sdk := strings.Index(html, "<script>")
	canvas := strings.Index(html, "<canvas")
	if sdk < 0 || canvas < 0 || sdk > canvas {
		t.Errorf("SDK вставлен после кода игры: sdk=%d canvas=%d", sdk, canvas)
	}
	if strings.Contains(html, "__") {
		t.Error("подстановки в SDK не раскрыты")
	}
}

func TestGetPlayableHTMLWithoutHead(t *testing.T) {
	games := &fakeGames{game: domain.Game{
		ID: "g1", Published: true, HTML: `<html><body><canvas></canvas></body></html>`,
	}}
	svc := newTestService(games, &recordingCollector{})

	html, _, err := svc.GetPlayableHTML(context.Background(), "", "g1")
	if err != nil {
		t.Fatalf("игра не отдана: %v", err)
	}
	if strings.Index(html, "<script>") > strings.Index(html, "<canvas") {
		t.Error("без <head> SDK вставлен после кода игры")
	}
}

func TestGetPlayableHTMLHidesForeignDraft(t *testing.T) {
	games := &fakeGames{game: domain.Game{
		ID: "g1", OwnerID: "owner-1", Published: false, HTML: gameHTML,
	}}
	svc := newTestService(games, &recordingCollector{})

	if _, _, err := svc.GetPlayableHTML(context.Background(), "owner-2", "g1"); !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("чужой черновик отдан: %v", err)
	}
	if _, _, err := svc.GetPlayableHTML(context.Background(), "owner-1", "g1"); err != nil {
		t.Fatalf("владелец не получил свой черновик: %v", err)
	}
}

func TestProcessEventsValidation(t *testing.T) {
	valid := Event{Type: "level_up"}

	tests := []struct {
		name  string
		batch EventBatch
		field string
	}{
		{"без игры", EventBatch{SessionID: "s1", Events: []Event{valid}}, "game_id"},
		{"без сессии", EventBatch{GameID: "g1", Events: []Event{valid}}, "session_id"},
		{"длинная сессия", EventBatch{GameID: "g1", SessionID: strings.Repeat("x", 65), Events: []Event{valid}}, "session_id"},
		{"пустая пачка", EventBatch{GameID: "g1", SessionID: "s1"}, "events"},
		{"переполненная пачка", EventBatch{GameID: "g1", SessionID: "s1", Events: make([]Event, 51)}, "events"},
	}

	svc := newTestService(&fakeGames{}, &recordingCollector{})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.ProcessEvents(context.Background(), tt.batch)
			if !errors.Is(err, apperr.ErrValidation) {
				t.Fatalf("ожидалась ошибка валидации, получено: %v", err)
			}
			var vErr *apperr.ValidationError
			if !errors.As(err, &vErr) || vErr.Field != tt.field {
				t.Errorf("ошибка указывает не на то поле: %v", err)
			}
		})
	}
}

func TestProcessEventsDropsUnpublishedGame(t *testing.T) {
	col := &recordingCollector{}
	svc := newTestService(&fakeGames{game: domain.Game{Published: false}}, col)

	err := svc.ProcessEvents(context.Background(), EventBatch{
		GameID: "g1", SessionID: "s1", Events: []Event{{Type: "session_start"}},
	})
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if n := len(col.all()); n != 0 {
		t.Errorf("принято %d событий черновика, ожидалось 0", n)
	}
}

func TestProcessEventsSkipsBadTypeKeepsRest(t *testing.T) {
	col := &recordingCollector{}
	svc := newTestService(&fakeGames{game: domain.Game{Published: true}}, col)

	err := svc.ProcessEvents(context.Background(), EventBatch{
		GameID:    "g1",
		SessionID: "s1",
		PlayerID:  strings.Repeat("p", 100),
		Events: []Event{
			{Type: "session_start"},
			{Type: "DROP TABLE events"},
			{Type: "слишком_по-русски"},
			{Type: "game_end", Result: "win"},
			{Type: "game_end", Result: "сломанный"},
		},
	})
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}

	got := col.all()
	if len(got) != 3 {
		t.Fatalf("принято %d событий, ожидалось 3", len(got))
	}
	if len(got[0].PlayerID) != maxPlayerIDLen {
		t.Errorf("идентификатор игрока не подрезан: %d символов", len(got[0].PlayerID))
	}
	if got[1].Result != "win" {
		t.Errorf("исход = %q, ожидался win", got[1].Result)
	}
	if got[2].Result != "" {
		t.Errorf("неизвестный исход %q не отброшен", got[2].Result)
	}
}

func TestProcessEventsDropsOversizedPayload(t *testing.T) {
	col := &recordingCollector{}
	svc := newTestService(&fakeGames{game: domain.Game{Published: true}}, col)

	big := []byte(`{"a":"` + strings.Repeat("x", maxPayloadBytes) + `"}`)
	err := svc.ProcessEvents(context.Background(), EventBatch{
		GameID: "g1", SessionID: "s1",
		Events: []Event{
			{Type: "custom", Payload: big},
			{Type: "custom", Payload: []byte("null")},
			{Type: "custom", Payload: []byte(`{"score":1}`)},
		},
	})
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}

	got := col.all()
	if len(got) != 3 {
		t.Fatalf("принято %d событий, ожидалось 3", len(got))
	}
	if got[0].Payload != nil || got[1].Payload != nil {
		t.Error("негодная полезная нагрузка не отброшена")
	}
	if string(got[2].Payload) != `{"score":1}` {
		t.Errorf("годная нагрузка потеряна: %q", got[2].Payload)
	}
}

func TestProcessEventsRejectsClientExposure(t *testing.T) {
	col := &recordingCollector{}
	svc := newTestService(&fakeGames{game: domain.Game{Published: true}}, col)

	err := svc.ProcessEvents(context.Background(), EventBatch{
		GameID: "g1", SessionID: "s1",
		Events: []Event{{Type: eventExposure}, {Type: "session_start"}},
	})
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if got := col.all(); len(got) != 1 || got[0].Type != "session_start" {
		t.Errorf("показ от клиента принят: %+v", got)
	}
}

func TestPlayExperimentRecordsExposureForServedVariant(t *testing.T) {
	games := &fakeGames{game: domain.Game{ID: "variant-b", Published: true, HTML: gameHTML}}
	router := &fakeRouter{gameID: "variant-b"}
	col := &recordingCollector{}
	svc := NewService(games, router, col, "http://localhost:8080")

	html, playerID, err := svc.PlayExperiment(context.Background(), "exp-1", "player_42")
	if err != nil {
		t.Fatalf("вариант не отдан: %v", err)
	}
	if playerID != "player_42" || router.player != "player_42" {
		t.Errorf("игрок подменён: вернулся %q, в маршрутизацию ушёл %q", playerID, router.player)
	}

	got := col.all()
	if len(got) != 1 || got[0].Type != eventExposure || got[0].GameID != "variant-b" || got[0].PlayerID != "player_42" {
		t.Fatalf("показ не записан: %+v", got)
	}
	// показ и сессия SDK должны совпадать, иначе воронку не собрать
	for _, want := range []string{`pid: "player_42"`, `sid: "` + got[0].SessionID + `"`, `game: "variant-b"`, "on: true"} {
		if !strings.Contains(html, want) {
			t.Errorf("в SDK нет %s", want)
		}
	}
	if !strings.Contains(string(got[0].Payload), "exp-1") {
		t.Errorf("показ не помечен экспериментом: %s", got[0].Payload)
	}
}

func TestPlayExperimentReplacesUnsafePlayerID(t *testing.T) {
	tests := []struct {
		name   string
		player string
	}{
		{name: "empty", player: ""},
		{name: "script injection", player: `x"};alert(1);//`},
		{name: "closing tag", player: "</script><script>alert(1)"},
		{name: "too long", player: strings.Repeat("p", 65)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			games := &fakeGames{game: domain.Game{ID: "v", Published: true, HTML: gameHTML}}
			svc := NewService(games, &fakeRouter{gameID: "v"}, &recordingCollector{}, "http://localhost:8080")

			html, playerID, err := svc.PlayExperiment(context.Background(), "exp-1", tt.player)
			if err != nil {
				t.Fatalf("вариант не отдан: %v", err)
			}
			if !playerIDRe.MatchString(playerID) || playerID == tt.player {
				t.Errorf("непригодный идентификатор не заменён: %q", playerID)
			}
			if strings.Contains(html, "alert(1)") {
				t.Error("значение из запроса попало в SDK")
			}
		})
	}
}

func TestPlayExperimentErrors(t *testing.T) {
	tests := []struct {
		name   string
		game   domain.Game
		router *fakeRouter
	}{
		{name: "experiment not running", router: &fakeRouter{err: apperr.ErrNotFound}},
		{name: "variant unpublished", game: domain.Game{ID: "v", HTML: gameHTML}, router: &fakeRouter{gameID: "v"}},
		{name: "variant without code", game: domain.Game{ID: "v", Published: true}, router: &fakeRouter{gameID: "v"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			col := &recordingCollector{}
			svc := NewService(&fakeGames{game: tt.game}, tt.router, col, "http://localhost:8080")

			if _, _, err := svc.PlayExperiment(context.Background(), "exp-1", "p1"); !errors.Is(err, apperr.ErrNotFound) {
				t.Fatalf("ожидалось ErrNotFound, получено: %v", err)
			}
			if n := len(col.all()); n != 0 {
				t.Errorf("при ошибке записано %d показов", n)
			}
		})
	}
}

func TestIsPublishedCachesLookup(t *testing.T) {
	games := &fakeGames{game: domain.Game{Published: true}}
	col := &recordingCollector{}
	svc := newTestService(games, col)

	batch := EventBatch{GameID: "g1", SessionID: "s1", Events: []Event{{Type: "ping"}}}
	for range 5 {
		if err := svc.ProcessEvents(context.Background(), batch); err != nil {
			t.Fatalf("неожиданная ошибка: %v", err)
		}
	}

	if games.pubCalls != 1 {
		t.Errorf("база опрошена %d раз, ожидался 1", games.pubCalls)
	}
	if n := len(col.all()); n != 5 {
		t.Errorf("принято %d событий, ожидалось 5", n)
	}
}
