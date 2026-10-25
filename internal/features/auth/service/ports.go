package service

import (
	"context"

	"beeline-games/internal/core/domain"
)

type UserRepository interface {
	SaveUser(ctx context.Context, name, role, accessKey string) (domain.User, error)
	GetUserByAccessKey(ctx context.Context, key string) (domain.User, error)
	GetUserByID(ctx context.Context, id string) (domain.User, error)
}
