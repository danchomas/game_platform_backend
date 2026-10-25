package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"beeline-games/internal/core/apperr"
	"beeline-games/internal/core/domain"
	"beeline-games/internal/core/htmlguard"
	"beeline-games/internal/core/llm"
)

func (s *Service) generate(ctx context.Context, g domain.Game, userText string) (int, error) {
	release, err := s.acquireSlot(ctx)
	if err != nil {
		return 0, err
	}
	defer release()

	msgs, err := s.buildPrompt(ctx, g, userText)
	if err != nil {
		return 0, err
	}

	res, err := s.askModel(ctx, msgs)
	if err != nil {
		return 0, err
	}

	return s.repo.AddVersion(ctx, domain.VersionInput{
		GameID:   g.ID,
		UserText: userText,
		Reply:    res.Reply,
		HTML:     res.HTML,
		Title:    res.Title,
		Genre:    res.Genre,
		Audience: res.Audience,
	})
}

func (s *Service) acquireSlot(ctx context.Context) (func(), error) {
	timer := time.NewTimer(waitForSlot)
	defer timer.Stop()

	select {
	case s.genSem <- struct{}{}:
		return func() { <-s.genSem }, nil
	case <-timer.C:
		return nil, apperr.ErrCapacity
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *Service) buildPrompt(ctx context.Context, g domain.Game, userText string) ([]llm.Msg, error) {
	hist, err := s.repo.Messages(ctx, g.ID)
	if err != nil {
		return nil, err
	}

	llmHist := make([]llm.Msg, 0, len(hist))
	for _, m := range hist {
		llmHist = append(llmHist, llm.Msg{Role: m.Role, Content: m.Content})
	}

	hint := ""
	if s.insights != nil {
		hint = s.insights.GetInsightsHint(ctx)
	}

	return llm.BuildMessages(hint, llmHist, g.HTML, userText), nil
}

func (s *Service) askModel(ctx context.Context, msgs []llm.Msg) (llm.Result, error) {
	const attempts = 2

	var lastErr error
	for attempt := range attempts {
		lctx, cancel := context.WithTimeout(ctx, s.llmTimeout)
		raw, err := s.llm.Chat(lctx, msgs)
		cancel()

		if err != nil {
			return llm.Result{}, err
		}

		res, err := parseAndCheck(raw)
		if err == nil {
			return res, nil
		}

		lastErr = fmt.Errorf("попытка %d: %w", attempt+1, err)
	}

	return llm.Result{}, lastErr
}

func parseAndCheck(raw string) (llm.Result, error) {
	res, err := llm.Parse(raw)
	if err != nil {
		return llm.Result{}, fmt.Errorf("%w: %w", apperr.ErrLLMFailed, err)
	}

	res.HTML = htmlguard.Sanitize(res.HTML)
	if err := htmlguard.Validate(res.HTML); err != nil {
		return llm.Result{}, err
	}

	return res, nil
}

func wrapLLMErr(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, llm.ErrNoKey):
		return fmt.Errorf("%w: %w", apperr.ErrLLMUnavailable, err)
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return err
	case errors.Is(err, apperr.ErrLLMFailed), errors.Is(err, apperr.ErrCapacity):
		return err
	default:
		return fmt.Errorf("%w: %w", apperr.ErrLLMFailed, err)
	}
}
