package service

import (
	"context"
	"slices"
	"time"

	"github.com/google/uuid"

	"beeline-games/internal/core/apperr"
	"beeline-games/internal/core/bandit"
	"beeline-games/internal/core/domain"
)

const (
	// posteriorTTL — как часто пересчитывать оценки вариантов: запрос по событиям
	// дороже, чем выбор варианта, а за 30 секунд картина почти не меняется.
	posteriorTTL = 30 * time.Second
	// settleWindow — сколько ждать исхода показа, прежде чем учесть его бандитом.
	settleWindow = 5 * time.Minute

	// Автоматическая победа: лидер лучший с вероятностью autoWinProb, у него не меньше
	// autoMinTrials учтённых показов, а у остальных — хотя бы autoMinRivalTrials.
	// Требовать много показов от каждого нельзя: бандит почти перестаёт показывать
	// проигравших, и порог не набрался бы никогда. Их неопределённость и так учтена
	// в апостериорном распределении, а минимум отсекает решение по паре сессий.
	autoWinProb        = 0.95
	autoMinTrials      = 30
	autoMinRivalTrials = 10

	probBestDraws = 4000
)

// posterior — апостериорная картина эксперимента: руки в порядке live.
type posterior struct {
	live  []string
	arms  []bandit.Arm
	probs []float64
	exp   time.Time
}

// winner — вариант, который бандит может объявить победителем; пусто — рано.
func (p posterior) winner() string {
	if len(p.live) < minVariants {
		return ""
	}
	best := p.leader()
	if p.probs[best] < autoWinProb || p.arms[best].Trials < autoMinTrials {
		return ""
	}
	for i, a := range p.arms {
		if i != best && a.Trials < autoMinRivalTrials {
			return ""
		}
	}
	return p.live[best]
}

func (p posterior) leader() int {
	best := 0
	for i, prob := range p.probs {
		if prob > p.probs[best] {
			best = i
		}
	}
	return best
}

func (p posterior) probOf(gameID string) float64 {
	if i := slices.Index(p.live, gameID); i >= 0 {
		return p.probs[i]
	}
	return 0
}

// PickVariant выбирает вариант для игрока по ссылке эксперимента.
//
// Новый игрок получает вариант по Thompson sampling и закрепляется за ним: при
// повторном заходе он увидит ту же игру, иначе его вторая сессия смешала бы
// впечатления от двух вариантов. Когда лидер становится лучшим с вероятностью
// autoWinProb, эксперимент завершается сам, и дальше все получают победителя.
func (s *Service) PickVariant(ctx context.Context, id, playerID string) (string, error) {
	if uuid.Validate(id) != nil {
		return "", apperr.ErrNotFound
	}
	rt, err := s.repo.Routing(ctx, id)
	if err != nil {
		return "", err
	}
	switch {
	case rt.Status == domain.ExperimentFinished && rt.WinnerID != "":
		return rt.WinnerID, nil
	case rt.Status != domain.ExperimentRunning || len(rt.Live) == 0:
		return "", apperr.ErrNotFound
	}

	assigned, err := s.repo.Assignment(ctx, id, playerID)
	if err != nil {
		return "", err
	}
	if slices.Contains(rt.Live, assigned) {
		return assigned, nil
	}

	post, err := s.posterior(ctx, id, rt.Live)
	if err != nil {
		return "", err
	}
	if winner := post.winner(); winner != "" {
		ok, err := s.finish(ctx, id, winner, domain.WinnerAuto)
		if err != nil {
			return "", err
		}
		if ok {
			return winner, nil
		}
	}

	gameID := rt.Live[bandit.Pick(post.arms, s.rnd)]
	if err := s.repo.Assign(ctx, id, playerID, gameID); err != nil {
		return "", err
	}
	return gameID, nil
}

// posterior отдаёт оценки вариантов из кэша или пересчитывает их. Кэш сбрасывается
// и при смене набора live: снятый с эксперимента вариант не должен получать трафик.
func (s *Service) posterior(ctx context.Context, id string, live []string) (posterior, error) {
	now := s.now()
	s.postMu.Lock()
	p, ok := s.posteriors[id]
	s.postMu.Unlock()
	if ok && now.Before(p.exp) && slices.Equal(p.live, live) {
		return p, nil
	}

	stats, err := s.repo.Arms(ctx, id, now.Add(-settleWindow))
	if err != nil {
		return posterior{}, err
	}
	byGame := make(map[string]domain.ArmStats, len(stats))
	for _, a := range stats {
		byGame[a.GameID] = a
	}
	p = posterior{live: slices.Clone(live), arms: make([]bandit.Arm, len(live)), exp: now.Add(posteriorTTL)}
	for i, gameID := range live {
		a := byGame[gameID]
		p.arms[i] = bandit.Arm{Successes: a.Successes, Trials: a.Trials}
	}
	p.probs = bandit.ProbBest(p.arms, s.rnd, probBestDraws)

	s.postMu.Lock()
	s.posteriors[id] = p
	s.postMu.Unlock()
	return p, nil
}

// finish записывает победителя и сбрасывает кэш оценок эксперимента.
func (s *Service) finish(ctx context.Context, id, gameID, source string) (bool, error) {
	ok, err := s.repo.Finish(ctx, id, gameID, source)
	if err != nil {
		return false, err
	}
	s.postMu.Lock()
	delete(s.posteriors, id)
	s.postMu.Unlock()
	return ok, nil
}
