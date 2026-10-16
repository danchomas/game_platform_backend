package service

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"beeline-games/internal/core/domain"
	coreErrs "beeline-games/internal/core/errors"
)

// Helper: проверяет права владельца
func (s *service) getOwnedGame(ctx context.Context, userID, gameID string) (domain.Game, error) {
	g, err := s.repo.GetGame(ctx, gameID)
	if err != nil {
		return g, err
	}
	if g.OwnerID != userID {
		return g, coreErrs.ErrForbidden
	}
	return g, nil
}

func (s *service) GetGame(ctx context.Context, userID, gameID string) (domain.Game, error) {
	g, err := s.repo.GetGame(ctx, gameID)
	if err != nil {
		return g, err
	}
	// Черновики доступны только владельцу
	if !g.Published && g.OwnerID != userID {
		return g, coreErrs.ErrNotFound
	}
	return g, nil
}

func (s *service) GetMyGames(ctx context.Context, userID string, limit, offset int) ([]domain.Game, error) {
	return s.repo.ListOwnerGames(ctx, userID, limit, offset)
}

func (s *service) GetGallery(ctx context.Context, sort, q string, limit, offset int) ([]domain.Game, error) {
	return s.repo.ListPublished(ctx, sort, strings.TrimSpace(q), limit, offset)
}

func (s *service) PatchGame(ctx context.Context, userID, gameID, title string) (domain.Game, error) {
	g, err := s.getOwnedGame(ctx, userID, gameID)
	if err != nil {
		return g, err
	}

	t := strings.TrimSpace(title)
	if n := utf8.RuneCountInString(t); n < 1 || n > 80 {
		return g, coreErrs.ErrInvalidInput
	}

	if err := s.repo.SetTitle(ctx, gameID, t); err != nil {
		return g, err
	}
	g.Title = t
	return g, nil
}

func (s *service) DeleteGame(ctx context.Context, userID, gameID string) error {
	_, err := s.getOwnedGame(ctx, userID, gameID)
	if err != nil {
		return err
	}
	return s.repo.DeleteGame(ctx, gameID)
}

func (s *service) PublishGame(ctx context.Context, userID, gameID string, publish bool) (domain.Game, error) {
	g, err := s.getOwnedGame(ctx, userID, gameID)
	if err != nil {
		return g, err
	}
	if publish && g.Version == 0 {
		return g, coreErrs.ErrInvalidInput // В игре ещё нет сгенерированного кода
	}

	if err := s.repo.SetPublished(ctx, gameID, publish); err != nil {
		return g, err
	}
	g.Published = publish
	return g, nil
}

func (s *service) GetMessages(ctx context.Context, userID, gameID string) ([]domain.Message, error) {
	if _, err := s.getOwnedGame(ctx, userID, gameID); err != nil {
		return nil, err
	}
	return s.repo.Messages(ctx, gameID)
}

func (s *service) GetVersions(ctx context.Context, userID, gameID string) ([]domain.VersionInfo, error) {
	if _, err := s.getOwnedGame(ctx, userID, gameID); err != nil {
		return nil, err
	}
	return s.repo.Versions(ctx, gameID)
}

func (s *service) RestoreVersion(ctx context.Context, userID, gameID string, version int) (domain.Game, string, int, error) {
	g, err := s.getOwnedGame(ctx, userID, gameID)
	if err != nil {
		return g, "", 0, err
	}

	html, err := s.repo.VersionHTML(ctx, gameID, version)
	if err != nil {
		return g, "", 0, err
	}

	reply := fmt.Sprintf("Восстановлена версия %d.", version)
	newVer, err := s.repo.AddVersion(ctx, domain.VersionInput{
		GameID: gameID,
		Reply:  reply,
		HTML:   html,
	})
	if err != nil {
		return g, "", 0, err
	}

	updatedGame, err := s.repo.GetGame(ctx, gameID)
	return updatedGame, reply, newVer, err
}

func (s *service) IsPublished(ctx context.Context, gameID string) (bool, error) {
	return s.repo.IsPublished(ctx, gameID)
}

func (s *service) CheckOwnership(ctx context.Context, userID, gameID string) (bool, error) {
	g, err := s.repo.GetGame(ctx, gameID)
	if err != nil {
		return false, err
	}
	return g.OwnerID == userID, nil
}
