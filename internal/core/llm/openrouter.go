package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"beeline-games/internal/core/apperr"
)

const DefaultBaseURL = "https://openrouter.ai/api/v1"

const chatPath = "/chat/completions"

const (
	// maxTokens — по умолчанию для Chat; агент передаёт свой лимит (LLM_MAX_TOKENS).
	maxTokens   = 8000
	temperature = 0.7

	retries    = 1
	retryDelay = 1500 * time.Millisecond

	maxResponseBytes = 8 << 20

	maxErrorSnippet = 300
)

const (
	appTitle = "Beeline Vibe Games"
	appURL   = "https://github.com/beeline-hackathon/vibe-games"
)

type Msg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Client struct {
	http    *http.Client
	baseURL string
	apiKey  string
	model   string
}

func NewClient(apiKey, model, baseURL string, timeout time.Duration) *Client {
	return &Client{
		http:    &http.Client{Timeout: timeout},
		baseURL: normalizeBaseURL(baseURL),
		apiKey:  apiKey,
		model:   model,
	}
}

func normalizeBaseURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return DefaultBaseURL
	}

	raw = strings.TrimRight(raw, "/")
	return strings.TrimSuffix(raw, chatPath)
}

func (c *Client) BaseURL() string { return c.baseURL }

func (c *Client) Model() string { return c.model }

var ErrNoKey = fmt.Errorf("%w: OPENROUTER_API_KEY не задан", apperr.ErrLLMUnavailable)

type chatRequest struct {
	Model       string         `json:"model"`
	Models      []string       `json:"models,omitempty"`
	Messages    []Msg          `json:"messages"`
	MaxTokens   int            `json:"max_tokens"`
	Temperature float64        `json:"temperature"`
	Usage       map[string]any `json:"usage,omitempty"`
	Reasoning   map[string]any `json:"reasoning,omitempty"`
	Stream      bool           `json:"stream,omitempty"`
}

// reasoningParam — параметр reasoning OpenRouter. Текст рассуждений не запрашиваем (exclude):
// агенту нужен только ответ, а токены рассуждений всё равно оплачиваются и видны в usage.
func reasoningParam(level string) map[string]any {
	switch level {
	case ReasoningOff:
		return map[string]any{"enabled": false}
	case ReasoningLow, ReasoningMedium, ReasoningHigh:
		return map[string]any{"effort": level, "exclude": true}
	default:
		return nil
	}
}

type chatResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		// Delta — кусок ответа в потоковом режиме
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage *chatUsage `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Code    any    `json:"code"`
	} `json:"error"`
}

type chatUsage struct {
	PromptTokens            int     `json:"prompt_tokens"`
	CompletionTokens        int     `json:"completion_tokens"`
	Cost                    float64 `json:"cost"`
	CompletionTokensDetails *struct {
		ReasoningTokens int `json:"reasoning_tokens"`
	} `json:"completion_tokens_details"`
}

func (u *chatUsage) toUsage() Usage {
	if u == nil {
		return Usage{}
	}
	out := Usage{PromptTokens: u.PromptTokens, CompletionTokens: u.CompletionTokens, CostUSD: u.Cost}
	if d := u.CompletionTokensDetails; d != nil {
		out.ReasoningTokens = d.ReasoningTokens
	}
	return out
}

// Chat — простой вызов модели клиента по умолчанию.
func (c *Client) Chat(ctx context.Context, msgs []Msg) (string, error) {
	resp, err := c.Complete(ctx, Request{Messages: msgs})
	return resp.Text, err
}

// Complete — вызов с выбором модели, запасными моделями (поле models OpenRouter),
// учётом стоимости (usage.include) и повтором при 429/5xx.
func (c *Client) Complete(ctx context.Context, req Request) (Response, error) {
	if c.apiKey == "" {
		return Response{}, ErrNoKey
	}

	model := req.Model
	if model == "" {
		model = c.model
	}
	if req.MaxTokens <= 0 {
		req.MaxTokens = maxTokens
	}
	if req.Temperature == 0 {
		req.Temperature = temperature
	}

	payload := chatRequest{
		Model:       model,
		Messages:    req.Messages,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Usage:       map[string]any{"include": true},
		Reasoning:   reasoningParam(req.Reasoning),
		Stream:      req.OnDelta != nil,
	}
	if len(req.Fallbacks) > 0 {
		payload.Models = append([]string{model}, req.Fallbacks...)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return Response{}, fmt.Errorf("сборка запроса к модели: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt <= retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return Response{}, ctx.Err()
			case <-time.After(time.Duration(attempt) * retryDelay):
			}
		}
		resp, err := c.do(ctx, body, req.OnDelta)
		if err == nil {
			if resp.Model == "" {
				resp.Model = model
			}
			return resp, nil
		}
		lastErr = err
		var se *statusError
		if !errors.As(err, &se) || !se.retryable() {
			// usage и finish_reason отдаём и при ошибке: по ним видно, куда ушли токены
			if resp.Model == "" {
				resp.Model = model
			}
			return resp, err
		}
	}
	return Response{}, lastErr
}

func (c *Client) do(ctx context.Context, body []byte, onDelta func(string)) (Response, error) {
	start := time.Now()
	endpoint := c.baseURL + chatPath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return Response{}, fmt.Errorf("%w: некорректный адрес %q: %v", apperr.ErrLLMUnavailable, endpoint, err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("HTTP-Referer", appURL)
	req.Header.Set("X-Title", appTitle)

	resp, err := c.http.Do(req)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return Response{}, err
		}
		return Response{}, fmt.Errorf("%w: запрос к %s не прошёл: %v", apperr.ErrLLMUnavailable, c.baseURL, err)
	}
	defer resp.Body.Close()

	// ошибки шлюза приходят обычным JSON и в потоковом режиме
	if onDelta != nil && resp.StatusCode == http.StatusOK {
		out, err := readStream(resp.Body, onDelta)
		out.Latency = time.Since(start)
		return out, err
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return Response{}, fmt.Errorf("%w: ответ модели не дочитан: %v", apperr.ErrLLMUnavailable, err)
	}

	out, err := parseChatResponse(resp.StatusCode, raw)
	out.Latency = time.Since(start)
	return out, err
}

// statusError — ошибка шлюза с HTTP-статусом: по нему решаем, повторять ли запрос.
type statusError struct {
	status int
	err    error
}

func (e *statusError) Error() string { return e.err.Error() }

func (e *statusError) Unwrap() error { return e.err }

func (e *statusError) retryable() bool {
	return e.status == http.StatusTooManyRequests || e.status == http.StatusRequestTimeout || e.status >= 500
}

func parseChatResponse(status int, raw []byte) (Response, error) {
	var out chatResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return Response{}, &statusError{status, fmt.Errorf("%w: HTTP %d, ответ не JSON: %s",
			apperr.ErrLLMUnavailable, status, snippet(raw))}
	}

	if out.Error != nil {
		return Response{}, &statusError{status, fmt.Errorf("%w: HTTP %d: %s", apperr.ErrLLMUnavailable, status, out.Error.Message)}
	}
	if status != http.StatusOK {
		return Response{}, &statusError{status, fmt.Errorf("%w: HTTP %d: %s", apperr.ErrLLMUnavailable, status, snippet(raw))}
	}
	if len(out.Choices) == 0 {
		return Response{}, fmt.Errorf("%w: модель вернула пустой список вариантов", apperr.ErrLLMFailed)
	}

	choice := out.Choices[0]
	r := Response{Text: choice.Message.Content, Model: out.Model, FinishReason: choice.FinishReason, Usage: out.Usage.toUsage()}
	return r, checkEmpty(r)
}

func checkEmpty(r Response) error {
	if strings.TrimSpace(r.Text) != "" {
		return nil
	}
	err := fmt.Errorf("%w: модель вернула пустой ответ (finish_reason=%q)", apperr.ErrLLMFailed, r.FinishReason)
	if r.Truncated() {
		err = fmt.Errorf("%w: %w", err, ErrEmptyTruncated)
	}
	return err
}

// readStream читает SSE-поток chat/completions: куски ответа отдаёт в onDelta,
// usage и finish_reason берёт из последних событий.
func readStream(body io.Reader, onDelta func(string)) (Response, error) {
	var r Response
	var text strings.Builder
	sc := bufio.NewScanner(body)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		// пустые строки разделяют события, строки с ':' — комментарии (OpenRouter шлёт «PROCESSING»)
		data, ok := strings.CutPrefix(line, "data:")
		if !ok {
			continue
		}
		data = strings.TrimSpace(data)
		if data == "[DONE]" {
			break
		}
		var ev chatResponse
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			continue
		}
		if ev.Error != nil {
			// посреди потока: часть ответа уже показана, повтор задвоил бы прогресс — не повторяем
			r.Text = text.String()
			return r, fmt.Errorf("%w: ошибка в потоке ответа: %s", apperr.ErrLLMUnavailable, ev.Error.Message)
		}
		if ev.Model != "" {
			r.Model = ev.Model
		}
		if ev.Usage != nil {
			r.Usage = ev.Usage.toUsage()
		}
		if len(ev.Choices) == 0 {
			continue
		}
		ch := ev.Choices[0]
		if ch.FinishReason != "" {
			r.FinishReason = ch.FinishReason
		}
		if d := ch.Delta.Content; d != "" {
			if text.Len()+len(d) > maxResponseBytes {
				return r, fmt.Errorf("%w: ответ модели больше %d байт", apperr.ErrLLMFailed, maxResponseBytes)
			}
			text.WriteString(d)
			onDelta(d)
		}
	}
	r.Text = text.String()
	if err := sc.Err(); err != nil {
		return r, fmt.Errorf("%w: ответ модели не дочитан: %v", apperr.ErrLLMUnavailable, err)
	}
	return r, checkEmpty(r)
}

func snippet(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > maxErrorSnippet {
		return s[:maxErrorSnippet] + "…"
	}
	return s
}

type Result struct {
	Reply, HTML, Title, Genre, Audience string
}

func between(s, open, close string) (string, bool) {
	i := strings.Index(s, open)
	if i < 0 {
		return "", false
	}
	i += len(open)
	j := strings.LastIndex(s, close)
	if j < i {

		return strings.TrimSpace(s[i:]), false
	}
	return strings.TrimSpace(s[i:j]), true
}

func stripFence(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		if nl := strings.Index(s, "\n"); nl >= 0 {
			s = s[nl+1:]
		}
		s = strings.TrimSuffix(strings.TrimSpace(s), "```")
	}
	return strings.TrimSpace(s)
}

