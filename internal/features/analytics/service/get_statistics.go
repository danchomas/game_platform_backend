package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"beeline-games/internal/core/domain"
	coreErrs "beeline-games/internal/core/errors"
)

func par(fns ...func() error) error {
	var wg sync.WaitGroup
	errs := make([]error, len(fns))
	for i, f := range fns {
		wg.Add(1)
		go func(idx int, fn func() error) {
			defer wg.Done()
			errs[idx] = fn()
		}(i, f)
	}
	wg.Wait()
	for _, e := range errs {
		if e != nil {
			return e
		}
	}
	return nil
}

func dateRange(days int) (time.Time, time.Time) {
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	return today.AddDate(0, 0, -(days - 1)), today.AddDate(0, 0, 1)
}

func (s *service) GetOverview(ctx context.Context, userID, role, scope string, days int) (DashboardData, error) {
	from, to := dateRange(days)
	f := domain.Filter{From: from, To: to}

	actualScope := "mine"
	if scope == "all" {
		if role != "manager" {
			return DashboardData{}, coreErrs.ErrForbidden
		}
		actualScope = "all"
	} else {
		f.OwnerID = userID
	}

	var data DashboardData
	data.Scope = actualScope
	data.Days = days

	err := par(
		func() (e error) { data.Summary, e = s.repo.Summary(ctx, f); return },
		func() (e error) { data.Daily, e = s.repo.Daily(ctx, f); return },
		func() (e error) { data.Games, e = s.repo.PerGame(ctx, f); return },
		func() (e error) { data.CustomEvents, e = s.repo.CustomEvents(ctx, f); return },
		func() (e error) { data.SessionDuration, e = s.repo.DurationBuckets(ctx, f); return },
	)
	if err != nil {
		return DashboardData{}, err
	}
	return data, nil
}

func (s *service) GetGameAnalytics(ctx context.Context, userID, role, gameID string, days int) (DashboardData, error) {

	from, to := dateRange(days)
	f := domain.Filter{From: from, To: to, GameID: gameID}

	var data DashboardData
	data.Days = days

	err := par(
		func() (e error) { data.Summary, e = s.repo.Summary(ctx, f); return },
		func() (e error) { data.Daily, e = s.repo.Daily(ctx, f); return },
		func() (e error) { data.CustomEvents, e = s.repo.CustomEvents(ctx, f); return },
		func() (e error) { data.SessionDuration, e = s.repo.DurationBuckets(ctx, f); return },
	)
	if err != nil {
		return DashboardData{}, err
	}
	return data, nil
}

func (s *service) GetInsightsHint(ctx context.Context) string {
	s.hintMu.Lock()
	defer s.hintMu.Unlock()

	if time.Now().Before(s.hintExp) {
		return s.hint
	}

	s.hintExp = time.Now().Add(5 * time.Minute)
	ins, err := s.repo.TopInsights(ctx, 3)
	if err != nil || len(ins) == 0 {
		s.hint = ""
		return ""
	}

	var b strings.Builder
	for _, in := range ins {
		fmt.Fprintf(&b, "- \"%s\" (genre: %s, audience: %s): %d sessions, avg session %.0fs, win rate %.0f%%\n",
			in.Title, in.Genre, in.Audience, in.Sessions, in.AvgSessionSec, in.WinRate*100)
	}
	s.hint = b.String()
	return s.hint
}
