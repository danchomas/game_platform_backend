// Package agent — агентный конвейер генерации игр:
// план → (подтверждение пользователем) → код → статическая проверка →
// тест в headless-браузере → исправление по найденным проблемам.
package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"beeline-games/internal/core/apperr"
	"beeline-games/internal/core/domain"
	"beeline-games/internal/core/gamekit"
	"beeline-games/internal/core/htmlguard"
	"beeline-games/internal/core/llm"
)

type Models struct {
	Planner   string
	Coder     string
	Fixer     string
	Fallbacks []string
	MaxTokens int
}

type Config struct {
	Models       Models
	MaxFixRounds int
	CallTimeout  time.Duration
	// Reasoning — бюджет рассуждений «думающих» моделей (llm.Reasoning*), см. LLM_REASONING.
	Reasoning string
}

type Agent struct {
	llm    llm.Provider
	tester Tester
	cfg    Config
}

// New: tester может быть nil — тогда остаются только статические проверки.
func New(p llm.Provider, t Tester, cfg Config) *Agent {
	if cfg.Models.Fixer == "" {
		cfg.Models.Fixer = cfg.Models.Coder
	}
	if cfg.Models.Planner == "" {
		cfg.Models.Planner = cfg.Models.Coder
	}
	if cfg.Models.MaxTokens <= 0 {
		cfg.Models.MaxTokens = 32000
	}
	if cfg.CallTimeout <= 0 {
		cfg.CallTimeout = 4 * time.Minute
	}
	return &Agent{llm: p, tester: t, cfg: cfg}
}

func (a *Agent) Models() Models { return a.cfg.Models }

// Trace собирает телеметрию одного запуска агента.
type Trace struct {
	Calls []domain.LLMCall
	// Issues — оставшиеся проблемы плюс исправленные на прошлых кругах (Fixed=true):
	// по ним видно, что чаще всего ломается в генерации.
	Issues    []domain.Issue
	Report    *TestReport
	FixRounds int
}

// withFixed дополняет текущие проблемы теми, что были на прошлых кругах и исчезли.
func withFixed(current, earlier []domain.Issue) []domain.Issue {
	have := make(map[string]bool, len(current))
	for _, i := range current {
		have[i.Code] = true
	}
	out := append([]domain.Issue{}, current...)
	for _, i := range earlier {
		if !have[i.Code] {
			have[i.Code] = true
			i.Fixed = true
			out = append(out, i)
		}
	}
	return out
}

func (t *Trace) Usage() llm.Usage {
	var u llm.Usage
	for _, c := range t.Calls {
		u.Add(llm.Usage{PromptTokens: c.PromptTokens, CompletionTokens: c.CompletionTokens, CostUSD: c.CostUSD})
	}
	return u
}

// Stage сообщает наружу о смене шага (для статуса игры и SSE).
type Stage func(status, detail string)

// call вызывает модель. Если «думающая» модель израсходовала лимит на рассуждения и не
// ответила, вызов повторяется один раз с выключенными рассуждениями.
func (a *Agent) call(ctx context.Context, role, model string, msgs []llm.Msg, temp float64, maxCont int, tr *Trace) (string, error) {
	text, err := a.callOnce(ctx, role, model, msgs, temp, maxCont, a.cfg.Reasoning, tr)
	if errors.Is(err, llm.ErrEmptyTruncated) && a.cfg.Reasoning != llm.ReasoningOff {
		return a.callOnce(ctx, role, model, msgs, temp, maxCont, llm.ReasoningOff, tr)
	}
	return text, err
}

