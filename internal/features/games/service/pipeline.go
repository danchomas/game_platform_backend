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
	"beeline-games/internal/core/domain"
	coreErrs "beeline-games/internal/core/errors"
	"beeline-games/internal/core/llm"
)

const (
	slotWaitSync  = 20 * time.Second
	slotWaitAsync = 3 * time.Minute
)

func validText(s string) (string, error) {
	s = strings.TrimSpace(s)
	if n := utf8.RuneCountInString(s); n < 3 || n > 4000 {
		return s, coreErrs.ErrInvalidInput
	}
	return s, nil
}

func (s *service) acquire(ctx context.Context, wait time.Duration) (func(), error) {
	select {
	case s.genSem <- struct{}{}:
		return func() { <-s.genSem }, nil
	case <-time.After(wait):
		return nil, errBusySlot
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *service) hint(ctx context.Context) llm.HintInput {
	if s.insights == nil {
		return llm.HintInput{}
	}
	return llm.HintInput{TopGames: s.insights.GetInsightsHint(ctx)}
}

func (s *service) setStatus(ctx context.Context, gameID, status, detail string, version int) {
	if err := s.repo.SetStatus(ctx, gameID, status, detail); err != nil {
		slog.Error("set game status", "game", gameID, "status", status, "err", err)
	}
	s.events.publish(domain.StatusEvent{GameID: gameID, Status: status, Detail: detail, Version: version, At: time.Now()})
}

func (s *service) history(ctx context.Context, gameID string) []llm.Msg {
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

func specOf(g domain.Game) *domain.GameSpec {
	if len(g.Spec) == 0 {
		return nil
	}
	var sp domain.GameSpec
	if json.Unmarshal(g.Spec, &sp) != nil {
		return nil
	}
	return &sp
}

func (s *service) finishGen(ctx context.Context, res domain.GenerationResult) {
	if err := s.repo.FinishGeneration(ctx, res); err != nil {
		slog.Error("save generation", "id", res.ID, "err", err)
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
		res.Test, _ = json.Marshal(tr.Report)
		res.Screenshot = tr.Report.Screenshot
	}
	return res
}

// plan — синхронный шаг планирования (несколько секунд, дешёвая модель).
func (s *service) plan(ctx context.Context, g domain.Game, kind, feedback string) (domain.GameSpec, error) {
	release, err := s.acquire(ctx, slotWaitSync)
	if err != nil {
		return domain.GameSpec{}, err
	}
	defer release()

	genID, err := s.repo.StartGeneration(ctx, g.ID, kind)
	if err != nil {
		return domain.GameSpec{}, err
	}
	start, tr := time.Now(), &agent.Trace{}
	in := agent.PlanInput{Prompt: g.Prompt, Hint: s.hint(ctx)}
	if kind == domain.GenRevise {
		in.Current, in.Feedback, in.History = specOf(g), feedback, s.history(ctx, g.ID)
	}
	spec, perr := s.agent.Plan(ctx, in, tr)

	res := genResult(genID, start, tr)
	res.Status = domain.StatusReady
	if perr != nil {
		res.Status, res.Error = domain.StatusFailed, perr.Error()
	}
	s.finishGen(context.WithoutCancel(ctx), res)
	return spec, perr
}

func (s *service) savePlan(ctx context.Context, gameID, userText string, spec domain.GameSpec) error {
	if err := s.repo.SetSpec(ctx, gameID, spec); err != nil {
		return err
	}
	if err := s.repo.AddMessage(ctx, gameID, "user", userText); err != nil {
		return err
	}
	return s.repo.AddMessage(ctx, gameID, "assistant", spec.Summary)
}

func (s *service) CreateGame(ctx context.Context, ownerID, prompt string, autoApprove bool) (domain.Game, domain.Plan, error) {
	prompt, err := validText(prompt)
	if err != nil {
		return domain.Game{}, domain.Plan{}, err
	}
	id, err := s.repo.CreateGame(ctx, ownerID, prompt)
	if err != nil {
		return domain.Game{}, domain.Plan{}, err
	}
	g := domain.Game{ID: id, OwnerID: ownerID, Prompt: prompt}

	spec, err := s.plan(ctx, g, domain.GenPlan, "")
	wctx := context.WithoutCancel(ctx)
	if err != nil {
		s.setStatus(wctx, id, domain.StatusFailed, "не удалось составить план: "+err.Error(), 0)
		g, _ = s.repo.GetGame(wctx, id)
		return g, domain.Plan{}, err
	}
	if err := s.savePlan(wctx, id, prompt, spec); err != nil {
		return g, domain.Plan{}, err
	}
	s.setStatus(wctx, id, domain.StatusAwaitingApproval, "", 0)

	if autoApprove {
		if ok, err := s.repo.TrySetStatus(wctx, id, domain.StatusGenerating, []string{domain.StatusAwaitingApproval}); err == nil && ok {
			s.launch(id, domain.GenBuild, "")
		}
	}
	g, err = s.repo.GetGame(wctx, id)
	return g, domain.Plan{Spec: spec, Reply: spec.Summary}, err
}

func (s *service) RevisePlan(ctx context.Context, userID, gameID, feedback string) (domain.Game, domain.Plan, error) {
	feedback, err := validText(feedback)
	if err != nil {
		return domain.Game{}, domain.Plan{}, err
	}
	g, err := s.getOwnedGame(ctx, userID, gameID)
	if err != nil {
		return g, domain.Plan{}, err
	}
	if g.Version > 0 {
		return g, domain.Plan{}, ErrBusy // игра уже собрана: дальше правки через чат
	}
	prev := g.Status
	ok, err := s.repo.TrySetStatus(ctx, gameID, domain.StatusPlanning, []string{domain.StatusAwaitingApproval, domain.StatusFailed})
	if err != nil {
		return g, domain.Plan{}, err
	}
	if !ok {
		return g, domain.Plan{}, ErrBusy
	}
	s.events.publish(domain.StatusEvent{GameID: gameID, Status: domain.StatusPlanning, At: time.Now()})

	kind := domain.GenRevise
	if specOf(g) == nil {
		kind = domain.GenPlan // плана ещё нет (прошлая попытка упала) — строим заново с учётом отзыва
		g.Prompt += "\n\nУточнение: " + feedback
	}
	spec, err := s.plan(ctx, g, kind, feedback)
	wctx := context.WithoutCancel(ctx)
	if err != nil {
		s.setStatus(wctx, gameID, prev, "не удалось обновить план: "+err.Error(), 0)
		return g, domain.Plan{}, err
	}
	if err := s.savePlan(wctx, gameID, feedback, spec); err != nil {
		return g, domain.Plan{}, err
	}
	s.setStatus(wctx, gameID, domain.StatusAwaitingApproval, "", 0)
	g, err = s.repo.GetGame(wctx, gameID)
	return g, domain.Plan{Spec: spec, Reply: spec.Summary}, err
}

// ApprovePlan запускает фоновую сборку. spec != nil — план, отредактированный во фронтенде.
func (s *service) ApprovePlan(ctx context.Context, userID, gameID string, spec *domain.GameSpec) (domain.Game, error) {
	g, err := s.getOwnedGame(ctx, userID, gameID)
	if err != nil {
		return g, err
	}
	if g.Version > 0 {
		return g, ErrBusy
	}
	if spec != nil {
		spec.Normalize()
		if g.Status != domain.StatusAwaitingApproval && g.Status != domain.StatusFailed {
			return g, ErrBusy
		}
		if err := s.repo.SetSpec(ctx, gameID, *spec); err != nil {
			return g, err
		}
	} else if specOf(g) == nil {
		return g, coreErrs.ErrInvalidInput
	}

	ok, err := s.repo.TrySetStatus(ctx, gameID, domain.StatusGenerating, []string{domain.StatusAwaitingApproval, domain.StatusFailed})
	if err != nil {
		return g, err
	}
	if !ok {
		return g, ErrBusy
	}
	s.events.publish(domain.StatusEvent{GameID: gameID, Status: domain.StatusGenerating, At: time.Now()})
	s.launch(gameID, domain.GenBuild, "")
	return s.repo.GetGame(ctx, gameID)
}

func (s *service) IterateGame(ctx context.Context, userID, gameID, instruction string) (domain.Game, error) {
	instruction, err := validText(instruction)
	if err != nil {
		return domain.Game{}, err
	}
	g, err := s.getOwnedGame(ctx, userID, gameID)
	if err != nil {
		return g, err
	}
	if g.Version == 0 {
		return g, coreErrs.ErrInvalidInput
	}
	ok, err := s.repo.TrySetStatus(ctx, gameID, domain.StatusGenerating, []string{domain.StatusReady})
	if err != nil {
		return g, err
	}
	if !ok {
		return g, ErrBusy
	}
	s.events.publish(domain.StatusEvent{GameID: gameID, Status: domain.StatusGenerating, At: time.Now()})
	s.launch(gameID, domain.GenEdit, instruction)
	return s.repo.GetGame(ctx, gameID)
}

func (s *service) launch(gameID, kind, instruction string) {
	s.jobs.Add(1)
	go func() {
		defer s.jobs.Done()
		defer func() {
			if r := recover(); r != nil {
				slog.Error("build panic", "game", gameID, "panic", r)
				s.failBuild(context.WithoutCancel(s.bg), gameID, kind, fmt.Errorf("внутренняя ошибка: %v", r))
			}
		}()
		s.runBuild(s.bg, gameID, kind, instruction)
	}()
}

func (s *service) failBuild(ctx context.Context, gameID, kind string, err error) {
	if kind == domain.GenEdit {
		s.setStatus(ctx, gameID, domain.StatusReady, "не удалось применить изменения: "+err.Error(), 0)
	} else {
		s.setStatus(ctx, gameID, domain.StatusFailed, "не удалось собрать игру: "+err.Error(), 0)
	}
}

func (s *service) runBuild(ctx context.Context, gameID, kind, instruction string) {
	wctx := context.WithoutCancel(ctx)

	release, err := s.acquire(ctx, slotWaitAsync)
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

	start, tr := time.Now(), &agent.Trace{}
	in := agent.BuildInput{Prompt: g.Prompt, Spec: specOf(g), Hint: s.hint(ctx)}
	if kind == domain.GenEdit {
		in.CurrentHTML, in.Instruction, in.History = g.HTML, instruction, s.history(ctx, gameID)
	}
	stage := func(status, detail string) { s.setStatus(wctx, gameID, status, detail, 0) }

	out, err := s.agent.Build(ctx, in, stage, tr)
	res := genResult(genID, start, tr)
	if err != nil {
		res.Status, res.Error = domain.StatusFailed, err.Error()
		s.finishGen(wctx, res)
		s.failBuild(wctx, gameID, kind, err)
		return
	}

	reply, detail := out.Result.Reply, ""
	res.Status = domain.StatusReady
	if domain.HasErrors(out.Issues) {
		res.Status = "ready_with_issues"
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
	v, err := s.repo.AddVersion(wctx, vin)
	if err != nil {
		res.Status, res.Error = domain.StatusFailed, err.Error()
		s.finishGen(wctx, res)
		s.failBuild(wctx, gameID, kind, err)
		return
	}
	res.Version = &v
	s.finishGen(wctx, res)
	s.setStatus(wctx, gameID, domain.StatusReady, detail, v)
}

func issueCodes(issues []domain.Issue) string {
	seen := map[string]bool{}
	var codes []string
	for _, i := range issues {
		if i.Severity == domain.SevError && !seen[i.Code] {
			seen[i.Code] = true
			codes = append(codes, i.Code)
		}
	}
	return strings.Join(codes, ", ")
}

func (s *service) Subscribe(ctx context.Context, userID, gameID string) (domain.Game, <-chan domain.StatusEvent, func(), error) {
	g, err := s.getOwnedGame(ctx, userID, gameID)
	if err != nil {
		return g, nil, nil, err
	}
	ch, cancel := s.events.subscribe(gameID)
	return g, ch, cancel, nil
}

func (s *service) ListGenerations(ctx context.Context, userID, gameID string) ([]domain.Generation, error) {
	if _, err := s.getOwnedGame(ctx, userID, gameID); err != nil {
		return nil, err
	}
	return s.repo.ListGenerations(ctx, gameID, 50)
}

func (s *service) Screenshot(ctx context.Context, userID, gameID string) ([]byte, error) {
	if _, err := s.GetGame(ctx, userID, gameID); err != nil {
		return nil, err
	}
	return s.repo.LatestScreenshot(ctx, gameID)
}

func (s *service) GenerationStats(ctx context.Context, role string, days int) (domain.GenerationStats, error) {
	if role != "manager" {
		return domain.GenerationStats{}, coreErrs.ErrForbidden
	}
	st, err := s.repo.GenerationStats(ctx, time.Now().AddDate(0, 0, -days))
	st.Days = days
	return st, err
}

func (s *service) RecoverInterrupted(ctx context.Context) error {
	n, err := s.repo.ResetInterrupted(ctx)
	if n > 0 {
		slog.Warn("reset interrupted generations", "games", n)
	}
	return err
}
