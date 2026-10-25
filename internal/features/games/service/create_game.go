package service

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"beeline-games/internal/core/apperr"
	"beeline-games/internal/core/domain"
)

func (s *Service) CreateAndGenerate(ctx context.Context, ownerID, prompt string) (domain.Game, string, int, error) {
	prompt, err := validatePrompt(prompt)
	if err != nil {
		return domain.Game{}, "", 0, err
	}

	id, err := s.repo.CreateGame(ctx, ownerID, prompt)
	if err != nil {
		return domain.Game{}, "", 0, err
	}

	s.inflight.Store(id, struct{}{})
	defer s.inflight.Delete(id)

	version, err := s.generate(ctx, domain.Game{ID: id}, prompt)
	if err != nil {
		if delErr := s.repo.DeleteGame(context.WithoutCancel(ctx), id); delErr != nil {
			return domain.Game{}, "", 0, fmt.Errorf("%w (карточка не удалена: %v)", wrapLLMErr(err), delErr)
		}
		return domain.Game{}, "", 0, wrapLLMErr(err)
	}

	return s.loadAfterGeneration(ctx, id, version)
}

func (s *Service) IterateGame(ctx context.Context, userID, gameID, instruction string) (domain.Game, string, int, error) {
	instruction, err := validatePrompt(instruction)
	if err != nil {
		return domain.Game{}, "", 0, err
	}

	g, err := s.getOwnedGame(ctx, userID, gameID)
	if err != nil {
		return domain.Game{}, "", 0, err
	}

	if _, busy := s.inflight.LoadOrStore(g.ID, struct{}{}); busy {
		return domain.Game{}, "", 0, apperr.ErrConflict
	}
	defer s.inflight.Delete(g.ID)

	version, err := s.generate(ctx, g, instruction)
	if err != nil {
		return domain.Game{}, "", 0, wrapLLMErr(err)
	}

	return s.loadAfterGeneration(ctx, gameID, version)
}

func (s *Service) loadAfterGeneration(ctx context.Context, gameID string, version int) (domain.Game, string, int, error) {
	g, err := s.repo.GetGame(ctx, gameID)
	if err != nil {
		return domain.Game{}, "", 0, err
	}
	return g, s.latestReply(ctx, gameID), version, nil
}

func (s *Service) latestReply(ctx context.Context, gameID string) string {
	msgs, err := s.repo.Messages(ctx, gameID)
	if err != nil || len(msgs) == 0 {
		return ""
	}
	return msgs[len(msgs)-1].Content
}

func validatePrompt(text string) (string, error) {
	text = strings.TrimSpace(text)
	if n := utf8.RuneCountInString(text); n < minPromptLen || n > maxPromptLen {
		return "", apperr.NewValidationError("prompt",
			fmt.Sprintf("текст: от %d до %d символов", minPromptLen, maxPromptLen))
	}
	return text, nil
}
