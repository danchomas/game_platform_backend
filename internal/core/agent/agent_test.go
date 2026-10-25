package agent

import (
	"context"
	"strings"
	"testing"

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
