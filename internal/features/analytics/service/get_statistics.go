package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"beeline-games/internal/core/apperr"
	"beeline-games/internal/core/domain"
)

func (s *Service) GetOverview(ctx context.Context, userID, role, scope string, days int) (domain.Dashboard, error) {
	days = clampDays(days)
	from, to := dateRange(days)

	f := domain.Filter{From: from, To: to}
	actualScope := domain.ScopeMine

	if scope == domain.ScopeAll {
		if role != domain.RoleManager {
			return domain.Dashboard{}, fmt.Errorf("общая аналитика доступна только менеджерам: %w", apperr.ErrForbidden)
		}
		actualScope = domain.ScopeAll
	} else {
		f.OwnerID = userID
	}

	data := domain.Dashboard{Scope: actualScope, Days: days}

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() (err error) { data.Summary, err = s.repo.Summary(gctx, f); return })
	g.Go(func() (err error) { data.Daily, err = s.repo.Daily(gctx, f); return })
	g.Go(func() (err error) { data.Games, err = s.repo.PerGame(gctx, f); return })
	g.Go(func() (err error) { data.CustomEvents, err = s.repo.CustomEvents(gctx, f); return })
	g.Go(func() (err error) { data.SessionDuration, err = s.repo.DurationBuckets(gctx, f); return })

	if err := g.Wait(); err != nil {
		return domain.Dashboard{}, err
	}
	return data, nil
}

func (s *Service) GetGameAnalytics(ctx context.Context, userID, role, gameID string, days int) (domain.Dashboard, error) {
	if err := s.authorizeGame(ctx, userID, role, gameID); err != nil {
		return domain.Dashboard{}, err
	}

	days = clampDays(days)
	from, to := dateRange(days)
	f := domain.Filter{From: from, To: to, GameID: gameID}

	data := domain.Dashboard{Scope: domain.ScopeMine, Days: days}

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() (err error) { data.Summary, err = s.repo.Summary(gctx, f); return })
	g.Go(func() (err error) { data.Daily, err = s.repo.Daily(gctx, f); return })
	g.Go(func() (err error) { data.CustomEvents, err = s.repo.CustomEvents(gctx, f); return })
	g.Go(func() (err error) { data.SessionDuration, err = s.repo.DurationBuckets(gctx, f); return })

	if err := g.Wait(); err != nil {
		return domain.Dashboard{}, err
	}
	return data, nil
}

func (s *Service) authorizeGame(ctx context.Context, userID, role, gameID string) error {
	if role == domain.RoleManager {
		return nil
	}

	owned, err := s.games.CheckOwnership(ctx, userID, gameID)
	if err != nil {
		return err
	}
	if !owned {
		return apperr.ErrForbidden
	}
	return nil
}

func (s *Service) GetInsightsHint(ctx context.Context) string {
	s.hintMu.Lock()
	defer s.hintMu.Unlock()

	if time.Now().Before(s.hintExp) {
		return s.hint
	}
	s.hintExp = time.Now().Add(hintTTL)

	insights, err := s.repo.TopInsights(ctx, hintGames)
	if err != nil || len(insights) == 0 {
		s.hint = ""
		return ""
	}

	var b strings.Builder
	for _, in := range insights {
		fmt.Fprintf(&b, "- %q (genre: %s, audience: %s): %d sessions, avg session %.0fs, win rate %.0f%%\n",
			in.Title, in.Genre, in.Audience, in.Sessions, in.AvgSessionSec, in.WinRate*100)
	}
	s.hint = b.String()
	return s.hint
}

func dateRange(days int) (from, to time.Time) {
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	return today.AddDate(0, 0, -(days - 1)), today.AddDate(0, 0, 1)
}
