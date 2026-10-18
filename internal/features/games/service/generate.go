package service

import (
	"context"
	"errors"
	"time"

	"beeline-games/internal/core/domain"
	"beeline-games/internal/core/llm"
)

var errBusy = errors.New("сервер занят генерациями, попробуйте через минуту")

func (s *service) generate(ctx context.Context, g domain.Game, userText string) (int, error) {

	select {
	case s.genSem <- struct{}{}:
		defer func() { <-s.genSem }()
	case <-time.After(20 * time.Second):
		return 0, errBusy
	case <-ctx.Done():
		return 0, ctx.Err()
	}

	hist, err := s.repo.Messages(ctx, g.ID)
	if err != nil {
		return 0, err
	}

	llmHist := make([]llm.Msg, 0, len(hist))
	for _, m := range hist {
		llmHist = append(llmHist, llm.Msg{Role: m.Role, Content: m.Content})
	}

	hint := ""
	if s.insights != nil {
		hint = s.insights.GetInsightsHint(ctx)
	}
	msgs := llm.BuildMessages(hint, llmHist, g.HTML, userText)

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

	return s.repo.AddVersion(ctx, domain.VersionInput{
		GameID:	g.ID, UserText: userText, Reply: res.Reply, HTML: res.HTML,
		Title:	res.Title, Genre: res.Genre, Audience: res.Audience,
	})
}
