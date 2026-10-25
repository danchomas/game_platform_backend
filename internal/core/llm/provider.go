package llm

import (
	"context"
	"errors"
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
	// Reasoning — бюджет скрытых рассуждений «думающих» моделей: ReasoningOff, low, medium, high.
	// Пусто — как решит провайдер (у части моделей рассуждения съедают весь max_tokens).
	Reasoning string
	// OnDelta != nil — ответ запрашивается потоком, каждый кусок текста приходит сюда по мере
	// написания (прогресс сборки). Итоговый Response тот же, что и без потока.
	OnDelta func(text string)
}

// Уровни Request.Reasoning.
const (
	ReasoningOff    = "off"
	ReasoningLow    = "low"
	ReasoningMedium = "medium"
	ReasoningHigh   = "high"
)

// ErrEmptyTruncated — модель потратила весь лимит токенов (обычно на рассуждения)
// и не написала ответа. Продолжение не поможет; помогает запрос с меньшими рассуждениями.
var ErrEmptyTruncated = errors.New("ответ пуст: лимит токенов исчерпан до начала ответа")

type Usage struct {
	PromptTokens int
	// CompletionTokens включает ReasoningTokens: рассуждения оплачиваются как выход.
	CompletionTokens int
	ReasoningTokens  int
	CostUSD          float64
}

func (u *Usage) Add(o Usage) {
	u.PromptTokens += o.PromptTokens
	u.CompletionTokens += o.CompletionTokens
	u.ReasoningTokens += o.ReasoningTokens
	u.CostUSD += o.CostUSD
}

// WithCallTimeout ограничивает каждый отдельный запрос к модели. Нужен для CompleteFull:
// продолжению оборванного ответа полагается свой таймаут, а не остаток от первой части.
func WithCallTimeout(p Provider, d time.Duration) Provider {
	return timeoutProvider{p: p, d: d}
}

type timeoutProvider struct {
	p Provider
	d time.Duration
}

func (t timeoutProvider) Complete(ctx context.Context, req Request) (Response, error) {
	if t.d <= 0 {
		return t.p.Complete(ctx, req)
	}
	cctx, cancel := context.WithTimeout(ctx, t.d)
	defer cancel()
	return t.p.Complete(cctx, req)
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
