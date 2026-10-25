package http

import (
	"context"

	"beeline-games/internal/core/domain"
)

type Auth interface {
	CreateUser(ctx context.Context, name, role, managerCode string) (domain.AuthResult, error)
	LoginUser(ctx context.Context, accessKey string) (domain.AuthResult, error)
	GetUser(ctx context.Context, id string) (domain.User, error)
}
