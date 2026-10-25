package llm

import (
	"context"
	"testing"
	"time"
)

type scripted struct {
	parts []Response
	reqs  []Request
}

func (s *scripted) Complete(_ context.Context, r Request) (Response, error) {
	s.reqs = append(s.reqs, r)
	resp := s.parts[0]
	s.parts = s.parts[1:]
	return resp, nil
}

func TestCompleteFullStitchesTruncated(t *testing.T) {
	p := &scripted{parts: []Response{
		{Text: "<game><html>", FinishReason: "length", Usage: Usage{CompletionTokens: 10, CostUSD: 0.01}},
		{Text: "</html></game>", FinishReason: "stop", Usage: Usage{CompletionTokens: 5, CostUSD: 0.005}},
	}}
	r, err := CompleteFull(context.Background(), p, Request{Messages: []Msg{{Role: "user", Content: "x"}}}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if r.Text != "<game><html></html></game>" || r.Usage.CompletionTokens != 15 || r.FinishReason != "stop" {
		t.Fatalf("bad stitch: %+v", r)
	}
	last := p.reqs[1].Messages
	if len(last) != 3 || last[1].Role != "assistant" || last[1].Content != "<game><html>" {
		t.Fatalf("continuation request should carry the partial answer: %+v", last)
	}
}

func TestCompleteFullStopsAfterLimit(t *testing.T) {
	p := &scripted{parts: []Response{{Text: "a", FinishReason: "length"}, {Text: "b", FinishReason: "length"}}}
	r, _ := CompleteFull(context.Background(), p, Request{}, 1)
	if r.Text != "ab" || len(p.reqs) != 2 {
		t.Fatalf("got %q after %d calls", r.Text, len(p.reqs))
	}
}

func TestParseJSONWithFenceAndProse(t *testing.T) {
	var v struct{ Title string }
	if err := ParseJSON("Вот план:\n```json\n{\"title\":\"Змейка\"}\n```", &v); err != nil || v.Title != "Змейка" {
		t.Fatalf("%+v %v", v, err)
	}
	if err := ParseJSON("нет json", &v); err == nil {
		t.Fatal("expected error")
	}
}

type slowProvider struct{ calls int }

func (s *slowProvider) Complete(ctx context.Context, _ Request) (Response, error) {
	s.calls++
	select {
	case <-time.After(60 * time.Millisecond):
	case <-ctx.Done():
		return Response{}, ctx.Err()
	}
	if s.calls == 1 {
		return Response{Text: "a", FinishReason: "length"}, nil
	}
	return Response{Text: "b", FinishReason: "stop"}, nil
}

func TestCallTimeoutAppliesToEachPart(t *testing.T) {
	// каждая часть идёт 60 мс при таймауте 100 мс: вместе 120 мс — общий таймаут бы оборвал продолжение
	p := &slowProvider{}
	r, err := CompleteFull(context.Background(), WithCallTimeout(p, 100*time.Millisecond), Request{}, 2)
	if err != nil || r.Text != "ab" {
		t.Fatalf("продолжение должно уложиться в свой таймаут: %q, %v", r.Text, err)
	}
}
