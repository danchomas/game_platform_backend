package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"beeline-games/internal/core/agent"
	"beeline-games/internal/core/apperr"
	"beeline-games/internal/core/domain"
	"beeline-games/internal/core/llm"
)

const roleManager = "manager"

// CreateGame: промпт → план (синхронно, дешёвая модель). autoApprove сразу запускает сборку.
func (s *Service) CreateGame(ctx context.Context, ownerID, prompt string, autoApprove bool) (domain.Game, domain.Plan, error) {
	prompt, err := validatePrompt(prompt)
	if err != nil {
		return domain.Game{}, domain.Plan{}, err
	}
	id, err := s.repo.CreateGame(ctx, ownerID, prompt)
	if err != nil {
		return domain.Game{}, domain.Plan{}, err
	}

	g := domain.Game{ID: id, OwnerID: ownerID, Prompt: prompt}
	spec, err := s.plan(ctx, g, domain.GenPlan, agent.PlanInput{}, waitForSlot)
	wctx := context.WithoutCancel(ctx)
	if err != nil {
		// черновик остаётся со статусом failed: план можно перепросить, а попытка видна в статистике
		s.setStatus(wctx, id, domain.StatusFailed, "не удалось составить план: "+err.Error(), 0)
		return domain.Game{}, domain.Plan{}, err
	}
	if err := s.savePlan(wctx, id, prompt, spec); err != nil {
		return domain.Game{}, domain.Plan{}, err
	}
	s.setStatus(wctx, id, domain.StatusAwaitingApproval, "", 0)

	if autoApprove {
		ok, err := s.repo.TrySetStatus(wctx, id, domain.StatusGenerating, []string{domain.StatusAwaitingApproval})
		if err != nil {
			return domain.Game{}, domain.Plan{}, err
		}
		if ok {
			s.launch(id, domain.GenBuild, "")
		}
	}

	g, err = s.repo.GetGame(wctx, id)
	return g, domain.Plan{Spec: spec, Reply: spec.Summary}, err
}

// RevisePlan правит план по отзыву. Доступно только до первой сборки.
func (s *Service) RevisePlan(ctx context.Context, userID, gameID, feedback string) (domain.Game, domain.Plan, error) {
	feedback, err := validatePrompt(feedback)
	if err != nil {
		return domain.Game{}, domain.Plan{}, err
	}
	g, err := s.getOwnedGame(ctx, userID, gameID)
	if err != nil {
		return domain.Game{}, domain.Plan{}, err
	}
	if !g.PlanStage() {
		return domain.Game{}, domain.Plan{}, planStateErr(g)
	}

	ok, err := s.repo.TrySetStatus(ctx, gameID, domain.StatusPlanning,
		[]string{domain.StatusAwaitingApproval, domain.StatusFailed})
	if err != nil {
		return domain.Game{}, domain.Plan{}, err
	}
	if !ok {
		return domain.Game{}, domain.Plan{}, apperr.ErrConflict
	}
	s.publish(gameID, domain.StatusPlanning, "", 0)

	kind := domain.GenRevise
	in := agent.PlanInput{Current: specOf(g), Feedback: feedback, History: s.history(ctx, gameID)}
	if in.Current == nil {
		// прошлый план не получился — строим заново с учётом уточнения
		kind, in = domain.GenPlan, agent.PlanInput{}
		g.Prompt += "\n\nУточнение: " + feedback
	}
	spec, err := s.plan(ctx, g, kind, in, waitForSlot)
	wctx := context.WithoutCancel(ctx)
	if err != nil {
		s.setStatus(wctx, gameID, g.Status, "не удалось обновить план: "+err.Error(), 0)
		return domain.Game{}, domain.Plan{}, err
	}
	if err := s.savePlan(wctx, gameID, feedback, spec); err != nil {
		return domain.Game{}, domain.Plan{}, err
	}
	s.setStatus(wctx, gameID, domain.StatusAwaitingApproval, "", 0)

	g, err = s.repo.GetGame(wctx, gameID)
	return g, domain.Plan{Spec: spec, Reply: spec.Summary}, err
}

