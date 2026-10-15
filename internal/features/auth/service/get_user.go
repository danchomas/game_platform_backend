package service

import (
	"context"
	"strings"

	"beeline-games/internal/core/domain"
	coreErrs "beeline-games/internal/core/errors"
	"beeline-games/internal/core/jwt"
)

func (s *service) LoginUser(ctx context.Context, accessKey string) (domain.AuthResult, error) {
	accessKey = strings.TrimSpace(accessKey)
	u, err := s.repo.GetUserByAccessKey(ctx, accessKey)
	if err != nil {
		return domain.AuthResult{}, coreErrs.ErrUnauthorized
	}

	token := jwt.Issue(s.jwtSecret, u.ID, u.Role, tokenTTL)
	return domain.AuthResult{User: u, Token: token}, nil
}

func (s *service) GetUser(ctx context.Context, id string) (domain.User, error) {
	return s.repo.GetUserByID(ctx, id)
}
