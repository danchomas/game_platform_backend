package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"beeline-games/internal/core/apperr"
	"beeline-games/internal/core/domain"
)

type fakeStats struct {
	filters  []domain.Filter
	insights []domain.Insight
	summary  domain.Summary
	err      error
	calls    int
}

func (f *fakeStats) Summary(_ context.Context, flt domain.Filter) (domain.Summary, error) {
	f.filters = append(f.filters, flt)
	return f.summary, f.err
}

func (f *fakeStats) Daily(context.Context, domain.Filter) ([]domain.DayPoint, error) {
	return nil, f.err
}

func (f *fakeStats) PerGame(context.Context, domain.Filter) ([]domain.GameRow, error) {
	return nil, f.err
}

func (f *fakeStats) CustomEvents(context.Context, domain.Filter) ([]domain.EventCount, error) {
	return nil, f.err
}

func (f *fakeStats) DurationBuckets(context.Context, domain.Filter) ([]domain.Bucket, error) {
	return nil, f.err
}

func (f *fakeStats) TopInsights(context.Context, int) ([]domain.Insight, error) {
	f.calls++
	return f.insights, f.err
}

type fakeOwnership struct {
	owned bool
	err   error
}

func (f fakeOwnership) CheckOwnership(context.Context, string, string) (bool, error) {
	return f.owned, f.err
}

func TestGetOverviewRestrictsPlatformScopeToManagers(t *testing.T) {
	svc := NewService(&fakeStats{}, fakeOwnership{})

	_, err := svc.GetOverview(context.Background(), "u1", domain.RolePlayer, domain.ScopeAll, 30)
	if !errors.Is(err, apperr.ErrForbidden) {
		t.Fatalf("игрок получил общий срез: %v", err)
	}

	data, err := svc.GetOverview(context.Background(), "u1", domain.RoleManager, domain.ScopeAll, 30)
	if err != nil {
		t.Fatalf("менеджер не получил общий срез: %v", err)
	}
	if data.Scope != domain.ScopeAll {
		t.Errorf("срез = %q, ожидался %q", data.Scope, domain.ScopeAll)
	}
}

func TestGetOverviewScopesToOwnerByDefault(t *testing.T) {
	stats := &fakeStats{}
	svc := NewService(stats, fakeOwnership{})

	data, err := svc.GetOverview(context.Background(), "u1", domain.RolePlayer, "", 30)
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if data.Scope != domain.ScopeMine {
		t.Errorf("срез = %q, ожидался %q", data.Scope, domain.ScopeMine)
	}
	if len(stats.filters) == 0 || stats.filters[0].OwnerID != "u1" {
		t.Error("фильтр не сузился до владельца")
	}
}

func TestDaysAreClampedByService(t *testing.T) {
	tests := []struct {
		name string
		days int
		want int
	}{
		{"ноль заменяется значением по умолчанию", 0, defaultDays},
		{"отрицательное заменяется значением по умолчанию", -10, defaultDays},
		{"запредельное подрезается", 100_000, maxDays},
		{"обычное проходит как есть", 7, 7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewService(&fakeStats{}, fakeOwnership{})
			data, err := svc.GetOverview(context.Background(), "u1", domain.RolePlayer, "", tt.days)
			if err != nil {
				t.Fatalf("неожиданная ошибка: %v", err)
			}
			if data.Days != tt.want {
				t.Errorf("days = %d, ожидалось %d", data.Days, tt.want)
			}
		})
	}
}

func TestGetGameAnalyticsChecksOwnership(t *testing.T) {
	tests := []struct {
		name      string
		role      string
		owned     bool
		wantAllow bool
	}{
		{"владелец", domain.RolePlayer, true, true},
		{"чужой игрок", domain.RolePlayer, false, false},
		{"менеджер без владения", domain.RoleManager, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewService(&fakeStats{}, fakeOwnership{owned: tt.owned})

			_, err := svc.GetGameAnalytics(context.Background(), "u1", tt.role, "g1", 30)
			switch {
			case tt.wantAllow && err != nil:
				t.Fatalf("доступ закрыт напрасно: %v", err)
			case !tt.wantAllow && !errors.Is(err, apperr.ErrForbidden):
				t.Fatalf("ожидался ErrForbidden, получено: %v", err)
			}
		})
	}
}

func TestGetGameAnalyticsFiltersByGame(t *testing.T) {
	stats := &fakeStats{}
	svc := NewService(stats, fakeOwnership{owned: true})

	if _, err := svc.GetGameAnalytics(context.Background(), "u1", domain.RolePlayer, "g1", 7); err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if len(stats.filters) == 0 || stats.filters[0].GameID != "g1" {
		t.Error("фильтр не сузился до игры")
	}
}

func TestGetInsightsHintCaches(t *testing.T) {
	stats := &fakeStats{insights: []domain.Insight{
		{Title: "Змейка", Genre: "аркада", Audience: "все", Sessions: 120, AvgSessionSec: 95, WinRate: 0.42},
	}}
	svc := NewService(stats, fakeOwnership{})

	first := svc.GetInsightsHint(context.Background())
	if !strings.Contains(first, "Змейка") {
		t.Fatalf("подсказка не собрана: %q", first)
	}

	for range 3 {
		svc.GetInsightsHint(context.Background())
	}
	if stats.calls != 1 {
		t.Errorf("хранилище опрошено %d раз, ожидался 1", stats.calls)
	}
}

func TestGetInsightsHintSwallowsRepositoryError(t *testing.T) {
	stats := &fakeStats{err: errors.New("база недоступна")}
	svc := NewService(stats, fakeOwnership{})

	if hint := svc.GetInsightsHint(context.Background()); hint != "" {
		t.Errorf("при ошибке ожидалась пустая подсказка, получено: %q", hint)
	}
}
