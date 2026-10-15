// Package llm — клиент OpenRouter и разбор ответа модели.
package llm

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Msg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Client struct {
	Key, Model, URL string
	HTTP            *http.Client
}

func NewClient(key, model, endpoint, proxyStr string, timeout time.Duration) *Client {
	client := &http.Client{Timeout: timeout}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	if proxyStr != "" {
		if pURL, err := url.Parse(proxyStr); err == nil {
			transport.Proxy = http.ProxyURL(pURL)
		}
	}
	client.Transport = transport

	return &Client{Key: key, Model: model, URL: endpoint, HTTP: client}
}

var ErrNoKey = errors.New("OPENROUTER_API_KEY не задан")

func (c *Client) Chat(ctx context.Context, msgs []Msg) (string, error) {
	if c.Key == "" {
		return "", ErrNoKey
	}
	body, _ := json.Marshal(map[string]any{
		"model":       c.Model,
		"messages":    msgs,
		"max_tokens":  8000,
		"temperature": 0.7,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.Key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("HTTP-Referer", "https://github.com/beeline-hackathon/vibe-games")
	req.Header.Set("X-Title", "Beeline Vibe Games")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return "", err
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("openrouter: HTTP %d, не JSON: %.200s", resp.StatusCode, raw)
	}
	if out.Error != nil {
		return "", fmt.Errorf("openrouter: %s", out.Error.Message)
	}
	if resp.StatusCode >= 300 || len(out.Choices) == 0 {
		return "", fmt.Errorf("openrouter: HTTP %d, пустой ответ", resp.StatusCode)
	}
	return out.Choices[0].Message.Content, nil
}

// ---------- разбор ответа ----------

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
		// закрывающий тег не пришёл (обрыв по лимиту токенов)
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
		// запасной вариант: html в markdown-блоке или «голый» документ
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
