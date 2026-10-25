package llm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Provider — единственная точка входа к любой LLM. OpenRouter — одна из реализаций;
// шлюз Билайна подключается новой реализацией без изменений агента и сервисов.
type Provider interface {
	Complete(ctx context.Context, req Request) (Response, error)
}

type Msg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Request struct {
	Model       string
	Fallbacks   []string
	Messages    []Msg
	MaxTokens   int
	Temperature float64
}

type Usage struct {
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	CostUSD          float64 `json:"cost_usd"`
}

func (u *Usage) Add(o Usage) {
	u.PromptTokens += o.PromptTokens
	u.CompletionTokens += o.CompletionTokens
	u.CostUSD += o.CostUSD
}

type Response struct {
	Text         string
	Model        string
	FinishReason string
	Usage        Usage
	Latency      time.Duration
}

// Truncated — модель упёрлась в max_tokens и ответ оборван.
func (r Response) Truncated() bool {
	return r.FinishReason == "length" || r.FinishReason == "max_tokens"
}

var ErrNoKey = errors.New("ключ LLM-провайдера не задан")

type HTTPError struct {
	Status int
	Msg    string
}

func (e *HTTPError) Error() string { return fmt.Sprintf("llm: HTTP %d: %s", e.Status, e.Msg) }

func (e *HTTPError) Retryable() bool {
	return e.Status == 408 || e.Status == 429 || e.Status >= 500
}

const continuePrompt = "Your previous answer was cut off by the output limit. Continue EXACTLY from the last character you wrote. Output only the continuation: no repetition, no explanations, do not reopen tags that are already open."

// CompleteFull склеивает ответ, если модель оборвалась по лимиту токенов:
// просит продолжить с места обрыва (до maxCont раз). Usage и Latency суммируются.
func CompleteFull(ctx context.Context, p Provider, req Request, maxCont int) (Response, error) {
	resp, err := p.Complete(ctx, req)
	if err != nil {
		return resp, err
	}
	var sb strings.Builder
	sb.WriteString(resp.Text)

	for i := 0; i < maxCont && resp.Truncated(); i++ {
		next := req
		next.Messages = append(append([]Msg{}, req.Messages...),
			Msg{Role: "assistant", Content: sb.String()},
			Msg{Role: "user", Content: continuePrompt})
		part, err := p.Complete(ctx, next)
		if err != nil {
			return resp, err
		}
		sb.WriteString(part.Text)
		resp.FinishReason = part.FinishReason
		resp.Usage.Add(part.Usage)
		resp.Latency += part.Latency
	}
	resp.Text = sb.String()
	return resp, nil
}
