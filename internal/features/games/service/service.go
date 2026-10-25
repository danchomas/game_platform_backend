package service

import (
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

	waitForSlot = 20 * time.Second
)

type Service struct {
	repo       GameRepository
	llm        LLMProvider
	insights   InsightProvider
	llmTimeout time.Duration

	genSem   chan struct{}
	inflight sync.Map
}

func NewService(repo GameRepository, model LLMProvider, insights InsightProvider, maxConcurrent int, llmTimeout time.Duration) *Service {
	if maxConcurrent < 1 {
		maxConcurrent = 1
	}
	return &Service{
		repo:       repo,
		llm:        model,
		insights:   insights,
		llmTimeout: llmTimeout,
		genSem:     make(chan struct{}, maxConcurrent),
	}
}

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
