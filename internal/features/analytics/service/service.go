package service

import (
	"sync"
	"time"
)

const (
	defaultDays = 30
	minDays     = 1
	maxDays     = 365

	hintTTL = 5 * time.Minute

	hintGames = 3
)

type Service struct {
	repo  StatsRepository
	games GameOwnership

	hintMu  sync.Mutex
	hint    string
	hintExp time.Time
}

func NewService(repo StatsRepository, games GameOwnership) *Service {
	return &Service{repo: repo, games: games}
}

func clampDays(days int) int {
	switch {
	case days <= 0:
		return defaultDays
	case days < minDays:
		return minDays
	case days > maxDays:
		return maxDays
	default:
		return days
	}
}
