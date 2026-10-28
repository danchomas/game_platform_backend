package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"beeline-games/internal/core/apperr"
	"beeline-games/internal/core/domain"
	"beeline-games/internal/core/gamekit"
	"beeline-games/internal/core/llm"
)

const goodGame = `<!DOCTYPE html><html><head><meta name="viewport" content="width=device-width"></head><body>
<script>
const {ctx} = GameKit.setup({background:'#000'});
GameKit.on('action', () => { window.GameSDK && GameSDK.start(); });
function over(){ window.GameSDK && GameSDK.end('win', 10); }
function loop(){ ctx.fillRect(0,0,10,10); requestAnimationFrame(loop); } loop();
</script></body></html>`

func wrap(html string) string {
	return `<reply>Готово</reply><meta>{"title":"Змейка","genre":"snake","audience":"бабушки"}</meta><game>` + html + `</game>`
}

type fakeLLM struct {
	answers []string
	roles   []string
}

func (f *fakeLLM) Complete(_ context.Context, r llm.Request) (llm.Response, error) {
	f.roles = append(f.roles, r.Model)
	a := f.answers[0]
	f.answers = f.answers[1:]
	return llm.Response{Text: a, FinishReason: "stop", Model: r.Model, Usage: llm.Usage{CostUSD: 0.01}}, nil
}

type fakeTester struct{ reports []TestReport }

func (f *fakeTester) Test(_ context.Context, html string) (TestReport, error) {
	r := f.reports[0]
	if len(f.reports) > 1 {
		f.reports = f.reports[1:]
	}
	return r, nil
}

var okReport = TestReport{StartCalled: true, FramesPlay: 30, HasCanvas: true, ChangedOnInput: true}

func newAgent(l llm.Provider, t Tester) *Agent {
	return New(l, t, Config{Models: Models{Planner: "planner", Coder: "coder", Fixer: "fixer"}, MaxFixRounds: 2})
}

func TestValidateGoodGame(t *testing.T) {
	if issues := Validate(goodGame, nil); domain.HasErrors(issues) {
		t.Fatalf("good game flagged: %+v", issues)
	}
}

func TestValidateCatchesProblems(t *testing.T) {
	bad := `<html><head><style>@import "x.css";</style></head><body><script>fetch('/x'); alert('hi')</script></body></html>`
	codes := map[string]bool{}
	for _, i := range Validate(bad, nil) {
		codes[i.Code] = true
	}
	for _, c := range []string{"external_resource", "network_call", "blocking_dialog", "no_sdk_start", "no_sdk_end", "no_input"} {
		if !codes[c] {
			t.Errorf("missing %s in %v", c, codes)
		}
	}
	if Validate(`obj.confirm(1); GameSDK.start(); GameSDK.end(); GameKit.on('tap',f); GameKit.setup()`, nil) != nil {
		t.Error("method named confirm must not be treated as a blocking dialog")
	}
}

func TestUnwantedButtonsAreSentToFixer(t *testing.T) {
	withButtons := strings.Replace(goodGame, "GameKit.on(", "GameKit.buttons('dpad'); GameKit.on(", 1)
	swipe := &domain.GameSpec{Controls: domain.Controls{Scheme: "swipe"}}

	l := &fakeLLM{answers: []string{wrap(withButtons), wrap(goodGame)}}
	tr := &Trace{}
	if _, err := newAgent(l, nil).Build(context.Background(), BuildInput{Prompt: "тетрис", Spec: swipe}, nil, tr); err != nil {
		t.Fatal(err)
	}
	if tr.FixRounds != 1 {
		t.Fatalf("buttons in a gesture game must be fixed, rounds=%d issues=%+v", tr.FixRounds, tr.Issues)
	}

	// при правке просьба пользователя важнее плана: кнопки по его просьбе не убираем
	l = &fakeLLM{answers: []string{wrap(withButtons)}}
	tr = &Trace{}
	in := BuildInput{Prompt: "тетрис", Spec: swipe, CurrentHTML: goodGame, Instruction: "добавь кнопки"}
	if _, err := newAgent(l, nil).Build(context.Background(), in, nil, tr); err != nil {
		t.Fatal(err)
	}
	if tr.FixRounds != 0 {
		t.Fatalf("edit must not be checked against the plan: %+v", tr.Issues)
	}

	if issues := Validate(`if (typeof GameKit.buttons === 'function') {}`+goodGame, swipe); domain.HasErrors(issues) {
		t.Fatalf("a feature check is not a button call: %+v", issues)
	}
}

