package llm

import (
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
	maxTokens   = 8000
	temperature = 0.7

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
	Model       string  `json:"model"`
	Messages    []Msg   `json:"messages"`
	MaxTokens   int     `json:"max_tokens"`
	Temperature float64 `json:"temperature"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Code    any    `json:"code"`
	} `json:"error"`
}

func (c *Client) Chat(ctx context.Context, msgs []Msg) (string, error) {
	if c.apiKey == "" {
		return "", ErrNoKey
	}

	body, err := json.Marshal(chatRequest{
		Model:       c.model,
		Messages:    msgs,
		MaxTokens:   maxTokens,
		Temperature: temperature,
	})
	if err != nil {
		return "", fmt.Errorf("сборка запроса к модели: %w", err)
	}

	endpoint := c.baseURL + chatPath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("%w: некорректный адрес %q: %v", apperr.ErrLLMUnavailable, endpoint, err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("HTTP-Referer", appURL)
	req.Header.Set("X-Title", appTitle)

	resp, err := c.http.Do(req)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return "", err
		}
		return "", fmt.Errorf("%w: запрос к %s не прошёл: %v", apperr.ErrLLMUnavailable, c.baseURL, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return "", fmt.Errorf("%w: ответ модели не дочитан: %v", apperr.ErrLLMUnavailable, err)
	}

	return parseChatResponse(resp.StatusCode, raw)
}

func parseChatResponse(status int, raw []byte) (string, error) {
	var out chatResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("%w: HTTP %d, ответ не JSON: %s",
			apperr.ErrLLMUnavailable, status, snippet(raw))
	}

	if out.Error != nil {
		return "", fmt.Errorf("%w: HTTP %d: %s", apperr.ErrLLMUnavailable, status, out.Error.Message)
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("%w: HTTP %d: %s", apperr.ErrLLMUnavailable, status, snippet(raw))
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("%w: модель вернула пустой список вариантов", apperr.ErrLLMFailed)
	}

	content := out.Choices[0].Message.Content
	if strings.TrimSpace(content) == "" {
		return "", fmt.Errorf("%w: модель вернула пустой ответ (finish_reason=%q)",
			apperr.ErrLLMFailed, out.Choices[0].FinishReason)
	}

	return content, nil
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
