package agent

import (
	"context"
	"strings"
	"testing"

	"beeline-games/internal/core/domain"
	"beeline-games/internal/core/llm"
)

func TestStepTrackerCatchesMarkersSplitAcrossChunks(t *testing.T) {
	tr := &stepTracker{}
	var steps []string
	for _, d := range []string{"<style>/* @st", "ep Оформление */\nbody{}", "</style><script>\n// @step Рисуем ", "поле\nlet x;\n"} {
		if tr.feed(d) {
			steps = append(steps, tr.step)
		}
	}
	if strings.Join(steps, "|") != "Оформление|Рисуем поле" {
		t.Fatalf("метки: %q", steps)
	}
	if tr.chars != len("<style>/* @step Оформление */\nbody{}</style><script>\n// @step Рисуем поле\nlet x;\n") {
		t.Errorf("chars = %d", tr.chars)
	}
	if stepOf("const step = 1;") != "" || stepOf("<!-- @step Разметка -->") != "Разметка" {
		t.Error("stepOf: лишняя метка или не распознан HTML-комментарий")
	}
}

// streamLLM отдаёт ответ кусками через OnDelta, как потоковый провайдер.
type streamLLM struct{ answer string }

func (s *streamLLM) Complete(_ context.Context, r llm.Request) (llm.Response, error) {
	if r.OnDelta != nil {
		for i := 0; i < len(s.answer); i += 40 {
			r.OnDelta(s.answer[i:min(i+40, len(s.answer))])
		}
	}
	return llm.Response{Text: s.answer, FinishReason: "stop"}, nil
}

func TestBuildReportsCodingProgress(t *testing.T) {
	game := strings.Replace(goodGame, "<script>", "<script>\n// @step Рисуем поле\n", 1)
	type tick struct {
		role            string
		chars, expected int
		step            string
	}
	var ticks []tick
	progress := func(role string, chars, expected int, step string) {
		ticks = append(ticks, tick{role, chars, expected, step})
	}

	out, err := newAgent(&streamLLM{answer: wrap(game)}, nil).Build(context.Background(),
		BuildInput{Prompt: "змейка", Spec: &domain.GameSpec{}, Progress: progress, ExpectedChars: 5000}, nil, &Trace{})
	if err != nil || domain.HasErrors(out.Issues) {
		t.Fatalf("err=%v issues=%+v", err, out.Issues)
	}
	if len(ticks) < 2 || ticks[0] != (tick{"coder", 0, 5000, ""}) {
		t.Fatalf("первым должен прийти старт кодера: %+v", ticks)
	}
	last := ticks[len(ticks)-1]
	if last.step != "Рисуем поле" || last.chars == 0 {
		t.Errorf("метка или объём не дошли: %+v", last)
	}

	// правка: ожидаемый объём — размер текущего кода
	ticks = nil
	_, err = newAgent(&streamLLM{answer: wrap(game)}, nil).Build(context.Background(),
		BuildInput{Prompt: "змейка", CurrentHTML: goodGame, Instruction: "медленнее", Progress: progress}, nil, &Trace{})
	if err != nil || len(ticks) == 0 || ticks[0].expected != len(goodGame) {
		t.Fatalf("err=%v ticks=%+v", err, ticks)
	}
}
