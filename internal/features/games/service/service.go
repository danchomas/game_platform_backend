package service

import (
	"context"
	"errors"
	"sync"

	"beeline-games/internal/core/agent"
	"beeline-games/internal/core/domain"
	"beeline-games/internal/features/games/repository/postgres"
)

// Agent — агентный конвейер (internal/core/agent), интерфейс нужен для тестов.
type Agent interface {
	Plan(ctx context.Context, in agent.PlanInput, tr *agent.Trace) (domain.GameSpec, error)
	Build(ctx context.Context, in agent.BuildInput, stage agent.Stage, tr *agent.Trace) (agent.BuildOutput, error)
}

type InsightProvider interface {
	GetInsightsHint(ctx context.Context) string
}

var (
	ErrBusy     = errors.New("для этой игры уже идёт генерация или она в другом состоянии")
	errBusySlot = errors.New("сервер занят генерациями, попробуйте через минуту")
)

type Service interface {
	// агентный конвейер
	CreateGame(ctx context.Context, ownerID, prompt string, autoApprove bool) (domain.Game, domain.Plan, error)
	RevisePlan(ctx context.Context, userID, gameID, feedback string) (domain.Game, domain.Plan, error)
	ApprovePlan(ctx context.Context, userID, gameID string, spec *domain.GameSpec) (domain.Game, error)
	IterateGame(ctx context.Context, userID, gameID, instruction string) (domain.Game, error)
	Subscribe(ctx context.Context, userID, gameID string) (domain.Game, <-chan domain.StatusEvent, func(), error)
	ListGenerations(ctx context.Context, userID, gameID string) ([]domain.Generation, error)
	Screenshot(ctx context.Context, userID, gameID string) ([]byte, error)
	GenerationStats(ctx context.Context, role string, days int) (domain.GenerationStats, error)
	RecoverInterrupted(ctx context.Context) error
	Wait()

	GetGame(ctx context.Context, userID, gameID string) (domain.Game, error)
	GetMyGames(ctx context.Context, userID string, limit, offset int) ([]domain.Game, error)
	GetGallery(ctx context.Context, sort, q string, limit, offset int) ([]domain.Game, error)

	PatchGame(ctx context.Context, userID, gameID, title string) (domain.Game, error)
	DeleteGame(ctx context.Context, userID, gameID string) error
	PublishGame(ctx context.Context, userID, gameID string, publish bool) (domain.Game, error)

	GetMessages(ctx context.Context, userID, gameID string) ([]domain.Message, error)
	GetVersions(ctx context.Context, userID, gameID string) ([]domain.VersionInfo, error)
	RestoreVersion(ctx context.Context, userID, gameID string, version int) (domain.Game, string, int, error)

	IsPublished(ctx context.Context, gameID string) (bool, error)
	CheckOwnership(ctx context.Context, userID, gameID string) (bool, error)
}

type service struct {
	repo     postgres.Repository
	agent    Agent
	insights InsightProvider

	// bg — контекст приложения: фоновые сборки живут дольше HTTP-запроса
	bg     context.Context
	genSem chan struct{}
	jobs   sync.WaitGroup
	events *broker
}

func NewService(bg context.Context, repo postgres.Repository, ag Agent, insights InsightProvider, maxConcurrent int) Service {
	if maxConcurrent <= 0 {
		maxConcurrent = 4
	}
	return &service{
		repo:     repo,
		agent:    ag,
		insights: insights,
		bg:       bg,
		genSem:   make(chan struct{}, maxConcurrent),
		events:   newBroker(),
	}
}

func (s *service) Wait() { s.jobs.Wait() }
