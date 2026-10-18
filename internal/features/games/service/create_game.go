package service

import (
	"context"
	"strings"
	"unicode/utf8"

	"beeline-games/internal/core/domain"
	coreErrs "beeline-games/internal/core/errors"
	"beeline-games/internal/features/games/repository/postgres"
)

func (s *service) CreateAndGenerate(ctx context.Context, ownerID, prompt string) (domain.Game, string, int, error) {
	prompt = strings.TrimSpace(prompt)
	if n := utf8.RuneCountInString(prompt); n < 3 || n > 4000 {
		return domain.Game{}, "", 0, coreErrs.ErrInvalidInput
	}

	id, err := s.repo.CreateGame(ctx, ownerID, prompt)
	if err != nil {
		return domain.Game{}, "", 0, err
	}

	s.inflight.Store(id, true)
	defer s.inflight.Delete(id)

	v, err := s.generate(ctx, domain.Game{ID: id}, prompt)
	if err != nil {
		_ = s.repo.DeleteGame(context.WithoutCancel(ctx), id)
		return domain.Game{}, "", 0, err
	}

	g, err := s.repo.GetGame(ctx, id)
	return g, getLatestReply(ctx, s.repo, id), v, err
}

func (s *service) IterateGame(ctx context.Context, userID, gameID, instruction string) (domain.Game, string, int, error) {
	instruction = strings.TrimSpace(instruction)
	if n := utf8.RuneCountInString(instruction); n < 3 || n > 4000 {
		return domain.Game{}, "", 0, coreErrs.ErrInvalidInput
	}

	g, err := s.repo.GetGame(ctx, gameID)
	if err != nil {
		return domain.Game{}, "", 0, err
	}
	if g.OwnerID != userID {
		return domain.Game{}, "", 0, coreErrs.ErrForbidden
	}

	if _, busy := s.inflight.LoadOrStore(g.ID, true); busy {
		return domain.Game{}, "", 0, coreErrs.ErrForbidden
	}
	defer s.inflight.Delete(g.ID)

	v, err := s.generate(ctx, g, instruction)
	if err != nil {
		return domain.Game{}, "", 0, err
	}

	updatedGame, err := s.repo.GetGame(ctx, gameID)
	return updatedGame, getLatestReply(ctx, s.repo, gameID), v, err
}

func getLatestReply(ctx context.Context, repo postgres.Repository, gameID string) string {
	msgs, err := repo.Messages(ctx, gameID)
	if err != nil || len(msgs) == 0 {
		return ""
	}
	return msgs[len(msgs)-1].Content
}