// ApprovePlan запускает фоновую сборку. spec != nil — план, отредактированный в клиенте.
func (s *Service) ApprovePlan(ctx context.Context, userID, gameID string, spec *domain.GameSpec) (domain.Game, error) {
	g, err := s.getOwnedGame(ctx, userID, gameID)
	if err != nil {
		return domain.Game{}, err
	}
	if !g.PlanStage() {
		return domain.Game{}, planStateErr(g)
	}
	if spec != nil {
		spec.Normalize()
		if err := s.repo.SetSpec(ctx, gameID, *spec); err != nil {
			return domain.Game{}, err
		}
	} else if specOf(g) == nil {
		return domain.Game{}, apperr.NewValidationError("spec", "плана ещё нет: уточните запрос в чате")
	}

	ok, err := s.repo.TrySetStatus(ctx, gameID, domain.StatusGenerating,
		[]string{domain.StatusAwaitingApproval, domain.StatusFailed})
	if err != nil {
		return domain.Game{}, err
	}
	if !ok {
		return domain.Game{}, apperr.ErrConflict
	}
	s.publish(gameID, domain.StatusGenerating, "", 0)
	s.launch(gameID, domain.GenBuild, "")
	return s.repo.GetGame(ctx, gameID)
}

// IterateGame — правка собранной игры в чате; идёт в фоне, как и сборка.
func (s *Service) IterateGame(ctx context.Context, userID, gameID, instruction string) (domain.Game, error) {
	instruction, err := validatePrompt(instruction)
	if err != nil {
		return domain.Game{}, err
	}
	g, err := s.getOwnedGame(ctx, userID, gameID)
	if err != nil {
		return domain.Game{}, err
	}
	if !g.Generated() {
		return domain.Game{}, apperr.ErrNotGenerated
	}

	s.reviews.stop(gameID) // правка пользователя важнее фоновой проверки
	ok, err := s.repo.TrySetStatus(ctx, gameID, domain.StatusGenerating, []string{domain.StatusReady})
	if err != nil {
		return domain.Game{}, err
	}
	if !ok {
		return domain.Game{}, apperr.ErrConflict
	}
	s.publish(gameID, domain.StatusGenerating, "", 0)
	s.launch(gameID, domain.GenEdit, instruction)
	return s.repo.GetGame(ctx, gameID)
}

func planStateErr(g domain.Game) error {
	if domain.Busy(g.Status) {
		return apperr.ErrConflict
	}
	return apperr.NewValidationError("status", "игра уже собрана: правки — через чат игры")
}

// plan вызывает планировщик; Prompt и Hint в in заполняются здесь.
func (s *Service) plan(ctx context.Context, g domain.Game, kind string, in agent.PlanInput, wait time.Duration) (domain.GameSpec, error) {
	release, err := s.acquireSlot(ctx, wait)
	if err != nil {
		return domain.GameSpec{}, err
	}
	defer release()

	genID, err := s.repo.StartGeneration(ctx, g.ID, kind)
	if err != nil {
		return domain.GameSpec{}, err
	}
	start, tr := time.Now(), &agent.Trace{}
	in.Prompt, in.Hint = g.Prompt, s.hint(ctx)
	spec, planErr := s.agent.Plan(ctx, in, tr)

	res := genResult(genID, start, tr)
	res.Status = domain.GenStatusReady
	if planErr != nil {
		res.Status, res.Error = domain.GenStatusFailed, planErr.Error()
	}
	s.finishGen(context.WithoutCancel(ctx), res)
	return spec, planErr
}

