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
	Planner string
	Coder   string
	Fixer   string
	// Reviewer — проверка логики кода по плану; пусто — без проверки.
	Reviewer  string
	Fallbacks []string
	MaxTokens int
}

type Config struct {
	Models       Models
	MaxFixRounds int
	CallTimeout  time.Duration
	// Reasoning — бюджет рассуждений «думающих» моделей (llm.Reasoning*) для кодера и исправителя.
	// У планировщика и ревьюера свои: пусто — как у кодера.
	Reasoning         string
	PlannerReasoning  string
	ReviewerReasoning string
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
	if cfg.PlannerReasoning == "" {
		cfg.PlannerReasoning = cfg.Reasoning
	}
	if cfg.ReviewerReasoning == "" {
		cfg.ReviewerReasoning = cfg.Reasoning
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
// w != nil — ответ идёт потоком, и его ход сообщается наружу.
func (a *Agent) call(ctx context.Context, role, model string, msgs []llm.Msg, temp float64, maxCont int, w watcher, tr *Trace) (string, error) {
	reasoning := a.reasoningFor(role)
	text, err := a.callOnce(ctx, role, model, msgs, temp, maxCont, reasoning, w, tr)
	if errors.Is(err, llm.ErrEmptyTruncated) && reasoning != llm.ReasoningOff {
		return a.callOnce(ctx, role, model, msgs, temp, maxCont, llm.ReasoningOff, w, tr)
	}
	return text, err
}

func (a *Agent) reasoningFor(role string) string {
	switch role {
	case "planner":
		return a.cfg.PlannerReasoning
	case "reviewer":
		return a.cfg.ReviewerReasoning
	default:
		return a.cfg.Reasoning
	}
}

func (a *Agent) callOnce(ctx context.Context, role, model string, msgs []llm.Msg, temp float64, maxCont int, reasoning string, w watcher, tr *Trace) (string, error) {
	var onDelta func(string)
	if w != nil {
		onDelta = w()
	}
	// LLM_TIMEOUT_SEC — на каждый запрос: продолжение длинного ответа получает свой таймаут
	resp, err := llm.CompleteFull(ctx, llm.WithCallTimeout(a.llm, a.cfg.CallTimeout), llm.Request{
		Model:       model,
		Fallbacks:   a.cfg.Models.Fallbacks,
		Messages:    msgs,
		MaxTokens:   a.cfg.Models.MaxTokens,
		Temperature: temp,
		Reasoning:   reasoning,
		OnDelta:     onDelta,
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
		OutputChars:      len(resp.Text),
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
	// Direction — направление варианта мультигенерации, пусто для обычной игры.
	Direction string
	Hint      llm.HintInput
}

// Plan составляет план игры или правит текущий по отзыву пользователя.
func (a *Agent) Plan(ctx context.Context, in PlanInput, tr *Trace) (domain.GameSpec, error) {
	msgs := llm.PlanMessages(in.Hint, in.History, in.Prompt, in.Current, in.Feedback, in.Direction)
	var spec domain.GameSpec
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		raw, err := a.call(ctx, "planner", a.cfg.Models.Planner, msgs, 0.4, 0, nil, tr)
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
	// Progress != nil — код пишется потоком, ход сообщается сюда
	Progress Progress
	// ExpectedChars — обычный размер новой игры по истории; 0 — значение по умолчанию.
	// При правке ожидаемый размер — размер текущего кода.
	ExpectedChars int
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
	expected := in.ExpectedChars
	if expected <= 0 {
		expected = defaultExpectedChars
	}
	if in.CurrentHTML == "" {
		msgs = llm.BuildMessages(in.Hint, *in.Spec, in.Prompt)
	} else {
		current := gamekit.Strip(in.CurrentHTML)
		msgs = llm.EditMessages(in.Hint, in.History, in.Spec, current, in.Instruction)
		expected = len(current) // правка возвращает файл целиком
	}
	res, err := a.code(ctx, "coder", a.cfg.Models.Coder, msgs, 0.7, in.Progress.watch("coder", expected), tr)
	if err != nil {
		return BuildOutput{}, err
	}
	html := gamekit.Strip(res.HTML)

	// Соответствие плану проверяем только у новой игры: при правке просьба пользователя
	// («сделай кнопками») важнее плана.
	planned := in.Spec
	if in.CurrentHTML != "" {
		planned = nil
	}
	var issues, earlier []domain.Issue
	reviewNeeded := planned != nil && a.cfg.Models.Reviewer != ""
	for round := 0; ; round++ {
		if err := ctx.Err(); err != nil {
			return BuildOutput{}, err
		}
		// ревьюер читает код параллельно с тестом в браузере: время сборки почти не растёт
		var reviewed chan []domain.Issue
		if reviewNeeded {
			reviewed = make(chan []domain.Issue, 1)
			go func(code string) { reviewed <- a.review(ctx, *planned, code, tr) }(html)
		}
		stage(domain.StatusValidating, "")
		issues = Validate(html, planned)

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
		if reviewed != nil {
			logic := <-reviewed
			issues = append(issues, logic...)
			// повторно проверяем только то, что ревьюер уже находил: исправлено ли
			reviewNeeded = domain.HasErrors(logic)
		}
		tr.Issues = withFixed(issues, earlier)

		if !domain.HasErrors(issues) || round >= a.cfg.MaxFixRounds {
			break
		}
		earlier = append(earlier, issues...)

		blocking := errorsOnly(issues)
		stage(domain.StatusFixing, fmt.Sprintf("круг %d: %s", round+1, issueCodes(blocking)))
		tr.FixRounds++
		fixed, ferr := a.fix(ctx, html, blocking, in.Progress, tr)
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
func (a *Agent) code(ctx context.Context, role, model string, msgs []llm.Msg, temp float64, w watcher, tr *Trace) (llm.Result, error) {
	var lastErr error
	for attempt := range 2 {
		raw, err := a.call(ctx, role, model, msgs, temp, 2, w, tr)
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

const maxReviewIssues = 5

// review — проверка логики кода по плану. Сбой ревьюера не мешает сборке: только предупреждение.
func (a *Agent) review(ctx context.Context, spec domain.GameSpec, code string, tr *Trace) []domain.Issue {
	raw, err := a.call(ctx, "reviewer", a.cfg.Models.Reviewer, llm.ReviewMessages(spec, code), 0.2, 0, nil, tr)
	var out struct {
		Issues []struct {
			Severity string `json:"severity"`
			Title    string `json:"title"`
			Details  string `json:"details"`
			Fix      string `json:"fix"`
		} `json:"issues"`
	}
	if err == nil {
		err = llm.ParseJSON(raw, &out)
	}
	if err != nil {
		return []domain.Issue{{Code: "review_unavailable", Severity: domain.SevWarn, Message: "logic review failed: " + clipMsg(err.Error())}}
	}
	var issues []domain.Issue
	for i, r := range out.Issues {
		if i == maxReviewIssues {
			break
		}
		sev := domain.SevWarn
		if strings.EqualFold(r.Severity, string(domain.SevError)) {
			sev = domain.SevError
		}
		msg := r.Title + ": " + r.Details
		if r.Fix != "" {
			msg += " Fix: " + r.Fix
		}
		issues = append(issues, domain.Issue{Code: fmt.Sprintf("logic_%d", i+1), Severity: sev, Message: clipMsg(msg)})
	}
	return issues
}

func clipMsg(s string) string {
	if r := []rune(s); len(r) > 600 {
		return string(r[:600]) + "…"
	}
	return s
}

// fixPatchChars — обычный объём правок исправителя, для полосы прогресса.
const fixPatchChars = 3000

// fix — круг исправления правками SEARCH/REPLACE; если они не легли на код,
// один запасной вызов возвращает файл целиком (роль fixer_full в телеметрии).
func (a *Agent) fix(ctx context.Context, html string, issues []domain.Issue, p Progress, tr *Trace) (llm.Result, error) {
	raw, err := a.call(ctx, "fixer", a.cfg.Models.Fixer, llm.FixMessages(html, issues), 0.2, 1,
		p.watch("fixer", fixPatchChars), tr)
	if err != nil {
		return llm.Result{}, err
	}
	res, perr := applyFix(html, raw)
	if perr == nil {
		return res, nil
	}
	return a.code(ctx, "fixer_full", a.cfg.Models.Fixer, llm.FixFullMessages(html, issues, perr.Error()), 0.2,
		p.watch("fixer", len(html)), tr)
}

func applyFix(html, raw string) (llm.Result, error) {
	if strings.Contains(raw, "<game>") {
		return parseGame(raw) // модель всё же прислала файл целиком — тоже годится
	}
	reply, edits, err := llm.ParseEdits(raw)
	if err != nil {
		return llm.Result{}, err
	}
	patched, err := llm.ApplyEdits(html, edits)
	if err != nil {
		return llm.Result{}, err
	}
	if !strings.Contains(strings.ToLower(patched), "</html>") {
		return llm.Result{}, errors.New("после правок документ оборван")
	}
	if reply == "" {
		reply = "Исправил найденные ошибки."
	}
	return checkGame(llm.Result{Reply: reply, HTML: patched})
}

func parseGame(raw string) (llm.Result, error) {
	res, err := llm.Parse(raw)
	if err != nil {
		return llm.Result{}, fmt.Errorf("%w: %w", apperr.ErrLLMFailed, err)
	}
	return checkGame(res)
}

func checkGame(res llm.Result) (llm.Result, error) {
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
