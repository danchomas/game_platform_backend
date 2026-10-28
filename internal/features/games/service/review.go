package service

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"beeline-games/internal/core/agent"
	"beeline-games/internal/core/domain"
)

// Ревью идёт после сборки, когда игрок уже играет: модель ревьюера может думать минутами,
// поэтому у него свои слоты и он не занимает слоты сборок.
const maxParallelReviews = 2

// reviewStore — идущие фоновые проверки. Правка пользователя важнее: она отменяет проверку.
type reviewStore struct {
	mu   sync.Mutex
	runs map[string]*reviewRun
	sem  chan struct{}
}

type reviewRun struct {
	cancel context.CancelFunc
	state  domain.ReviewState
}

func newReviewStore() *reviewStore {
	return &reviewStore{runs: make(map[string]*reviewRun), sem: make(chan struct{}, maxParallelReviews)}
}

func (rs *reviewStore) add(gameID string, cancel context.CancelFunc) *reviewRun {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	if old := rs.runs[gameID]; old != nil {
		old.cancel()
	}
	run := &reviewRun{cancel: cancel, state: domain.ReviewState{StartedAt: time.Now(), Phase: domain.ReviewPhaseReviewing}}
	rs.runs[gameID] = run
	return run
}

func (rs *reviewStore) phase(gameID, phase string) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	if r := rs.runs[gameID]; r != nil {
		r.state.Phase = phase
	}
}

// done снимает проверку, только если это та же самая (новая могла её заменить).
func (rs *reviewStore) done(gameID string, run *reviewRun) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	if rs.runs[gameID] == run {
		delete(rs.runs, gameID)
	}
}

func (rs *reviewStore) stop(gameID string) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	if r := rs.runs[gameID]; r != nil {
		r.cancel()
		delete(rs.runs, gameID)
	}
}

func (rs *reviewStore) get(gameID string) *domain.ReviewState {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	if r := rs.runs[gameID]; r != nil {
		st := r.state
		return &st
	}
	return nil
}

// startReview ставит фоновую проверку только что собранной версии. Регистрируется синхронно,
// до статуса ready: первый же опрос /status после сборки должен её увидеть.
func (s *Service) startReview(gameID string, version int, spec *domain.GameSpec) {
	if spec == nil || !s.agent.ReviewEnabled() {
		return
	}
	ctx, cancel := context.WithCancel(s.bg)
	run := s.reviews.add(gameID, cancel)
	s.jobs.Add(1)
	go func() {
		defer s.jobs.Done()
		defer s.reviews.done(gameID, run)
		defer cancel()
		defer func() {
			if r := recover(); r != nil {
				slog.Error("фоновая проверка игры упала", "game", gameID, "panic", r)
			}
		}()
		s.runReview(ctx, gameID, version, *spec)
	}()
}

func (s *Service) runReview(ctx context.Context, gameID string, version int, spec domain.GameSpec) {
	select {
	case s.reviews.sem <- struct{}{}:
		defer func() { <-s.reviews.sem }()
	case <-ctx.Done():
		return
	}
	wctx := context.WithoutCancel(ctx)
	genID, err := s.repo.StartGeneration(ctx, gameID, domain.GenReview)
	if err != nil {
		slog.Error("проверка игры не начата", "game", gameID, "err", err)
		return
	}
	start, tr := time.Now(), &agent.Trace{}
	res := domain.GenerationResult{ID: genID}
	finish := func(status, errText string) {
		r := genResult(genID, start, tr)
		r.Status, r.Error, r.Version = status, errText, res.Version
		if len(r.Issues) == 0 {
			r.Issues = res.Issues
		}
		s.finishGen(wctx, r)
	}

	html, err := s.repo.VersionHTML(ctx, gameID, version)
	if err != nil {
		finish(domain.GenStatusFailed, err.Error())
		return
	}
	issues, err := s.agent.Review(ctx, spec, html, tr)
	res.Issues = issues
	if err != nil {
		finish(domain.GenStatusFailed, reviewError(ctx, err))
		return
	}
	if !domain.HasErrors(issues) {
		status := domain.GenStatusReady
		if len(issues) > 0 {
			status = domain.GenStatusWithIssues
		}
		finish(status, "")
		return
	}

	s.reviews.phase(gameID, domain.ReviewPhaseImproving)
	out, err := s.agent.Improve(ctx, html, issues, tr)
	if err != nil {
		finish(domain.GenStatusFailed, reviewError(ctx, err))
		return
	}
	saved, err := s.saveReviewed(ctx, gameID, version, out, issues)
	if err != nil {
		finish(domain.GenStatusFailed, reviewError(ctx, err))
		return
	}
	res.Version = &saved
	finish(domain.GenStatusReady, "")
}

var errSuperseded = errors.New("игру изменили, пока шла проверка: улучшение не применено")

// saveReviewed сохраняет улучшенную версию, только если игру за это время никто не трогал.
// Статус держится занятым лишь на время записи: правка пользователя ждёт миллисекунды, не минуты.
func (s *Service) saveReviewed(ctx context.Context, gameID string, base int, out agent.BuildOutput, issues []domain.Issue) (int, error) {
	if ctx.Err() != nil {
		return 0, ctx.Err()
	}
	wctx := context.WithoutCancel(ctx)
	ok, err := s.repo.TrySetStatus(wctx, gameID, domain.StatusGenerating, []string{domain.StatusReady})
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, errSuperseded
	}
	g, err := s.repo.GetGame(wctx, gameID)
	if err != nil || g.Version != base {
		s.setStatus(wctx, gameID, domain.StatusReady, "", 0)
		if err == nil {
			err = errSuperseded
		}
		return 0, err
	}
	version, err := s.repo.AddVersion(wctx, domain.VersionInput{
		GameID:      gameID,
		Reply:       reviewReply(issues),
		HTML:        out.HTML,
		Instruction: "автопроверка логики ревьюером",
	})
	if err != nil {
		s.setStatus(wctx, gameID, domain.StatusReady, "", 0)
		return 0, err
	}
	s.setStatus(wctx, gameID, domain.StatusReady, "", version)
	return version, nil
}

func reviewReply(issues []domain.Issue) string {
	var fixed []string
	for _, i := range issues {
		if i.Severity == domain.SevError {
			title, _, _ := strings.Cut(i.Message, ":")
			fixed = append(fixed, "• "+strings.TrimSpace(title))
		}
	}
	return "Ревьюер проверил логику игры и поправил:\n" + strings.Join(fixed, "\n")
}

func reviewError(ctx context.Context, err error) string {
	if ctx.Err() != nil {
		return "проверка отменена: игру начали править"
	}
	return err.Error()
}