func (a *Agent) callOnce(ctx context.Context, role, model string, msgs []llm.Msg, temp float64, maxCont int, reasoning string, tr *Trace) (string, error) {
	// LLM_TIMEOUT_SEC — на каждый запрос: продолжение длинного ответа получает свой таймаут
	resp, err := llm.CompleteFull(ctx, llm.WithCallTimeout(a.llm, a.cfg.CallTimeout), llm.Request{
		Model:       model,
		Fallbacks:   a.cfg.Models.Fallbacks,
		Messages:    msgs,
		MaxTokens:   a.cfg.Models.MaxTokens,
		Temperature: temp,
		Reasoning:   reasoning,
	}, maxCont)

	c := domain.LLMCall{
		Role:             role,
		Model:            model,
		PromptTokens:     resp.Usage.PromptTokens,
		CompletionTokens: resp.Usage.CompletionTokens,
		ReasoningTokens:  resp.Usage.ReasoningTokens,
		CostUSD:          resp.Usage.CostUSD,
		LatencyMs:        resp.Latency.Milliseconds(),
		FinishReason:     resp.FinishReason,
	}
	if resp.Model != "" {
		c.Model = resp.Model
	}
	if err != nil {
		c.Error = err.Error()
	}
	if tr != nil {
		tr.Calls = append(tr.Calls, c)
	}
	return resp.Text, err
}

type PlanInput struct {
	Prompt   string
	History  []llm.Msg
	Current  *domain.GameSpec
	Feedback string
	Hint     llm.HintInput
}

// Plan составляет план игры или правит текущий по отзыву пользователя.
func (a *Agent) Plan(ctx context.Context, in PlanInput, tr *Trace) (domain.GameSpec, error) {
	msgs := llm.PlanMessages(in.Hint, in.History, in.Prompt, in.Current, in.Feedback)
	var spec domain.GameSpec
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		raw, err := a.call(ctx, "planner", a.cfg.Models.Planner, msgs, 0.4, 0, tr)
		if err != nil {
			return spec, err
		}
		spec = domain.GameSpec{}
		if lastErr = llm.ParseJSON(raw, &spec); lastErr == nil {
			break
		}
		msgs = append(msgs, llm.Msg{Role: "assistant", Content: raw},
			llm.Msg{Role: "user", Content: "That was not valid JSON. Return ONLY the JSON object of the plan."})
	}
	if lastErr != nil {
		return spec, fmt.Errorf("%w: план не разобран: %w", apperr.ErrLLMFailed, lastErr)
	}
	spec.Normalize()
	if strings.TrimSpace(spec.Summary) == "" {
		spec.Summary = fallbackSummary(spec)
	}
	return spec, nil
}

func fallbackSummary(s domain.GameSpec) string {
	parts := []string{"• " + s.Title + ": " + s.Concept}
	if s.Controls.Description != "" {
		parts = append(parts, "• Управление: "+s.Controls.Description)
	}
	if s.WinCondition != "" {
		parts = append(parts, "• Победа: "+s.WinCondition)
	}
	return strings.Join(parts, "\n")
}

type BuildInput struct {
	Prompt      string
	Spec        *domain.GameSpec
	CurrentHTML string // пусто — новая игра по плану; иначе — правка
	Instruction string
	History     []llm.Msg
	Hint        llm.HintInput
}

type BuildOutput struct {
	HTML   string // с встроенным GameKit, готово к сохранению
	Result llm.Result
	Issues []domain.Issue // оставшиеся после всех кругов исправления
}

