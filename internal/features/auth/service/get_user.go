package service

import (
	"context"
	"errors"
	"strings"

	"beeline-games/internal/core/apperr"
	"beeline-games/internal/core/domain"
	"beeline-games/internal/core/jwt"
)

func (s *Service) LoginUser(ctx context.Context, accessKey string) (domain.AuthResult, error) {
	accessKey = strings.TrimSpace(accessKey)
	if accessKey == "" {
		return domain.AuthResult{}, apperr.ErrUnauthorized
	}

	u, err := s.repo.GetUserByAccessKey(ctx, accessKey)
	if errors.Is(err, apperr.ErrNotFound) {
		return domain.AuthResult{}, apperr.ErrUnauthorized
	}
	if err != nil {
		return domain.AuthResult{}, err
	}

	return domain.AuthResult{
		User:  u,
		Token: jwt.Issue(s.jwtSecret, u.ID, u.Role, tokenTTL),
	}, nil
}

func (s *Service) GetUser(ctx context.Context, id string) (domain.User, error) {
	return s.repo.GetUserByID(ctx, id)
}