func (s *Service) savePlan(ctx context.Context, gameID, userText string, spec domain.GameSpec) error {
	if err := s.repo.SetSpec(ctx, gameID, spec); err != nil {
		return err
	}
	if err := s.repo.AddMessage(ctx, gameID, "user", userText); err != nil {
		return err
	}
	return s.repo.AddMessage(ctx, gameID, "assistant", spec.Summary)
}

func (s *Service) launch(gameID, kind, instruction string) {
	s.background(gameID, kind, func(ctx context.Context) {
		s.runBuild(ctx, gameID, kind, instruction, waitForBuildSlot)
	})
}

// background запускает работу агента над игрой в контексте приложения.
func (s *Service) background(gameID, kind string, job func(ctx context.Context)) {
	s.jobs.Add(1)
	go func() {
		defer s.jobs.Done()
		defer func() {
			if r := recover(); r != nil {
				slog.Error("сборка игры упала", "game", gameID, "panic", r)
				s.failBuild(context.WithoutCancel(s.bg), gameID, kind, fmt.Errorf("внутренняя ошибка: %v", r))
			}
		}()
		job(s.bg)
	}()
}

// StartVariant создаёт игру-вариант эксперимента и в фоне ведёт её от плана до сборки
// без подтверждения: варианты сравнивают игроки, а не автор по одному.
func (s *Service) StartVariant(ctx context.Context, in domain.VariantInput) (string, error) {
	prompt, err := validatePrompt(in.Prompt)
	if err != nil {
		return "", err
	}
	in.Prompt = prompt
	id, err := s.repo.CreateVariant(ctx, in)
	if err != nil {
		return "", err
	}
	s.background(id, domain.GenBuild, func(ctx context.Context) { s.runVariant(ctx, id, in) })
	return id, nil
}

func (s *Service) runVariant(ctx context.Context, gameID string, in domain.VariantInput) {
	wctx := context.WithoutCancel(ctx)

	g := domain.Game{ID: gameID, OwnerID: in.OwnerID, Prompt: in.Prompt}
	spec, err := s.plan(ctx, g, domain.GenPlan, agent.PlanInput{Direction: in.Direction}, waitForVariantSlot)
	if err != nil {
		s.setStatus(wctx, gameID, domain.StatusFailed, "не удалось составить план: "+err.Error(), 0)
		return
	}
	userText := in.Prompt + "\n\nВариант " + in.Label + ": " + in.Direction
	if err := s.savePlan(wctx, gameID, userText, spec); err != nil {
		s.failBuild(wctx, gameID, domain.GenBuild, err)
		return
	}
	s.setStatus(wctx, gameID, domain.StatusGenerating, "", 0)
	s.runBuild(ctx, gameID, domain.GenBuild, "", waitForVariantSlot)
}

