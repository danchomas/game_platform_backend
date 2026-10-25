package service

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"beeline-games/internal/core/domain"
)

// Пока мало истории, оценка сборки — по умолчанию.
const (
	minEstimateSamples = 3
	defaultBuildMs     = 150_000
)

// progressStore — ход идущих сборок для экрана ожидания. Только в памяти процесса,
// как и SSE-брокер: после перезапуска сборки всё равно сбрасываются.
type progressStore struct {
	mu   sync.Mutex
	runs map[string]*progressRun
}

type progressRun struct {
	status, detail string
	p              domain.BuildProgress
}

func newProgressStore() *progressStore {
	return &progressStore{runs: make(map[string]*progressRun)}
}

func (ps *progressStore) start(gameID string, expectedMs int64) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.runs[gameID] = &progressRun{
		status: domain.StatusGenerating,
		p:      domain.BuildProgress{StartedAt: time.Now(), ExpectedMs: expectedMs},
	}
}

func (ps *progressStore) finish(gameID string) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	delete(ps.runs, gameID)
}

// stage — смена шага: проверки и тест идут без модели, счёт символов сбрасывается.
func (ps *progressStore) stage(gameID, status, detail string) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	if r := ps.runs[gameID]; r != nil {
		r.status, r.detail = status, detail
		if status != domain.StatusGenerating && status != domain.StatusFixing {
			r.p.Role, r.p.Chars, r.p.ExpectedChars, r.p.Step = "", 0, 0, ""
		}
	}
}

// update — новый кусок кода от модели. Возвращает текущий шаг и снимок прогресса.
func (ps *progressStore) update(gameID, role string, chars, expected int, step string) (domain.StatusEvent, bool) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	r := ps.runs[gameID]
	if r == nil {
		return domain.StatusEvent{}, false
	}
	// скорость считаем с первого символа ответа: время на обдумывание в неё не входит
	if r.p.Role != role || chars < r.p.Chars || (r.p.Chars == 0 && chars > 0) {
		r.p.PhaseStartedAt = time.Now()
	}
	r.p.Role, r.p.Chars, r.p.ExpectedChars, r.p.Step = role, chars, expected, step
	p := r.p
	return domain.StatusEvent{GameID: gameID, Status: r.status, Detail: r.detail, At: time.Now(), Progress: &p}, true
}

func (ps *progressStore) get(gameID string) *domain.BuildProgress {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	if r := ps.runs[gameID]; r != nil {
		p := r.p
		return &p
	}
	return nil
}

// estimate — обычная длительность и размер кода такой сборки; без истории — значения по умолчанию.
func (s *Service) estimate(ctx context.Context, kind string) domain.BuildEstimate {
	est, err := s.repo.BuildEstimate(ctx, kind)
	if err != nil {
		slog.Warn("оценка сборки по истории недоступна", "kind", kind, "err", err)
	}
	if err != nil || est.Samples < minEstimateSamples {
		return domain.BuildEstimate{DurationMs: defaultBuildMs}
	}
	return est
}

// progressFn — следилка агента: обновляет прогресс и рассылает его подписчикам SSE.
func (s *Service) progressFn(gameID string) func(role string, chars, expected int, step string) {
	return func(role string, chars, expected int, step string) {
		if ev, ok := s.progress.update(gameID, role, chars, expected, step); ok {
			s.events.publish(ev)
		}
	}
}
