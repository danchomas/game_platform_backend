package service

import (
	"context"
	"slices"
	"sync"
	"time"

	"beeline-games/internal/core/apperr"
	"beeline-games/internal/core/domain"
)

const (
	expID   = "6f1c1d36-8a3c-4c39-9a57-6a3f3c0e8b11"
	ownerID = "owner-1"
)

type fakeRepo struct {
	mu sync.Mutex

	experiments map[string]domain.Experiment
	variants    map[string][]domain.Variant
	stats       []domain.VariantStats
	routing     domain.ExperimentRouting
	statsSince  time.Time
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		experiments: make(map[string]domain.Experiment),
		variants:    make(map[string][]domain.Variant),
	}
}

// seed — эксперимент владельца ownerID в статусе status с заданными вариантами.
func (f *fakeRepo) seed(status string, variants ...domain.Variant) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.experiments[expID] = domain.Experiment{ID: expID, OwnerID: ownerID, Status: status, CreatedAt: time.Unix(100, 0)}
	f.variants[expID] = variants
}

func (f *fakeRepo) Create(_ context.Context, ownerID, title, prompt string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.experiments[expID] = domain.Experiment{ID: expID, OwnerID: ownerID, Title: title, Prompt: prompt, Status: domain.ExperimentDraft}
	return expID, nil
}

func (f *fakeRepo) Get(_ context.Context, id string) (domain.Experiment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	x, ok := f.experiments[id]
	if !ok {
		return domain.Experiment{}, apperr.ErrNotFound
	}
	return x, nil
}

func (f *fakeRepo) ListOwner(context.Context, string, int) ([]domain.Experiment, error) {
	return nil, nil
}

func (f *fakeRepo) Variants(_ context.Context, id string) ([]domain.Variant, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.variants[id]), nil
}

func (f *fakeRepo) Launch(_ context.Context, id string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	x := f.experiments[id]
	if x.Status != domain.ExperimentDraft {
		return false, nil
	}
	now := time.Unix(200, 0)
	x.Status, x.LaunchedAt = domain.ExperimentRunning, &now
	f.experiments[id] = x
	return true, nil
}

func (f *fakeRepo) Finish(_ context.Context, id, winnerID string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	x := f.experiments[id]
	if x.Status == domain.ExperimentDraft {
		return false, nil
	}
	x.Status, x.WinnerID = domain.ExperimentFinished, winnerID
	f.experiments[id] = x
	return true, nil
}

func (f *fakeRepo) Routing(context.Context, string) (domain.ExperimentRouting, error) {
	return f.routing, nil
}

func (f *fakeRepo) VariantStats(_ context.Context, _ string, since time.Time) ([]domain.VariantStats, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.statsSince = since
	return f.stats, nil
}

func (f *fakeRepo) experiment() domain.Experiment {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.experiments[expID]
}

// fakeGames записывает запущенные варианты и опубликованные игры.
type fakeGames struct {
	mu sync.Mutex

	started   []domain.VariantInput
	published []string
	startErr  error
}

func (g *fakeGames) StartVariant(_ context.Context, in domain.VariantInput) (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.startErr != nil {
		return "", g.startErr
	}
	g.started = append(g.started, in)
	return "game-" + in.Label, nil
}

func (g *fakeGames) PublishGame(_ context.Context, _, gameID string, publish bool) (domain.Game, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if publish {
		g.published = append(g.published, gameID)
	}
	return domain.Game{ID: gameID, Published: publish}, nil
}

func ready(label string) domain.Variant {
	return domain.Variant{GameID: "game-" + label, Label: label, Status: domain.StatusReady,
		Version: 1, BuildStatus: domain.GenStatusReady}
}
