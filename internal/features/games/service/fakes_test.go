package service

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"sync"
	"time"

	"beeline-games/internal/core/agent"
	"beeline-games/internal/core/apperr"
	"beeline-games/internal/core/domain"
	"beeline-games/internal/core/llm"
)

type fakeRepo struct {
	mu sync.Mutex

	games    map[string]domain.Game
	messages map[string][]domain.Message
	versions map[string]map[int]string
	gens     map[int64]domain.GenerationResult
	genKinds map[int64]string

	nextID  int
	nextGen int64
	deleted []string

	lastQuery domain.GameQuery
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		games:    make(map[string]domain.Game),
		messages: make(map[string][]domain.Message),
		versions: make(map[string]map[int]string),
		gens:     make(map[int64]domain.GenerationResult),
		genKinds: make(map[int64]string),
	}
}

func (f *fakeRepo) CreateGame(_ context.Context, ownerID, prompt string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.nextID++
	id := "game-" + string(rune('a'+f.nextID-1))
	f.games[id] = domain.Game{ID: id, OwnerID: ownerID, Prompt: prompt, Status: domain.StatusPlanning}
	return id, nil
}

func (f *fakeRepo) CreateVariant(ctx context.Context, in domain.VariantInput) (string, error) {
	id, _ := f.CreateGame(ctx, in.OwnerID, in.Prompt)

	f.mu.Lock()
	defer f.mu.Unlock()
	g := f.games[id]
	g.ExperimentID, g.VariantLabel = in.ExperimentID, in.Label
	f.games[id] = g
	return id, nil
}

// seedGame — готовая собранная игра для тестов правок и отката.
func (f *fakeRepo) seedGame(ownerID, html string) string {
	id, _ := f.CreateGame(context.Background(), ownerID, "готовая игра")
	f.mu.Lock()
	g := f.games[id]
	g.Status = domain.StatusReady
	f.games[id] = g
	f.mu.Unlock()
	_, _ = f.AddVersion(context.Background(), domain.VersionInput{GameID: id, Reply: "готово", HTML: html})
	return id
}

func (f *fakeRepo) game(id string) domain.Game {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.games[id]
}

func (f *fakeRepo) GetGame(_ context.Context, id string) (domain.Game, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

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

	g, ok := f.games[in.GameID]
	if !ok {
		return 0, apperr.ErrNotFound
	}
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
	return slices.Clone(f.messages[gameID]), nil
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

func (f *fakeRepo) SetStatus(_ context.Context, id, status, detail string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	g := f.games[id]
	g.Status, g.StatusDetail = status, detail
	f.games[id] = g
	return nil
}

func (f *fakeRepo) TrySetStatus(_ context.Context, id, status string, from []string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	g, ok := f.games[id]
	if !ok || !slices.Contains(from, g.Status) {
		return false, nil
	}
	g.Status, g.StatusDetail = status, ""
	f.games[id] = g
	return true, nil
}

func (f *fakeRepo) SetSpec(_ context.Context, id string, spec domain.GameSpec) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	g := f.games[id]
	g.Spec, _ = json.Marshal(spec)
	if spec.Title != "" {
		g.Title = spec.Title
	}
	f.games[id] = g
	return nil
}

func (f *fakeRepo) AddMessage(_ context.Context, gameID, role, content string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.messages[gameID] = append(f.messages[gameID], domain.Message{Role: role, Content: content})
	return nil
}

func (f *fakeRepo) ResetInterrupted(context.Context) (int64, error) { return 0, nil }

func (f *fakeRepo) StartGeneration(_ context.Context, _ string, kind string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.nextGen++
	f.genKinds[f.nextGen] = kind
	return f.nextGen, nil
}

func (f *fakeRepo) FinishGeneration(_ context.Context, g domain.GenerationResult) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.gens[g.ID] = g
	return nil
}

// lastGen — итог последнего запуска агента указанного вида.
func (f *fakeRepo) lastGen(kind string) (domain.GenerationResult, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	for id := f.nextGen; id > 0; id-- {
		if f.genKinds[id] == kind {
			g, ok := f.gens[id]
			return g, ok
		}
	}
	return domain.GenerationResult{}, false
}

func (f *fakeRepo) ListGenerations(context.Context, string, int) ([]domain.Generation, error) {
	return nil, nil
}

func (f *fakeRepo) LatestScreenshot(context.Context, string) ([]byte, error) {
	return nil, apperr.ErrNotFound
}

func (f *fakeRepo) GenerationStats(context.Context, time.Time) (domain.GenerationStats, error) {
	return domain.GenerationStats{}, nil
}

func (f *fakeRepo) BuildEstimate(context.Context, string) (domain.BuildEstimate, error) {
	return domain.BuildEstimate{Samples: 5, DurationMs: 90_000, CoderChars: 20_000}, nil
}

// fakeAgent — сценарий агента без LLM: план и сборка из заданных значений.
type fakeAgent struct {
	mu sync.Mutex

	spec     domain.GameSpec
	planErr  error
	html     string
	issues   []domain.Issue
	buildErr error

	// block != nil — Build ждёт закрытия канала (проверка конкурентных запросов)
	block   chan struct{}
	entered chan struct{}

	lastPlan  agent.PlanInput
	lastBuild agent.BuildInput
	builds    int
}

func (a *fakeAgent) Plan(_ context.Context, in agent.PlanInput, tr *agent.Trace) (domain.GameSpec, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.lastPlan = in
	tr.Calls = append(tr.Calls, domain.LLMCall{Role: "planner", CostUSD: 0.001})
	return a.spec, a.planErr
}

func (a *fakeAgent) Build(ctx context.Context, in agent.BuildInput, stage agent.Stage, tr *agent.Trace) (agent.BuildOutput, error) {
	a.mu.Lock()
	a.lastBuild = in
	a.builds++
	block, entered := a.block, a.entered
	a.mu.Unlock()

	stage(domain.StatusGenerating, "")
	if in.Progress != nil {
		in.Progress("coder", 1200, in.ExpectedChars, "Рисуем поле")
	}
	if entered != nil {
		entered <- struct{}{}
	}
	if block != nil {
		select {
		case <-block:
		case <-ctx.Done():
			return agent.BuildOutput{}, ctx.Err()
		}
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	tr.Calls = append(tr.Calls, domain.LLMCall{Role: "coder", CostUSD: 0.01})
	tr.Issues = a.issues
	if a.buildErr != nil {
		return agent.BuildOutput{}, a.buildErr
	}
	return agent.BuildOutput{
		HTML:   a.html,
		Result: llm.Result{Reply: "Готово", Title: "Змейка", Genre: "snake"},
		Issues: a.issues,
	}, nil
}

type staticInsights struct{ hint string }

func (s staticInsights) GetInsightsHint(context.Context) string { return s.hint }

func gameHTML(marker string) string {
	return "<!doctype html><html><head></head><body><canvas></canvas><script>" +
		marker + strings.Repeat("// наполнение до минимального размера\n", 12) +
		"</script></body></html>"
}

func snakeSpec() domain.GameSpec {
	return domain.GameSpec{Title: "Змейка", Genre: "snake", Summary: "• Змейка для бабушки"}
}
