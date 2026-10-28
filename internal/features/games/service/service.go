package service

import (
	"context"
	"sync"
	"time"

	"beeline-games/internal/core/domain"
)

const (
	minPromptLen = 3
	maxPromptLen = 4000

	minTitleLen = 1
	maxTitleLen = 80

	defaultOwnerLimit   = 50
	maxOwnerLimit       = 200
	defaultGalleryLimit = 24
	maxGalleryLimit     = 100
	maxOffset           = 1 << 20

	// План — короткий синхронный вызов; сборка идёт в фоне и может подождать слот дольше.
	waitForSlot      = 20 * time.Second
	waitForBuildSlot = 3 * time.Minute
	// Варианты мультигенерации стартуют пачкой и встают в очередь за слотами сами за собой.
	waitForVariantSlot = 20 * time.Minute

	generationsLimit = 50
)

type Service struct {
	repo     GameRepository
	agent    Agent
	insights InsightProvider

	// bg — контекст приложения: фоновые сборки живут дольше HTTP-запроса.
	bg     context.Context
	genSem chan struct{}
	jobs   sync.WaitGroup
	events *broker
}

func NewService(bg context.Context, repo GameRepository, ag Agent, insights InsightProvider, maxConcurrent int) *Service {
	if maxConcurrent < 1 {
		maxConcurrent = 1
	}
	return &Service{
		repo:     repo,
		agent:    ag,
		insights: insights,
		bg:       bg,
		genSem:   make(chan struct{}, maxConcurrent),
		events:   newBroker(),
	}
}

// Wait дожидается фоновых сборок (graceful shutdown: их контекст уже отменён).
func (s *Service) Wait() { s.jobs.Wait() }

func ownerQuery(limit, offset int) domain.GameQuery {
	return domain.GameQuery{
		Limit:  clamp(limit, 1, maxOwnerLimit, defaultOwnerLimit),
		Offset: clamp(offset, 0, maxOffset, 0),
	}
}

func galleryQuery(sort, search string, limit, offset int) domain.GameQuery {
	if sort != domain.SortPopular {
		sort = domain.SortNew
	}
	return domain.GameQuery{
		Sort:   sort,
		Search: search,
		Limit:  clamp(limit, 1, maxGalleryLimit, defaultGalleryLimit),
		Offset: clamp(offset, 0, maxOffset, 0),
	}
}

func clamp(v, lo, hi, def int) int {
	if v <= 0 {
		v = def
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func (s *Service) SetInsights(insights InsightProvider) {
	s.insights = insights
}
