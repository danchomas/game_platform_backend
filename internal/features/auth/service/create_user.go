package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"
	"unicode/utf8"

	"beeline-games/internal/core/domain"
	coreErrs "beeline-games/internal/core/errors"
	"beeline-games/internal/core/jwt"
)

const tokenTTL = 30 * 24 * time.Hour

func (s *service) CreateUser(ctx context.Context, name, role, managerCode string) (domain.AuthResult, error) {
	name = strings.TrimSpace(name)
	if n := utf8.RuneCountInString(name); n < 2 || n > 40 {
		return domain.AuthResult{}, coreErrs.ErrInvalidInput
	}

	actualRole := "player"
	if role == "manager" {
		if s.managerCode != "" && managerCode != s.managerCode {
			return domain.AuthResult{}, coreErrs.ErrForbidden
		}
		actualRole = "manager"
	}

	accessKey := randomKey()
	u, err := s.repo.SaveUser(ctx, name, actualRole, accessKey)
	if err != nil {
		return domain.AuthResult{}, err
	}

	token := jwt.Issue(s.jwtSecret, u.ID, u.Role, tokenTTL)

	return domain.AuthResult{
		User:      u,
		Token:     token,
		AccessKey: accessKey,
	}, nil
}

func randomKey() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
