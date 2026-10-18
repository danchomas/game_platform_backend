package service

import (
	"beeline-games/internal/core/domain"
	"beeline-games/internal/features/auth/repository/postgres"
	"context"
)

type Service interface {
	CreateUser(ctx context.Context, name, role, managerCode string) (domain.AuthResult, error)
	LoginUser(ctx context.Context, accessKey string) (domain.AuthResult, error)
	GetUser(ctx context.Context, id string) (domain.User, error)
}

type service struct {
	repo		postgres.Repository
	jwtSecret	string
	managerCode	string
}

func NewService(repo postgres.Repository, jwtSecret, managerCode string) Service {
	return &service{
		repo:		repo,
		jwtSecret:	jwtSecret,
		managerCode:	managerCode,
	}
}