// Build генерирует (или правит) игру и доводит её до рабочего состояния.
func (a *Agent) Build(ctx context.Context, in BuildInput, stage Stage, tr *Trace) (BuildOutput, error) {
	if stage == nil {
		stage = func(string, string) {}
	}
	if in.CurrentHTML == "" && in.Spec == nil {
		return BuildOutput{}, apperr.NewValidationError("spec", "нет ни плана, ни текущего кода")
	}

	stage(domain.StatusGenerating, "")
	var msgs []llm.Msg
	if in.CurrentHTML == "" {
		msgs = llm.BuildMessages(in.Hint, *in.Spec, in.Prompt)
	} else {
		msgs = llm.EditMessages(in.Hint, in.History, in.Spec, gamekit.Strip(in.CurrentHTML), in.Instruction)
	}
	res, err := a.code(ctx, "coder", a.cfg.Models.Coder, msgs, 0.7, tr)
	if err != nil {
		return BuildOutput{}, err
	}
	html := gamekit.Strip(res.HTML)

	var issues, earlier []domain.Issue
	for round := 0; ; round++ {
		if err := ctx.Err(); err != nil {
			return BuildOutput{}, err
		}
		stage(domain.StatusValidating, "")
		issues = Validate(html, in.Spec)

		if a.tester != nil {
			stage(domain.StatusTesting, "")
			rep, terr := a.tester.Test(ctx, gamekit.Embed(html))
			if terr != nil {
				issues = append(issues, domain.Issue{Code: "test_unavailable", Severity: domain.SevWarn, Message: "browser test failed to run: " + terr.Error()})
			} else {
				tr.Report = &rep
				issues = append(issues, rep.Issues()...)
			}
		}
		tr.Issues = withFixed(issues, earlier)

		if !domain.HasErrors(issues) || round >= a.cfg.MaxFixRounds {
			break
		}
		earlier = append(earlier, issues...)

		blocking := errorsOnly(issues)
		stage(domain.StatusFixing, fmt.Sprintf("круг %d: %s", round+1, issueCodes(blocking)))
		tr.FixRounds++
		fixed, ferr := a.code(ctx, "fixer", a.cfg.Models.Fixer, llm.FixMessages(html, blocking), 0.2, tr)
		if ferr != nil {
			if ctx.Err() != nil {
				return BuildOutput{}, ctx.Err()
			}
			break // оставляем последнюю версию, проблемы попадут в отчёт
		}
		html = gamekit.Strip(fixed.HTML)
		if fixed.Title != "" {
			res.Title, res.Genre, res.Audience = fixed.Title, fixed.Genre, fixed.Audience
		}
	}

	res.HTML = gamekit.Embed(html)
	if in.Spec != nil && in.CurrentHTML == "" {
		if res.Title == "" {
			res.Title = in.Spec.Title
		}
		if res.Genre == "" {
			res.Genre = in.Spec.Genre
		}
		if res.Audience == "" {
			res.Audience = in.Spec.Audience
		}
	}
	if tr.FixRounds > 0 && !domain.HasErrors(issues) {
		res.Reply += " (автопроверка нашла и исправила ошибки)"
	}
	return BuildOutput{HTML: res.HTML, Result: res, Issues: issues}, nil
}

// code — вызов модели, которая должна вернуть игру. Ответ проходит llm.Parse и htmlguard;
// непригодный ответ получает ещё одну попытку, сетевая ошибка — нет.
func (a *Agent) code(ctx context.Context, role, model string, msgs []llm.Msg, temp float64, tr *Trace) (llm.Result, error) {
	var lastErr error
	for attempt := range 2 {
		raw, err := a.call(ctx, role, model, msgs, temp, 2, tr)
		if err != nil {
			return llm.Result{}, err
		}
		res, perr := parseGame(raw)
		if perr == nil {
			return res, nil
		}
		lastErr = fmt.Errorf("попытка %d: %w", attempt+1, perr)
	}
	return llm.Result{}, lastErr
}

func parseGame(raw string) (llm.Result, error) {
	res, err := llm.Parse(raw)
	if err != nil {
		return llm.Result{}, fmt.Errorf("%w: %w", apperr.ErrLLMFailed, err)
	}
	res.HTML = htmlguard.Sanitize(res.HTML)
	if err := htmlguard.Validate(res.HTML); err != nil {
		return llm.Result{}, err
	}
	return res, nil
}

func errorsOnly(issues []domain.Issue) []domain.Issue {
	var out []domain.Issue
	for _, i := range issues {
		if i.Severity == domain.SevError {
			out = append(out, i)
		}
	}
	return out
}

func issueCodes(issues []domain.Issue) string {
	codes := make([]string, 0, len(issues))
	for _, i := range issues {
		codes = append(codes, i.Code)
	}
	return strings.Join(codes, ", ")
}
