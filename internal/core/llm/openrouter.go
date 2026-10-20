package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const defaultOpenRouterURL = "https://openrouter.ai/api/v1/chat/completions"

// OpenRouter — OpenAI-совместимый chat/completions. Подходит и для любого
// другого OpenAI-совместимого шлюза: достаточно сменить endpoint и ключ.
type OpenRouter struct {
	key, url string
	http     *http.Client
	retries  int
}

func NewOpenRouter(key, endpoint, proxyStr string, timeout time.Duration) *OpenRouter {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if proxyStr != "" {
		if pURL, err := url.Parse(proxyStr); err == nil {
			transport.Proxy = http.ProxyURL(pURL)
		}
	}
	if endpoint == "" {
		endpoint = defaultOpenRouterURL
	}
	return &OpenRouter{
		key:     key,
		url:     endpoint,
		http:    &http.Client{Timeout: timeout, Transport: transport},
		retries: 2,
	}
}

type orRequest struct {
	Model       string         `json:"model"`
	Models      []string       `json:"models,omitempty"`
	Messages    []Msg          `json:"messages"`
	MaxTokens   int            `json:"max_tokens,omitempty"`
	Temperature float64        `json:"temperature"`
	Usage       map[string]any `json:"usage"`
}

type orResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int     `json:"prompt_tokens"`
		CompletionTokens int     `json:"completion_tokens"`
		Cost             float64 `json:"cost"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Code    any    `json:"code"`
	} `json:"error"`
}

func (c *OpenRouter) Complete(ctx context.Context, req Request) (Response, error) {
	if c.key == "" {
		return Response{}, ErrNoKey
	}
	body := orRequest{
		Model:       req.Model,
		Messages:    req.Messages,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Usage:       map[string]any{"include": true},
	}
	if len(req.Fallbacks) > 0 {
		body.Models = append([]string{req.Model}, req.Fallbacks...)
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return Response{}, err
	}

	var lastErr error
	for attempt := 0; attempt <= c.retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return Response{}, ctx.Err()
			case <-time.After(time.Duration(attempt*2) * time.Second):
			}
		}
		resp, err := c.do(ctx, raw)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		var he *HTTPError
		if !errors.As(err, &he) || !he.Retryable() {
			return Response{}, err
		}
	}
	return Response{}, lastErr
}

func (c *OpenRouter) do(ctx context.Context, body []byte) (Response, error) {
	start := time.Now()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return Response{}, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.key)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("HTTP-Referer", "https://github.com/beeline-hackathon/vibe-games")
	httpReq.Header.Set("X-Title", "Beeline Vibe Games")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return Response{}, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return Response{}, err
	}

	var out orResponse
	if err := json.Unmarshal(data, &out); err != nil {
		return Response{}, &HTTPError{Status: resp.StatusCode, Msg: fmt.Sprintf("не JSON: %.200s", data)}
	}
	if out.Error != nil {
		status := resp.StatusCode
		if status < 300 {
			status = http.StatusBadGateway
		}
		return Response{}, &HTTPError{Status: status, Msg: out.Error.Message}
	}
	if resp.StatusCode >= 300 || len(out.Choices) == 0 {
		return Response{}, &HTTPError{Status: resp.StatusCode, Msg: "пустой ответ"}
	}

	r := Response{
		Text:         out.Choices[0].Message.Content,
		Model:        out.Model,
		FinishReason: out.Choices[0].FinishReason,
		Latency:      time.Since(start),
	}
	if out.Usage != nil {
		r.Usage = Usage{
			PromptTokens:     out.Usage.PromptTokens,
			CompletionTokens: out.Usage.CompletionTokens,
			CostUSD:          out.Usage.Cost,
		}
	}
	return r, nil
}
