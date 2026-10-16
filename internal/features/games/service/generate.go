package service

import (
	"context"
	"errors"
	"time"

	"beeline-games/internal/core/domain"
	"beeline-games/internal/core/llm"
)

var errBusy = errors.New("сервер занят генерациями, попробуйте через минуту")

// generate - внутренняя функция оркестрации LLM и базы
func (s *service) generate(ctx context.Context, g domain.Game, userText string) (int, error) {
	// 1. Очередь (семафор)
	select {
	case s.genSem <- struct{}{}:
		defer func() { <-s.genSem }()
	case <-time.After(20 * time.Second):
		return 0, errBusy
	case <-ctx.Done():
		return 0, ctx.Err()
	}

	// 2. Получаем историю чата
	hist, err := s.repo.Messages(ctx, g.ID)
	if err != nil {
		return 0, err
	}

	llmHist := make([]llm.Msg, 0, len(hist))
	for _, m := range hist {
		llmHist = append(llmHist, llm.Msg{Role: m.Role, Content: m.Content})
	}

	// 3. Формируем промпт (подмешивая аналитику)
	hint := ""
	if s.insights != nil {
		hint = s.insights.GetInsightsHint(ctx)
	}
	msgs := llm.BuildMessages(hint, llmHist, g.HTML, userText)

	// 4. Поход в LLM с ретраем
	var res llm.Result
	for attempt := 0; attempt < 2; attempt++ {
		lctx, cancel := context.WithTimeout(ctx, s.llmTimeout)
		raw, err := s.llm.Chat(lctx, msgs)
		cancel()

		if err != nil {
			return 0, err
		}
		if res, err = llm.Parse(raw); err == nil {
			break
		} else if attempt == 1 {
			return 0, err
		}
	}

	// 5. Сохраняем версию в БД
	return s.repo.AddVersion(ctx, domain.VersionInput{
		GameID: g.ID, UserText: userText, Reply: res.Reply, HTML: res.HTML,
		Title: res.Title, Genre: res.Genre, Audience: res.Audience,
	})
}
