package service

import (
	"time"
)

const tokenTTL = 30 * 24 * time.Hour

const (
	minNameLen = 2
	maxNameLen = 40
)

type Service struct {
	repo        UserRepository
	jwtSecret   string
	managerCode string
}

func NewService(repo UserRepository, jwtSecret, managerCode string) *Service {
	return &Service{
		repo:        repo,
		jwtSecret:   jwtSecret,
		managerCode: managerCode,
	}
}