func TestPlanDropsButtonsFromGestureSchemes(t *testing.T) {
	l := &fakeLLM{answers: []string{`{"title":"Тетрис","controls":{"scheme":"mixed","gestures":["swipe","tap"],"buttons":["left","right"]}}`}}
	spec, err := newAgent(l, nil).Plan(context.Background(), PlanInput{Prompt: "тетрис"}, &Trace{})
	if err != nil {
		t.Fatal(err)
	}
	if len(spec.Controls.Buttons) != 0 {
		t.Fatalf("buttons survived a gesture scheme: %+v", spec.Controls)
	}
}

func TestPlanNormalizes(t *testing.T) {
	l := &fakeLLM{answers: []string{"```json\n{\"title\":\"Змейка\",\"controls\":{\"scheme\":\"weird\"},\"ui\":{\"scale\":\"xlarge\"}}\n```"}}
	spec, err := newAgent(l, nil).Plan(context.Background(), PlanInput{Prompt: "змейка для бабушки"}, &Trace{})
	if err != nil {
		t.Fatal(err)
	}
	if spec.Controls.Scheme != "tap" || spec.UI.MinTouchPx < 96 || spec.Summary == "" {
		t.Fatalf("not normalized: %+v", spec)
	}
}

func TestBuildFixesRuntimeErrors(t *testing.T) {
	l := &fakeLLM{answers: []string{wrap(goodGame), wrap(goodGame)}}
	tester := &fakeTester{reports: []TestReport{{Errors: []string{"ReferenceError: x is not defined"}}, okReport}}
	tr := &Trace{}
	var stages []string
	out, err := newAgent(l, tester).Build(context.Background(),
		BuildInput{Prompt: "змейка", Spec: &domain.GameSpec{Title: "Змейка"}},
		func(s, _ string) { stages = append(stages, s) }, tr)
	if err != nil {
		t.Fatal(err)
	}
	if tr.FixRounds != 1 || domain.HasErrors(out.Issues) {
		t.Fatalf("expected one successful fix round, got %d rounds, issues %+v", tr.FixRounds, out.Issues)
	}
	if strings.Join(l.roles, ",") != "coder,fixer" {
		t.Fatalf("unexpected model calls: %v", l.roles)
	}
	if !strings.Contains(out.HTML, `data-gamekit="1"`) {
		t.Fatal("GameKit must be embedded into the saved game")
	}
	if len(tr.Calls) != 2 || tr.Usage().CostUSD < 0.019 {
		t.Fatalf("calls not traced: %+v", tr.Calls)
	}
	if !strings.Contains(strings.Join(stages, ","), domain.StatusFixing) {
		t.Fatalf("fixing stage not reported: %v", stages)
	}
	fixed := map[string]bool{}
	for _, i := range tr.Issues {
		fixed[i.Code] = i.Fixed
	}
	if !fixed["runtime_error"] || !fixed["not_started"] || domain.HasErrors(tr.Issues) {
		t.Fatalf("fixed issues must stay in the trace for statistics: %v", fixed)
	}
}

func TestBuildGivesUpAfterMaxRounds(t *testing.T) {
	l := &fakeLLM{answers: []string{wrap(goodGame), wrap(goodGame), wrap(goodGame)}}
	tester := &fakeTester{reports: []TestReport{{StartCalled: false}}}
	tr := &Trace{}
	out, err := newAgent(l, tester).Build(context.Background(),
		BuildInput{Prompt: "x", Spec: &domain.GameSpec{}}, nil, tr)
	if err != nil {
		t.Fatal(err)
	}
	if tr.FixRounds != 2 || !domain.HasErrors(out.Issues) {
		t.Fatalf("rounds=%d issues=%+v", tr.FixRounds, out.Issues)
	}
}

