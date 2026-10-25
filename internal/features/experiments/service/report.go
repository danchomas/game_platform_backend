package service

import (
	"context"
	"hash/fnv"

	"github.com/google/uuid"

	"beeline-games/internal/core/apperr"
	"beeline-games/internal/core/domain"
)

// Report — дашборд эксперимента: воронки вариантов с момента запуска и текущий лидер.
func (s *Service) Report(ctx context.Context, userID, id string) (domain.ExperimentReport, error) {
	x, err := s.owned(ctx, userID, id)
	if err != nil {
		return domain.ExperimentReport{}, err
	}
	variants, err := s.repo.Variants(ctx, id)
	if err != nil {
		return domain.ExperimentReport{}, err
	}

	since := x.CreatedAt
	if x.LaunchedAt != nil {
		since = *x.LaunchedAt
	}
	stats, err := s.repo.VariantStats(ctx, id, since)
	if err != nil {
		return domain.ExperimentReport{}, err
	}

	rep := domain.ExperimentReport{Experiment: x, Variants: variants, Stats: stats}
	rep.LeaderID, rep.Significant = leader(stats)
	return rep, nil
}

// leader — вариант с лучшей нижней границей интервала удержания. Это осторожная оценка:
// пара удачных сессий на маленькой выборке её не переворачивает. significant — интервал
// лидера целиком выше интервалов всех остальных показанных вариантов.
func leader(stats []domain.VariantStats) (id string, significant bool) {
	best := -1
	for i, st := range stats {
		if st.Exposures == 0 {
			continue
		}
		if best < 0 || st.RetentionLow > stats[best].RetentionLow {
			best = i
		}
	}
	if best < 0 {
		return "", false
	}

	rivals := 0
	significant = true
	for i, st := range stats {
		if i == best || st.Exposures == 0 {
			continue
		}
		rivals++
		if st.RetentionHigh >= stats[best].RetentionLow {
			significant = false
		}
	}
	return stats[best].GameID, significant && rivals > 0
}

// PickVariant выбирает вариант для игрока по ссылке эксперимента. Пока идёт эксперимент,
// игрок закреплён за вариантом хэшем своего идентификатора: при повторном заходе он увидит
// ту же игру. После выбора победителя все получают победителя.
func (s *Service) PickVariant(ctx context.Context, id, playerID string) (string, error) {
	if uuid.Validate(id) != nil {
		return "", apperr.ErrNotFound
	}
	rt, err := s.repo.Routing(ctx, id)
	if err != nil {
		return "", err
	}
	return route(rt, id, playerID)
}

func route(rt domain.ExperimentRouting, id, playerID string) (string, error) {
	switch {
	case rt.Status == domain.ExperimentFinished && rt.WinnerID != "":
		return rt.WinnerID, nil
	case rt.Status == domain.ExperimentRunning && len(rt.Live) > 0:
		h := fnv.New32a()
		_, _ = h.Write([]byte(id + ":" + playerID))
		return rt.Live[h.Sum32()%uint32(len(rt.Live))], nil
	}
	return "", apperr.ErrNotFound
}
