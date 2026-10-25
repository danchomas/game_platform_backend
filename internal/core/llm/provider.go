package llm

import (
	"context"
	"strings"
	"time"
)

// Provider — единственная точка входа к модели для агента. Client (OpenRouter и любой
// OpenAI-совместимый шлюз) — одна из реализаций; шлюз Билайна подключается так же.
type Provider interface {
	Complete(ctx context.Context, req Request) (Response, error)
}

type Request struct {
	// Model пусто — модель клиента по умолчанию.
	Model       string
	Fallbacks   []string
	Messages    []Msg
	MaxTokens   int
	Temperature float64
}

type Usage struct {
	PromptTokens     int
	CompletionTokens int
	CostUSD          float64
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

const continuePrompt = "Your previous answer was cut off by the output limit. Continue EXACTLY from the last character you wrote. Output only the continuation: no repetition, no explanations, do not reopen tags that are already open."

// CompleteFull склеивает ответ, оборванный по лимиту токенов: просит модель продолжить
// с места обрыва (до maxCont раз). Usage и Latency суммируются.
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