func (s *Service) runBuild(ctx context.Context, gameID, kind, instruction string, wait time.Duration) {
	wctx := context.WithoutCancel(ctx)

	release, err := s.acquireSlot(ctx, wait)
	if err != nil {
		s.failBuild(wctx, gameID, kind, err)
		return
	}
	defer release()

	g, err := s.repo.GetGame(ctx, gameID)
	if err != nil {
		s.failBuild(wctx, gameID, kind, err)
		return
	}
	genID, err := s.repo.StartGeneration(ctx, gameID, kind)
	if err != nil {
		s.failBuild(wctx, gameID, kind, err)
		return
	}

	est := s.estimate(ctx, kind)
	s.progress.start(gameID, est.DurationMs)
	defer s.progress.finish(gameID)

	start, tr := time.Now(), &agent.Trace{}
	in := agent.BuildInput{Prompt: g.Prompt, Spec: specOf(g), Hint: s.hint(ctx),
		Progress: s.progressFn(gameID), ExpectedChars: est.CoderChars}
	if kind == domain.GenEdit {
		in.CurrentHTML, in.Instruction, in.History = g.HTML, instruction, s.history(ctx, gameID)
	}
	stage := func(status, detail string) {
		s.progress.stage(gameID, status, detail)
		s.setStatus(wctx, gameID, status, detail, 0)
	}

	out, err := s.agent.Build(ctx, in, stage, tr)
	res := genResult(genID, start, tr)
	if err != nil {
		res.Status, res.Error = domain.GenStatusFailed, err.Error()
		s.finishGen(wctx, res)
		s.failBuild(wctx, gameID, kind, err)
		return
	}

	reply, detail := out.Result.Reply, ""
	res.Status = domain.GenStatusReady
	if domain.HasErrors(out.Issues) {
		res.Status = domain.GenStatusWithIssues
		detail = "автопроверка: остались проблемы (" + issueCodes(out.Issues) + ")"
		reply += "\n\nАвтопроверка нашла проблемы, которые не удалось исправить автоматически. Опишите в чате, что работает не так, — поправлю."
	}
	vin := domain.VersionInput{
		GameID: gameID, Reply: reply, HTML: out.HTML,
		Title: out.Result.Title, Genre: out.Result.Genre, Audience: out.Result.Audience,
		Instruction: "сборка по утверждённому плану",
	}
	if kind == domain.GenEdit {
		vin.UserText, vin.Instruction = instruction, instruction
	}
	version, err := s.repo.AddVersion(wctx, vin)
	if err != nil {
		res.Status, res.Error = domain.GenStatusFailed, err.Error()
		s.finishGen(wctx, res)
		s.failBuild(wctx, gameID, kind, err)
		return
	}
	res.Version = &version
	s.finishGen(wctx, res)
	// первая сборка уходит на фоновую проверку логики; игрок уже играет и может править
	if kind == domain.GenBuild && g.ExperimentID == "" {
		s.startReview(gameID, version, specOf(g))
	}
	s.setStatus(wctx, gameID, domain.StatusReady, detail, version)
}

// failBuild: неудачная правка оставляет рабочую версию, неудачная первая сборка — черновик с планом.
func (s *Service) failBuild(ctx context.Context, gameID, kind string, err error) {
	if kind == domain.GenEdit {
		s.setStatus(ctx, gameID, domain.StatusReady, "не удалось применить изменения: "+err.Error(), 0)
		return
	}
	s.setStatus(ctx, gameID, domain.StatusFailed, "не удалось собрать игру: "+err.Error(), 0)
}

func (s *Service) acquireSlot(ctx context.Context, wait time.Duration) (func(), error) {
	timer := time.NewTimer(wait)
	defer timer.Stop()

	select {
	case s.genSem <- struct{}{}:
		return func() { <-s.genSem }, nil
	case <-timer.C:
		return nil, apperr.ErrCapacity
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *Service) hint(ctx context.Context) llm.HintInput {
	if s.insights == nil {
		return llm.HintInput{}
	}
	return llm.HintInput{TopGames: s.insights.GetInsightsHint(ctx)}
}

func (s *Service) history(ctx context.Context, gameID string) []llm.Msg {
	msgs, err := s.repo.Messages(ctx, gameID)
	if err != nil {
		return nil
	}
	out := make([]llm.Msg, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, llm.Msg{Role: m.Role, Content: m.Content})
	}
	return out
}

func (s *Service) setStatus(ctx context.Context, gameID, status, detail string, version int) {
	if err := s.repo.SetStatus(ctx, gameID, status, detail); err != nil {
		slog.Error("статус генерации не сохранён", "game", gameID, "status", status, "err", err)
	}
	s.publish(gameID, status, detail, version)
}

func (s *Service) publish(gameID, status, detail string, version int) {
	ev := domain.StatusEvent{GameID: gameID, Status: status, Detail: detail, Version: version, At: time.Now()}
	if domain.Busy(status) {
		ev.Progress = s.progress.get(gameID)
	}
	s.events.publish(ev)
}

