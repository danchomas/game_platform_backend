package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"unicode/utf8"

	"beeline-games/internal/core/apperr"
	"beeline-games/internal/core/domain"
	"beeline-games/internal/core/jwt"
)

func (s *Service) CreateUser(ctx context.Context, name, role, managerCode string) (domain.AuthResult, error) {
	name = strings.TrimSpace(name)
	if n := utf8.RuneCountInString(name); n < minNameLen || n > maxNameLen {
		return domain.AuthResult{}, apperr.NewValidationError("name",
			fmt.Sprintf("имя: от %d до %d символов", minNameLen, maxNameLen))
	}

	actualRole := domain.RolePlayer
	if role == domain.RoleManager {
		if s.managerCode != "" && managerCode != s.managerCode {
			return domain.AuthResult{}, fmt.Errorf("неверный код менеджера: %w", apperr.ErrForbidden)
		}
		actualRole = domain.RoleManager
	}

	accessKey, err := randomKey()
	if err != nil {
		return domain.AuthResult{}, err
	}

	u, err := s.repo.SaveUser(ctx, name, actualRole, accessKey)
	if err != nil {
		return domain.AuthResult{}, err
	}

	return domain.AuthResult{
		User:      u,
		Token:     jwt.Issue(s.jwtSecret, u.ID, u.Role, tokenTTL),
		AccessKey: accessKey,
	}, nil
}

func randomKey() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("генерация кода доступа: %w", err)
	}
	return hex.EncodeToString(b), nil
}