func TestEditSendsCodeWithoutGameKit(t *testing.T) {
	var sent string
	l := &capture{fn: func(r llm.Request) { sent = r.Messages[len(r.Messages)-1].Content }, answer: wrap(goodGame)}
	_, err := newAgent(l, nil).Build(context.Background(),
		BuildInput{CurrentHTML: gamekit.Embed(goodGame), Instruction: "сделай кнопки больше"}, nil, &Trace{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(sent, "data-gamekit") || !strings.Contains(sent, "сделай кнопки больше") {
		t.Fatal("edit prompt must contain the change request and the code without GameKit")
	}
}

type capture struct {
	fn     func(llm.Request)
	answer string
}

func (c *capture) Complete(_ context.Context, r llm.Request) (llm.Response, error) {
	c.fn(r)
	return llm.Response{Text: c.answer, FinishReason: "stop"}, nil
}

type reasoningLLM struct{ reqs []llm.Request }

func (r *reasoningLLM) Complete(_ context.Context, req llm.Request) (llm.Response, error) {
	r.reqs = append(r.reqs, req)
	if len(r.reqs) == 1 {
		return llm.Response{FinishReason: "length", Usage: llm.Usage{CompletionTokens: 32000}},
			fmt.Errorf("%w: %w", apperr.ErrLLMFailed, llm.ErrEmptyTruncated)
	}
	return llm.Response{Text: `{"title":"Змейка"}`, FinishReason: "stop"}, nil
}

func TestCallRetriesWithoutReasoningWhenBudgetIsExhausted(t *testing.T) {
	l := &reasoningLLM{}
	a := New(l, nil, Config{Models: Models{Planner: "p", Coder: "c"}, Reasoning: llm.ReasoningLow})
	tr := &Trace{}

	if _, err := a.Plan(context.Background(), PlanInput{Prompt: "змейка"}, tr); err != nil {
		t.Fatalf("план после повтора: %v", err)
	}
	if len(l.reqs) != 2 || l.reqs[0].Reasoning != llm.ReasoningLow || l.reqs[1].Reasoning != llm.ReasoningOff {
		t.Fatalf("ожидался повтор с reasoning=off: %+v", l.reqs)
	}
	if len(tr.Calls) != 2 || tr.Calls[0].CompletionTokens != 32000 || tr.Calls[0].Error == "" {
		t.Errorf("неудачная попытка должна остаться в телеметрии с токенами: %+v", tr.Calls)
	}
}

func callRoles(tr *Trace) string {
	roles := make([]string, 0, len(tr.Calls))
	for _, c := range tr.Calls {
		roles = append(roles, c.Role)
	}
	return strings.Join(roles, ",")
}

func TestFixerSendsEditsInsteadOfWholeFile(t *testing.T) {
	broken := strings.Replace(goodGame, "function over()", "brokenCall();\nfunction over()", 1)
	patch := "<reply>Убрал лишний вызов</reply>\n<patch>\n<<<<<<< SEARCH\nbrokenCall();\n=======\n>>>>>>> REPLACE\n</patch>"
	l := &fakeLLM{answers: []string{wrap(broken), patch}}
	tester := &fakeTester{reports: []TestReport{{Errors: []string{"ReferenceError: brokenCall is not defined"}, StartCalled: true, FramesPlay: 30, ChangedOnInput: true}, okReport}}
	tr := &Trace{}
	out, err := newAgent(l, tester).Build(context.Background(), BuildInput{Prompt: "змейка", Spec: &domain.GameSpec{}}, nil, tr)
	if err != nil {
		t.Fatal(err)
	}
	if callRoles(tr) != "coder,fixer" || strings.Contains(out.HTML, "brokenCall") || domain.HasErrors(out.Issues) {
		t.Fatalf("roles=%s issues=%+v", callRoles(tr), out.Issues)
	}
	if !strings.Contains(out.HTML, "function over()") {
		t.Fatal("the rest of the game must stay as it was")
	}
}

func TestFixerFallsBackToWholeFileWhenEditsDoNotApply(t *testing.T) {
	bad := "<reply>x</reply><patch>\n<<<<<<< SEARCH\nthis line is not in the game\n=======\nfoo();\n>>>>>>> REPLACE\n</patch>"
	l := &fakeLLM{answers: []string{wrap(goodGame), bad, wrap(goodGame)}}
	tester := &fakeTester{reports: []TestReport{{Errors: []string{"TypeError: x"}, StartCalled: true, FramesPlay: 30, ChangedOnInput: true}, okReport}}
	tr := &Trace{}
	if _, err := newAgent(l, tester).Build(context.Background(), BuildInput{Prompt: "змейка", Spec: &domain.GameSpec{}}, nil, tr); err != nil {
		t.Fatal(err)
	}
	if callRoles(tr) != "coder,fixer,fixer_full" {
		t.Fatalf("roles=%s", callRoles(tr))
	}
}

func TestIncompleteBrowserTestDoesNotBlameTheGame(t *testing.T) {
	// так выглядел отчёт, когда тест упирался в таймаут: кадры «не шли», хотя игра работала
	rep := TestReport{StartCalled: true, FramesPlay: 0, Incomplete: "context deadline exceeded"}
	codes := map[string]domain.Severity{}
	for _, i := range rep.Issues() {
		codes[i.Code] = i.Severity
	}
	if codes["test_incomplete"] != domain.SevWarn || len(codes) != 1 {
		t.Fatalf("only a warning about the test is expected, got %v", codes)
	}
	if issues := (TestReport{StartCalled: true, FramesPlay: 30, ScreensCompared: false}).Issues(); len(issues) != 0 {
		t.Fatalf("missing screenshots must not mean a frozen screen: %+v", issues)
	}
}

// scriptLLM — ответы по порядку; запоминает бюджет рассуждений по модели и отдаёт запросы в fn.
type scriptLLM struct {
	answers   []string
	fn        func(llm.Request)
	reasoning map[string]string
}

func (s *scriptLLM) Complete(_ context.Context, r llm.Request) (llm.Response, error) {
	if s.fn != nil {
		s.fn(r)
	}
	if s.reasoning != nil {
		s.reasoning[r.Model] = r.Reasoning
	}
	a := s.answers[0]
	s.answers = s.answers[1:]
	return llm.Response{Text: a, FinishReason: "stop", Model: r.Model}, nil
}

func reviewAgent(l llm.Provider, t Tester) *Agent {
	return New(l, t, Config{
		Models:       Models{Planner: "planner", Coder: "coder", Fixer: "fixer", Reviewer: "reviewer"},
		MaxFixRounds: 2, Reasoning: llm.ReasoningOff, PlannerReasoning: llm.ReasoningMedium, ReviewerReasoning: llm.ReasoningLow,
	})
}

const lanesInSideView = `{"issues":[{"severity":"error","title":"Полосы при виде сбоку","details":"draw() uses lane = 0..2 while the plan says side view","fix":"remove lanes, move along X"},{"severity":"warn","title":"баланс","details":"x","fix":"y"}]}`

func TestBuildDoesNotWaitForTheReviewer(t *testing.T) {
	l := &scriptLLM{answers: []string{wrap(goodGame)}}
	tr := &Trace{}
	if _, err := reviewAgent(l, &fakeTester{reports: []TestReport{okReport}}).Build(context.Background(),
		BuildInput{Prompt: "x", Spec: &domain.GameSpec{}}, nil, tr); err != nil {
		t.Fatal(err)
	}
	if callRoles(tr) != "coder" {
		t.Fatalf("the reviewer runs in the background after the build, got %s", callRoles(tr))
	}
}

func TestReviewReturnsLogicIssues(t *testing.T) {
	var sent string
	l := &scriptLLM{answers: []string{lanesInSideView}, fn: func(r llm.Request) { sent = r.Messages[1].Content }}
	issues, err := reviewAgent(l, nil).Review(context.Background(), domain.GameSpec{Camera: domain.Camera{View: "side"}}, gamekit.Embed(goodGame), &Trace{})
	if err != nil || len(issues) != 2 {
		t.Fatalf("issues=%+v err=%v", issues, err)
	}
	if issues[0].Code != "logic_1" || issues[0].Severity != domain.SevError || issues[1].Severity != domain.SevWarn {
		t.Fatalf("unexpected issues: %+v", issues)
	}
	if strings.Contains(sent, `data-gamekit`) || !strings.Contains(sent, `"view": "side"`) {
		t.Fatal("the reviewer must get the plan and the code without GameKit")
	}
	if _, err := reviewAgent(&scriptLLM{answers: []string{"not json"}}, nil).Review(context.Background(), domain.GameSpec{}, goodGame, &Trace{}); err == nil {
		t.Fatal("a broken reviewer answer must be an error")
	}
}

func TestImproveKeepsTheGameOnlyIfItStillWorks(t *testing.T) {
	issues := []domain.Issue{{Code: "logic_1", Severity: domain.SevError, Message: "x"}}
	tr := &Trace{}
	out, err := reviewAgent(&scriptLLM{answers: []string{wrap(goodGame)}}, &fakeTester{reports: []TestReport{okReport}}).
		Improve(context.Background(), gamekit.Embed(goodGame), issues, tr)
	if err != nil || !strings.Contains(out.HTML, `data-gamekit="1"`) || callRoles(tr) != "fixer" {
		t.Fatalf("err=%v roles=%s", err, callRoles(tr))
	}

	broken := TestReport{Errors: []string{"TypeError: x"}, StartCalled: true, FramesPlay: 30}
	_, err = reviewAgent(&scriptLLM{answers: []string{wrap(goodGame)}}, &fakeTester{reports: []TestReport{broken}}).
		Improve(context.Background(), goodGame, issues, &Trace{})
	if !errors.Is(err, ErrNotImproved) {
		t.Fatalf("a fix that breaks the game must be dropped, got %v", err)
	}
}

func TestEachRoleGetsItsOwnReasoning(t *testing.T) {
	l := &scriptLLM{answers: []string{`{"title":"x"}`, wrap(goodGame), `{"issues":[]}`}, reasoning: map[string]string{}}
	a := reviewAgent(l, nil)
	spec, err := a.Plan(context.Background(), PlanInput{Prompt: "x"}, &Trace{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Build(context.Background(), BuildInput{Prompt: "x", Spec: &spec}, nil, &Trace{}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Review(context.Background(), spec, goodGame, &Trace{}); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"planner": llm.ReasoningMedium, "coder": llm.ReasoningOff, "reviewer": llm.ReasoningLow}
	for role, r := range want {
		if l.reasoning[role] != r {
			t.Errorf("%s reasoning = %q, want %q", role, l.reasoning[role], r)
		}
	}
}