func (s *Service) finishGen(ctx context.Context, res domain.GenerationResult) {
	if err := s.repo.FinishGeneration(ctx, res); err != nil {
		slog.Error("телеметрия генерации не сохранена", "generation", res.ID, "err", err)
	}
}

func genResult(id int64, start time.Time, tr *agent.Trace) domain.GenerationResult {
	res := domain.GenerationResult{
		ID:         id,
		DurationMs: time.Since(start).Milliseconds(),
		FixRounds:  tr.FixRounds,
		Issues:     tr.Issues,
		Calls:      tr.Calls,
	}
	if tr.Report != nil {
		res.TestReport, _ = json.Marshal(tr.Report)
		res.Screenshot = tr.Report.Screenshot
	}
	return res
}

func specOf(g domain.Game) *domain.GameSpec {
	if len(g.Spec) == 0 {
		return nil
	}
	var spec domain.GameSpec
	if json.Unmarshal(g.Spec, &spec) != nil {
		return nil
	}
	return &spec
}

func issueCodes(issues []domain.Issue) string {
	seen := map[string]bool{}
	var codes []string
	for _, i := range issues {
		if i.Severity == domain.SevError && !i.Fixed && !seen[i.Code] {
			seen[i.Code] = true
			codes = append(codes, i.Code)
		}
	}
	return strings.Join(codes, ", ")
}

func (s *Service) Subscribe(ctx context.Context, userID, gameID string) (domain.Game, <-chan domain.StatusEvent, func(), error) {
	g, err := s.getOwnedGame(ctx, userID, gameID)
	if err != nil {
		return domain.Game{}, nil, nil, err
	}
	ch, cancel := s.events.subscribe(gameID)
	return g, ch, cancel, nil
}

// GetStatus — игра без HTML; пока идёт сборка — её ход, пока идёт фоновая проверка — её состояние.
func (s *Service) GetStatus(ctx context.Context, userID, gameID string) (domain.Game, *domain.BuildProgress, *domain.ReviewState, error) {
	g, err := s.getOwnedGame(ctx, userID, gameID)
	if err != nil {
		return domain.Game{}, nil, nil, err
	}
	g.HTML = ""
	review := s.reviews.get(gameID)
	if !domain.Busy(g.Status) {
		return g, nil, review, nil
	}
	return g, s.progress.get(gameID), review, nil
}

func (s *Service) ListGenerations(ctx context.Context, userID, gameID string) ([]domain.Generation, error) {
	if _, err := s.getOwnedGame(ctx, userID, gameID); err != nil {
		return nil, err
	}
	return s.repo.ListGenerations(ctx, gameID, generationsLimit)
}

func (s *Service) Screenshot(ctx context.Context, userID, gameID string) ([]byte, error) {
	if _, err := s.GetGame(ctx, userID, gameID); err != nil {
		return nil, err
	}
	return s.repo.LatestScreenshot(ctx, gameID)
}

func (s *Service) GenerationStats(ctx context.Context, role string, days int) (domain.GenerationStats, error) {
	if role != roleManager {
		return domain.GenerationStats{}, apperr.ErrForbidden
	}
	days = clamp(days, 1, 365, 30)
	st, err := s.repo.GenerationStats(ctx, time.Now().AddDate(0, 0, -days))
	st.Days = days
	return st, err
}

// RecoverInterrupted помечает генерации, оборванные перезапуском сервера.
func (s *Service) RecoverInterrupted(ctx context.Context) error {
	n, err := s.repo.ResetInterrupted(ctx)
	if n > 0 {
		slog.Warn("сброшены прерванные генерации", "games", n)
	}
	return err
}

func validatePrompt(text string) (string, error) {
	text = strings.TrimSpace(text)
	if n := utf8.RuneCountInString(text); n < minPromptLen || n > maxPromptLen {
		return "", apperr.NewValidationError("prompt",
			fmt.Sprintf("текст: от %d до %d символов", minPromptLen, maxPromptLen))
	}
	return text, nil
}
