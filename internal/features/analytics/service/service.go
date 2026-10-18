package service

import (
	"context"
	"sync"
	"time"

	"beeline-games/internal/core/domain"
	"beeline-games/internal/features/analytics/repository/postgres"
)

type DashboardData struct {
	Scope		string			`json:"scope"`
	Days		int			`json:"days"`
	Summary		domain.Summary		`json:"summary"`
	Daily		[]domain.DayPoint	`json:"daily"`
	Games		[]domain.GameRow	`json:"games,omitempty"`
	CustomEvents	[]domain.EventCount	`json:"custom_events"`
	SessionDuration	[]domain.Bucket		`json:"session_duration"`
}

type Service interface {
	GetOverview(ctx context.Context, userID, role, scope string, days int) (DashboardData, error)
	GetGameAnalytics(ctx context.Context, userID, role, gameID string, days int) (DashboardData, error)
	GetInsightsHint(ctx context.Context) string
}

type service struct {
	repo	postgres.Repository
	hintMu	sync.Mutex
	hint	string
	hintExp	time.Time
}

func NewService(repo postgres.Repository) Service {
	return &service{repo: repo}
}
