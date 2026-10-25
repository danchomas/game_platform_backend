package service

import (
	"context"
	"strings"
	"sync"
	"time"

	"beeline-games/internal/core/apperr"
	"beeline-games/internal/core/domain"
	"beeline-games/internal/core/llm"
)

type fakeRepo struct {
	mu sync.Mutex

	games    map[string]domain.Game
	messages map[string][]domain.Message
	versions map[string]map[int]string

	nextID  int
	addErr  error
	getErr  error
	deleted []string

	lastQuery domain.GameQuery
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		games:    make(map[string]domain.Game),
		messages: make(map[string][]domain.Message),
		versions: make(map[string]map[int]string),
	}
}

func (f *fakeRepo) CreateGame(_ context.Context, ownerID, prompt string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.nextID++
	id := "game-" + string(rune('a'+f.nextID-1))
	f.games[id] = domain.Game{ID: id, OwnerID: ownerID, Prompt: prompt}
	return id, nil
}

func (f *fakeRepo) GetGame(_ context.Context, id string) (domain.Game, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.getErr != nil {
		return domain.Game{}, f.getErr
	}
	g, ok := f.games[id]
	if !ok {
		return domain.Game{}, apperr.ErrNotFound
	}
	return g, nil
}

func (f *fakeRepo) ListOwnerGames(_ context.Context, _ string, q domain.GameQuery) ([]domain.Game, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.lastQuery = q
	return nil, nil
}

func (f *fakeRepo) ListPublished(_ context.Context, q domain.GameQuery) ([]domain.Game, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.lastQuery = q
	return nil, nil
}

func (f *fakeRepo) AddVersion(_ context.Context, in domain.VersionInput) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.addErr != nil {
		return 0, f.addErr
	}

	g := f.games[in.GameID]
	g.ID = in.GameID
	g.Version++
	g.HTML = in.HTML
	if in.Title != "" {
		g.Title = in.Title
	}
	f.games[in.GameID] = g

	if f.versions[in.GameID] == nil {
		f.versions[in.GameID] = make(map[int]string)
	}
	f.versions[in.GameID][g.Version] = in.HTML

	if in.UserText != "" {
		f.messages[in.GameID] = append(f.messages[in.GameID], domain.Message{Role: "user", Content: in.UserText})
	}
	f.messages[in.GameID] = append(f.messages[in.GameID], domain.Message{Role: "assistant", Content: in.Reply})

	return g.Version, nil
}

func (f *fakeRepo) Messages(_ context.Context, gameID string) ([]domain.Message, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.messages[gameID], nil
}

func (f *fakeRepo) Versions(context.Context, string) ([]domain.VersionInfo, error) { return nil, nil }

func (f *fakeRepo) VersionHTML(_ context.Context, gameID string, v int) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	html, ok := f.versions[gameID][v]
	if !ok {
		return "", apperr.ErrNotFound
	}
	return html, nil
}

func (f *fakeRepo) SetPublished(_ context.Context, id string, published bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	g := f.games[id]
	g.Published = published
	f.games[id] = g
	return nil
}

func (f *fakeRepo) SetTitle(_ context.Context, id, title string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	g := f.games[id]
	g.Title = title
	f.games[id] = g
	return nil
}

func (f *fakeRepo) DeleteGame(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	delete(f.games, id)
	f.deleted = append(f.deleted, id)
	return nil
}

func (f *fakeRepo) IsPublished(_ context.Context, id string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.games[id].Published, nil
}

type scriptedLLM struct {
	mu       sync.Mutex
	replies  []string
	err      error
	calls    int
	lastMsgs []llm.Msg
}

func (s *scriptedLLM) Chat(ctx context.Context, msgs []llm.Msg) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.calls++
	s.lastMsgs = msgs

	if s.err != nil {
		return "", s.err
	}
	if len(s.replies) == 0 {
		return "", context.DeadlineExceeded
	}

	reply := s.replies[0]
	s.replies = s.replies[1:]
	return reply, nil
}

type blockingLLM struct {
	entered chan struct{}
	release chan struct{}
	reply   string
}

func (b *blockingLLM) Chat(ctx context.Context, _ []llm.Msg) (string, error) {
	b.entered <- struct{}{}
	select {
	case <-b.release:
		return b.reply, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

type staticInsights struct{ hint string }

func (s staticInsights) GetInsightsHint(context.Context) string { return s.hint }

func goodReply(title, marker string) string {
	html := "<!doctype html><html><head></head><body><canvas></canvas><script>" +
		marker + strings.Repeat("// наполнение до минимального размера\n", 12) +
		"</script></body></html>"

	return "<reply>готово</reply>" +
		`<meta>{"Title":"` + title + `","Genre":"аркада","Audience":"все"}</meta>` +
		"<game>" + html + "</game>"
}

const testTimeout = 2 * time.Second
