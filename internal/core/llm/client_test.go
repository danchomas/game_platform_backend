package llm

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"beeline-games/internal/core/apperr"
)

func TestNormalizeBaseURL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"пусто — OpenRouter напрямую", "", DefaultBaseURL},
		{"только пробелы", "   ", DefaultBaseURL},
		{"базовый адрес как есть", "https://openrouter.ai/api/v1", "https://openrouter.ai/api/v1"},
		{"лишний слэш на конце", "https://openrouter.ai/api/v1/", "https://openrouter.ai/api/v1"},
		{"полный эндпоинт подрезается", "https://openrouter.ai/api/v1/chat/completions", "https://openrouter.ai/api/v1"},
		{"свой OpenAI-совместимый сервер", "http://ollama:11434/v1", "http://ollama:11434/v1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeBaseURL(tt.in); got != tt.want {
				t.Fatalf("normalizeBaseURL(%q) = %q, ожидалось %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestChatSendsExpectedRequest(t *testing.T) {
	var (
		gotPath   string
		gotAuth   string
		gotTitle  string
		gotReqRaw []byte
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotTitle = r.Header.Get("X-Title")
		gotReqRaw, _ = io.ReadAll(r.Body)

		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"привет"}}]}`)
	}))
	defer srv.Close()

	c := NewClient("test-key", "test/model", srv.URL+"/v1", time.Second)

	got, err := c.Chat(context.Background(), []Msg{{Role: "user", Content: "сделай змейку"}})
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if got != "привет" {
		t.Errorf("ответ = %q, ожидалось %q", got, "привет")
	}

	if gotPath != "/v1/chat/completions" {
		t.Errorf("путь = %q, ожидался %q", gotPath, "/v1/chat/completions")
	}
	if gotAuth != "Bearer test-key" {
		t.Errorf("заголовок авторизации = %q", gotAuth)
	}
	if gotTitle != appTitle {
		t.Errorf("атрибуция = %q, ожидалась %q", gotTitle, appTitle)
	}

	var req chatRequest
	if err := json.Unmarshal(gotReqRaw, &req); err != nil {
		t.Fatalf("тело запроса не разобрано: %v", err)
	}
	if req.Model != "test/model" {
		t.Errorf("модель в запросе = %q", req.Model)
	}
	if len(req.Messages) != 1 || req.Messages[0].Content != "сделай змейку" {
		t.Errorf("диалог в запросе потерян: %+v", req.Messages)
	}
}

func TestChatWithoutKey(t *testing.T) {
	c := NewClient("", "test/model", "", time.Second)

	_, err := c.Chat(context.Background(), []Msg{{Role: "user", Content: "привет"}})
	if !errors.Is(err, ErrNoKey) {
		t.Fatalf("ожидалась ErrNoKey, получено: %v", err)
	}
	if !errors.Is(err, apperr.ErrLLMUnavailable) {
		t.Error("ErrNoKey не разворачивается в ErrLLMUnavailable")
	}
}

func TestChatNonJSONResponseIsUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = io.WriteString(w, "<html><body>502 Bad Gateway</body></html>")
	}))
	defer srv.Close()

	c := NewClient("test-key", "test/model", srv.URL, time.Second)

	_, err := c.Chat(context.Background(), []Msg{{Role: "user", Content: "привет"}})
	if !errors.Is(err, apperr.ErrLLMUnavailable) {
		t.Fatalf("ожидалась ErrLLMUnavailable, получено: %v", err)
	}
	if !strings.Contains(err.Error(), "502") {
		t.Errorf("в ошибке нет статуса: %v", err)
	}
}

func TestChatUpstreamErrorBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":{"message":"No auth credentials found"}}`)
	}))
	defer srv.Close()

	c := NewClient("bad-key", "test/model", srv.URL, time.Second)

	_, err := c.Chat(context.Background(), []Msg{{Role: "user", Content: "привет"}})
	if !errors.Is(err, apperr.ErrLLMUnavailable) {
		t.Fatalf("ожидалась ErrLLMUnavailable, получено: %v", err)
	}
	if !strings.Contains(err.Error(), "No auth credentials found") {
		t.Errorf("сообщение шлюза потеряно: %v", err)
	}
}

func TestChatEmptyContentIsFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"   "},"finish_reason":"length"}]}`)
	}))
	defer srv.Close()

	c := NewClient("test-key", "test/model", srv.URL, time.Second)

	_, err := c.Chat(context.Background(), []Msg{{Role: "user", Content: "привет"}})
	if !errors.Is(err, apperr.ErrLLMFailed) {
		t.Fatalf("ожидалась ErrLLMFailed, получено: %v", err)
	}
	if !strings.Contains(err.Error(), "length") {
		t.Errorf("причина обрыва потеряна: %v", err)
	}
}

func TestChatUnreachableHost(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	addr := srv.URL
	srv.Close()

	c := NewClient("test-key", "test/model", addr, 500*time.Millisecond)

	_, err := c.Chat(context.Background(), []Msg{{Role: "user", Content: "привет"}})
	if !errors.Is(err, apperr.ErrLLMUnavailable) {
		t.Fatalf("ожидалась ErrLLMUnavailable, получено: %v", err)
	}
}

func TestChatPropagatesCancellation(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		<-block
	}))
	defer srv.Close()
	defer close(block)

	c := NewClient("test-key", "test/model", srv.URL, time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.Chat(ctx, []Msg{{Role: "user", Content: "привет"}})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ожидалась context.Canceled, получено: %v", err)
	}
}

func TestCompleteSendsReasoningBudget(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"ok"},"finish_reason":"stop"}]}`)
	}))
	defer srv.Close()

	c := NewClient("test-key", "test/model", srv.URL, time.Second)
	if _, err := c.Complete(context.Background(), Request{Messages: []Msg{{Role: "user", Content: "x"}}, Reasoning: ReasoningLow}); err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	r, _ := got["reasoning"].(map[string]any)
	if r["effort"] != "low" || r["exclude"] != true {
		t.Errorf("reasoning в запросе = %v, ожидался effort=low, exclude=true", got["reasoning"])
	}

	if _, err := c.Complete(context.Background(), Request{Messages: []Msg{{Role: "user", Content: "x"}}, Reasoning: ReasoningOff}); err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if r, _ := got["reasoning"].(map[string]any); r["enabled"] != false {
		t.Errorf("reasoning=off должен выключать рассуждения, получено %v", got["reasoning"])
	}
}

func TestEmptyTruncatedKeepsUsage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"model":"m","choices":[{"message":{"content":""},"finish_reason":"length"}],"usage":{"prompt_tokens":10,"completion_tokens":32000,"cost":0.02}}`)
	}))
	defer srv.Close()

	c := NewClient("test-key", "test/model", srv.URL, time.Second)
	resp, err := c.Complete(context.Background(), Request{Messages: []Msg{{Role: "user", Content: "x"}}})
	if !errors.Is(err, ErrEmptyTruncated) || !errors.Is(err, apperr.ErrLLMFailed) {
		t.Fatalf("ожидались ErrEmptyTruncated и ErrLLMFailed, получено: %v", err)
	}
	if resp.Usage.CompletionTokens != 32000 || resp.FinishReason != "length" {
		t.Errorf("usage при пустом ответе потерян: %+v", resp)
	}
}

func TestUsageReportsReasoningTokens(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":900,"completion_tokens_details":{"reasoning_tokens":700}}}`)
	}))
	defer srv.Close()

	resp, err := NewClient("k", "m", srv.URL, time.Second).Complete(context.Background(), Request{})
	if err != nil || resp.Usage.ReasoningTokens != 700 || resp.Usage.CompletionTokens != 900 {
		t.Fatalf("reasoning_tokens не разобран: %+v, %v", resp.Usage, err)
	}
}

func TestCompleteStreamsDeltas(t *testing.T) {
	var gotReq chatRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotReq)
		w.Header().Set("Content-Type", "text/event-stream")
		for _, ev := range []string{
			": OPENROUTER PROCESSING",
			`data: {"model":"m/real","choices":[{"delta":{"content":"<game>при"}}]}`,
			`data: {"choices":[{"delta":{"content":"вет</game>"},"finish_reason":"stop"}]}`,
			`data: {"choices":[],"usage":{"prompt_tokens":10,"completion_tokens":5,"cost":0.002,"completion_tokens_details":{"reasoning_tokens":2}}}`,
			"data: [DONE]",
		} {
			_, _ = io.WriteString(w, ev+"\n\n")
		}
	}))
	defer srv.Close()

	var deltas []string
	resp, err := NewClient("k", "m", srv.URL, time.Second).Complete(context.Background(),
		Request{Messages: []Msg{{Role: "user", Content: "x"}}, OnDelta: func(d string) { deltas = append(deltas, d) }})
	if err != nil {
		t.Fatal(err)
	}
	if !gotReq.Stream {
		t.Error("с OnDelta запрос должен быть потоковым")
	}
	if resp.Text != "<game>привет</game>" || strings.Join(deltas, "|") != "<game>при|вет</game>" {
		t.Errorf("текст %q, куски %q", resp.Text, deltas)
	}
	if resp.FinishReason != "stop" || resp.Model != "m/real" || resp.Usage.CostUSD != 0.002 || resp.Usage.ReasoningTokens != 2 {
		t.Errorf("метаданные потока потеряны: %+v", resp)
	}
}

func TestStreamGatewayErrorIsRetriedBeforeAnyDelta(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = io.WriteString(w, `{"error":{"message":"busy"}}`)
			return
		}
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
	}))
	defer srv.Close()

	var got strings.Builder
	resp, err := NewClient("k", "m", srv.URL, time.Second).Complete(context.Background(),
		Request{Messages: []Msg{{Role: "user", Content: "x"}}, OnDelta: func(d string) { got.WriteString(d) }})
	if err != nil || resp.Text != "ok" || got.String() != "ok" || calls != 2 {
		t.Fatalf("calls=%d text=%q deltas=%q err=%v", calls, resp.Text, got.String(), err)
	}
}

func TestStreamErrorMidwayIsNotRetried(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"half\"}}]}\n\ndata: {\"error\":{\"message\":\"upstream died\"}}\n\n")
	}))
	defer srv.Close()

	_, err := NewClient("k", "m", srv.URL, time.Second).Complete(context.Background(),
		Request{Messages: []Msg{{Role: "user", Content: "x"}}, OnDelta: func(string) {}})
	if !errors.Is(err, apperr.ErrLLMUnavailable) || calls != 1 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
}
