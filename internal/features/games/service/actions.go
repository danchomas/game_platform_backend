package service

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"beeline-games/internal/core/apperr"
	"beeline-games/internal/core/domain"
)

func (s *Service) getOwnedGame(ctx context.Context, userID, gameID string) (domain.Game, error) {
	g, err := s.repo.GetGame(ctx, gameID)
	if err != nil {
		return domain.Game{}, err
	}
	if g.OwnerID != userID {
		return domain.Game{}, apperr.ErrForbidden
	}
	return g, nil
}

func (s *Service) GetGame(ctx context.Context, userID, gameID string) (domain.Game, error) {
	g, err := s.repo.GetGame(ctx, gameID)
	if err != nil {
		return domain.Game{}, err
	}
	if !g.Published && g.OwnerID != userID {
		return domain.Game{}, apperr.ErrNotFound
	}
	return g, nil
}

func (s *Service) GetMyGames(ctx context.Context, userID string, limit, offset int) ([]domain.Game, error) {
	return s.repo.ListOwnerGames(ctx, userID, ownerQuery(limit, offset))
}

func (s *Service) GetGallery(ctx context.Context, sort, search string, limit, offset int) ([]domain.Game, error) {
	return s.repo.ListPublished(ctx, galleryQuery(sort, strings.TrimSpace(search), limit, offset))
}

func (s *Service) PatchGame(ctx context.Context, userID, gameID, title string) (domain.Game, error) {
	g, err := s.getOwnedGame(ctx, userID, gameID)
	if err != nil {
		return domain.Game{}, err
	}

	title = strings.TrimSpace(title)
	if n := utf8.RuneCountInString(title); n < minTitleLen || n > maxTitleLen {
		return domain.Game{}, apperr.NewValidationError("title",
			fmt.Sprintf("название: от %d до %d символов", minTitleLen, maxTitleLen))
	}

	if err := s.repo.SetTitle(ctx, gameID, title); err != nil {
		return domain.Game{}, err
	}

	g.Title = title
	return g, nil
}

func (s *Service) DeleteGame(ctx context.Context, userID, gameID string) error {
	if _, err := s.getOwnedGame(ctx, userID, gameID); err != nil {
		return err
	}
	s.reviews.stop(gameID)
	return s.repo.DeleteGame(ctx, gameID)
}

func (s *Service) PublishGame(ctx context.Context, userID, gameID string, publish bool) (domain.Game, error) {
	g, err := s.getOwnedGame(ctx, userID, gameID)
	if err != nil {
		return domain.Game{}, err
	}
	if publish && !g.Generated() {
		return domain.Game{}, apperr.ErrNotGenerated
	}

	if err := s.repo.SetPublished(ctx, gameID, publish); err != nil {
		return domain.Game{}, err
	}

	g.Published = publish
	return g, nil
}

func (s *Service) GetMessages(ctx context.Context, userID, gameID string) ([]domain.Message, error) {
	if _, err := s.getOwnedGame(ctx, userID, gameID); err != nil {
		return nil, err
	}
	return s.repo.Messages(ctx, gameID)
}

func (s *Service) GetVersions(ctx context.Context, userID, gameID string) ([]domain.VersionInfo, error) {
	if _, err := s.getOwnedGame(ctx, userID, gameID); err != nil {
		return nil, err
	}
	return s.repo.Versions(ctx, gameID)
}

func (s *Service) RestoreVersion(ctx context.Context, userID, gameID string, version int) (domain.Game, string, int, error) {
	if _, err := s.getOwnedGame(ctx, userID, gameID); err != nil {
		return domain.Game{}, "", 0, err
	}
	if version < 1 {
		return domain.Game{}, "", 0, apperr.NewValidationError("version", "номер версии начинается с 1")
	}

	html, err := s.repo.VersionHTML(ctx, gameID, version)
	if err != nil {
		return domain.Game{}, "", 0, err
	}

	s.reviews.stop(gameID) // откат пользователя важнее фоновой проверки
	reply := fmt.Sprintf("Восстановлена версия %d.", version)
	newVersion, err := s.repo.AddVersion(ctx, domain.VersionInput{
		GameID: gameID,
		Reply:  reply,
		HTML:   html,
	})
	if err != nil {
		return domain.Game{}, "", 0, err
	}

	g, err := s.repo.GetGame(ctx, gameID)
	if err != nil {
		return domain.Game{}, "", 0, err
	}
	return g, reply, newVersion, nil
}

func (s *Service) IsPublished(ctx context.Context, gameID string) (bool, error) {
	return s.repo.IsPublished(ctx, gameID)
}

func (s *Service) CheckOwnership(ctx context.Context, userID, gameID string) (bool, error) {
	g, err := s.repo.GetGame(ctx, gameID)
	if err != nil {
		return false, err
	}
	return g.OwnerID == userID, nil
}