var ErrBadFormat = errors.New("модель вернула ответ не в ожидаемом формате")

func Parse(raw string) (Result, error) {
	var r Result
	if v, ok := between(raw, "<reply>", "</reply>"); ok {
		r.Reply = v
	}
	if v, ok := between(raw, "<meta>", "</meta>"); ok {
		var m struct{ Title, Genre, Audience string }
		if json.Unmarshal([]byte(stripFence(v)), &m) == nil {
			r.Title, r.Genre, r.Audience = clip(m.Title, 80), clip(m.Genre, 40), clip(m.Audience, 60)
		}
	}
	game, closed := between(raw, "<game>", "</game>")
	if game == "" {

		if i := strings.Index(raw, "```html"); i >= 0 {
			game = stripFence(raw[i:])
			closed = true
		} else if i := strings.Index(strings.ToLower(raw), "<!doctype html"); i >= 0 {
			game, closed = raw[i:], true
		}
	}
	game = stripFence(game)
	low := strings.ToLower(game)
	if len(game) < 200 || !strings.Contains(low, "<html") || !closed || !strings.Contains(low, "</html>") {
		return r, ErrBadFormat
	}
	r.HTML = game
	if r.Reply == "" {
		r.Reply = "Готово! Игра обновлена."
	}
	return r, nil
}

func clip(s string, n int) string {
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}
