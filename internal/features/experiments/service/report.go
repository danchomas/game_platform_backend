package service

import (
	"context"

	"beeline-games/internal/core/domain"
)

// Report — дашборд эксперимента: воронки вариантов с момента запуска, доля трафика,
// которую им отдал бандит, и вероятность каждого оказаться лучшим. Если лидер уже
// набрал порог автоматической победы, отчёт сам завершает эксперимент: автор не
// должен ждать следующего игрока, чтобы увидеть итог.
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
	rep := domain.ExperimentReport{
		Experiment: x, Variants: variants, Stats: stats,
		AutoWinProb: autoWinProb, AutoMinTrials: autoMinTrials, AutoMinRivalTrials: autoMinRivalTrials,
	}
	if x.Status == domain.ExperimentDraft {
		return rep, nil
	}

	post, err := s.posterior(ctx, id, liveVariants(variants))
	if err != nil {
		return domain.ExperimentReport{}, err
	}
	var total int64
	for _, st := range stats {
		total += st.Exposures
	}
	for i := range rep.Stats {
		st := &rep.Stats[i]
		st.ProbBest = post.probOf(st.GameID)
		if total > 0 {
			st.TrafficShare = float64(st.Exposures) / float64(total)
		}
	}
	if len(post.live) > 0 {
		rep.LeaderID = post.live[post.leader()]
	}

	winner := post.winner()
	rep.Significant = winner != ""
	if winner != "" && x.Status == domain.ExperimentRunning {
		ok, err := s.finish(ctx, id, winner, domain.WinnerAuto)
		if err != nil {
			return domain.ExperimentReport{}, err
		}
		if ok {
			if rep.Experiment, err = s.repo.Get(ctx, id); err != nil {
				return domain.ExperimentReport{}, err
			}
		}
	}
	return rep, nil
}

// liveVariants — варианты, которые раздаёт ссылка, в порядке меток (как в Routing).
func liveVariants(variants []domain.Variant) []string {
	live := make([]string, 0, len(variants))
	for _, v := range variants {
		if v.Published && v.Version > 0 {
			live = append(live, v.GameID)
		}
	}
	return live
}
