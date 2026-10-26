package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"beeline-games/internal/core/apperr"
	"beeline-games/internal/core/domain"
)

func TestPickVariantRouting(t *testing.T) {
	tests := []struct {
		name     string
		status   string
		winner   string
		variants []domain.Variant
		id       string
		want     string
		err      error
	}{
		{name: "draft is hidden", status: domain.ExperimentDraft, variants: []domain.Variant{live("A"), live("B")}, err: apperr.ErrNotFound},
		{name: "running without live variants", status: domain.ExperimentRunning, variants: []domain.Variant{ready("A")}, err: apperr.ErrNotFound},
		{name: "finished serves winner", status: domain.ExperimentFinished, winner: "game-B", variants: []domain.Variant{live("A"), live("B")}, want: "game-B"},
		{name: "finished without winner", status: domain.ExperimentFinished, variants: []domain.Variant{live("A")}, err: apperr.ErrNotFound},
		{name: "malformed id", status: domain.ExperimentRunning, variants: []domain.Variant{live("A")}, id: "nope", err: apperr.ErrNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakeRepo()
			repo.seed(tt.status, tt.variants...)
			x := repo.experiments[expID]
			x.WinnerID = tt.winner
			repo.experiments[expID] = x
			svc, _ := newBanditService(repo)

			id := expID
			if tt.id != "" {
				id = tt.id
			}
			got, err := svc.PickVariant(context.Background(), id, "p1")
			if !errors.Is(err, tt.err) || got != tt.want {
				t.Errorf("PickVariant = %q, %v; ожидалось %q, %v", got, err, tt.want, tt.err)
			}
		})
	}
}

func TestPickVariantKeepsReturningPlayer(t *testing.T) {
	repo := newFakeRepo()
	repo.seed(domain.ExperimentRunning, live("A"), live("B"))
	repo.assignments["p1"] = "game-B"
	// данные говорят в пользу A, но вернувшийся игрок остаётся в своём варианте
	repo.arms = []domain.ArmStats{{GameID: "game-A", Trials: 200, Successes: 150}, {GameID: "game-B", Trials: 200, Successes: 10}}
	svc, _ := newBanditService(repo)

	for range 5 {
		if got, err := svc.PickVariant(context.Background(), expID, "p1"); err != nil || got != "game-B" {
			t.Fatalf("вернувшийся игрок получил %q, %v; ожидался его game-B", got, err)
		}
	}
	if repo.armsCalls != 0 {
		t.Errorf("для закреплённого игрока бандит не нужен, а оценки запрошены %d раз", repo.armsCalls)
	}
}

func TestPickVariantReassignsWhenVariantRemoved(t *testing.T) {
	repo := newFakeRepo()
	repo.seed(domain.ExperimentRunning, live("A"), live("B"), ready("C"))
	repo.assignments["p1"] = "game-C"
	svc, _ := newBanditService(repo)

	got, err := svc.PickVariant(context.Background(), expID, "p1")
	if err != nil || (got != "game-A" && got != "game-B") {
		t.Fatalf("игрок снятого варианта получил %q, %v", got, err)
	}
	if repo.assignments["p1"] != got {
		t.Errorf("новое закрепление не сохранено: %q", repo.assignments["p1"])
	}
}

func TestPickVariantSplitsEvenlyWithoutData(t *testing.T) {
	repo := newFakeRepo()
	repo.seed(domain.ExperimentRunning, live("A"), live("B"), live("C"))
	svc, _ := newBanditService(repo)

	counts := pickMany(t, svc, 900)
	for _, id := range []string{"game-A", "game-B", "game-C"} {
		// без данных Thompson делит поровну: по 300 ± случайный разброс
		if counts[id] < 220 || counts[id] > 380 {
			t.Errorf("вариант %s получил %d из 900 новых игроков (%v)", id, counts[id], counts)
		}
	}
	if len(repo.assignments) != 900 {
		t.Errorf("закреплено %d игроков, ожидалось 900", len(repo.assignments))
	}
}

func TestPickVariantShiftsTrafficToBetterVariant(t *testing.T) {
	repo := newFakeRepo()
	repo.seed(domain.ExperimentRunning, live("A"), live("B"))
	// 48% против 25%: B, скорее всего, хуже (P ≈ 3%), но не наверняка, и у A
	// меньше autoMinTrials показов — автопобеды ещё нет
	repo.arms = []domain.ArmStats{{GameID: "game-A", Trials: 25, Successes: 12}, {GameID: "game-B", Trials: 40, Successes: 10}}
	svc, _ := newBanditService(repo)

	counts := pickMany(t, svc, 400)
	if counts["game-A"] < 340 {
		t.Errorf("лучший вариант получил %d из 400 новых игроков, ожидалось подавляющее большинство (%v)", counts["game-A"], counts)
	}
	if counts["game-B"] == 0 {
		t.Error("вариант с шансом оказаться лучшим совсем перестал показываться: исследования нет")
	}
	if x := repo.experiment(); x.Status != domain.ExperimentRunning {
		t.Errorf("при %d показах у A эксперимент не должен завершаться сам", repo.arms[0].Trials)
	}
}

func TestPickVariantFinishesExperimentAutomatically(t *testing.T) {
	repo := newFakeRepo()
	repo.seed(domain.ExperimentRunning, live("A"), live("B"))
	repo.arms = []domain.ArmStats{{GameID: "game-A", Trials: 120, Successes: 18}, {GameID: "game-B", Trials: 110, Successes: 60}}
	svc, _ := newBanditService(repo)

	got, err := svc.PickVariant(context.Background(), expID, "new-player")
	if err != nil || got != "game-B" {
		t.Fatalf("PickVariant = %q, %v; ожидался победитель game-B", got, err)
	}
	x := repo.experiment()
	if x.Status != domain.ExperimentFinished || x.WinnerID != "game-B" || x.WinnerSource != domain.WinnerAuto {
		t.Errorf("эксперимент не завершён автоматически: %+v", x)
	}
	// дальше все игроки, включая закреплённых за проигравшим, получают победителя
	repo.assignments["old"] = "game-A"
	if got, _ := svc.PickVariant(context.Background(), expID, "old"); got != "game-B" {
		t.Errorf("после победы игрок получил %q", got)
	}
}

func TestAutoWinnerNeedsEnoughData(t *testing.T) {
	tests := []struct {
		name string
		arms []domain.ArmStats
		want string
	}{
		{name: "clear and enough data", arms: []domain.ArmStats{{GameID: "game-A", Trials: 100, Successes: 10}, {GameID: "game-B", Trials: 100, Successes: 50}}, want: "game-B"},
		{name: "clear but leader underexposed", arms: []domain.ArmStats{{GameID: "game-A", Trials: 100, Successes: 10}, {GameID: "game-B", Trials: 20, Successes: 18}}},
		{name: "rival barely tried", arms: []domain.ArmStats{{GameID: "game-A", Trials: 5, Successes: 0}, {GameID: "game-B", Trials: 100, Successes: 60}}},
		// бандит перестал показывать проигравшего, но его 12 показов уже говорят достаточно
		{name: "starved loser does not block", arms: []domain.ArmStats{{GameID: "game-A", Trials: 12, Successes: 1}, {GameID: "game-B", Trials: 150, Successes: 80}}, want: "game-B"},
		{name: "enough data but close", arms: []domain.ArmStats{{GameID: "game-A", Trials: 100, Successes: 40}, {GameID: "game-B", Trials: 100, Successes: 44}}},
		{name: "no data", arms: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakeRepo()
			repo.seed(domain.ExperimentRunning, live("A"), live("B"))
			repo.arms = tt.arms
			svc, _ := newBanditService(repo)

			p, err := svc.posterior(context.Background(), expID, []string{"game-A", "game-B"})
			if err != nil {
				t.Fatal(err)
			}
			if got := p.winner(); got != tt.want {
				t.Errorf("winner = %q, ожидалось %q (P(best) = %v)", got, tt.want, p.probs)
			}
		})
	}
}

func TestPosteriorUsesSettledExposuresAndCache(t *testing.T) {
	repo := newFakeRepo()
	repo.seed(domain.ExperimentRunning, live("A"), live("B"))
	svc, clock := newBanditService(repo)

	pickMany(t, svc, 10)
	if repo.armsCalls != 1 {
		t.Errorf("оценки пересчитаны %d раз за 10 игроков, ожидался 1 (кэш)", repo.armsCalls)
	}
	if want := clock.t.Add(-settleWindow); !repo.armsBefore.Equal(want) {
		t.Errorf("бандит учитывает показы до %v, ожидалось до %v", repo.armsBefore, want)
	}

	clock.t = clock.t.Add(posteriorTTL + time.Second)
	if _, err := svc.PickVariant(context.Background(), expID, "late"); err != nil {
		t.Fatal(err)
	}
	if repo.armsCalls != 2 {
		t.Errorf("после истечения кэша оценки не пересчитаны: %d вызовов", repo.armsCalls)
	}

	// снятый с эксперимента вариант сбрасывает кэш сразу, не дожидаясь TTL
	repo.variants[expID][1].Published = false
	if got, _ := svc.PickVariant(context.Background(), expID, "after-unpublish"); got != "game-A" {
		t.Errorf("снятый вариант всё ещё получает игроков: %q", got)
	}
	if repo.armsCalls != 3 {
		t.Errorf("смена набора вариантов не сбросила кэш: %d вызовов", repo.armsCalls)
	}
}

func TestReport(t *testing.T) {
	repo := newFakeRepo()
	repo.seed(domain.ExperimentDraft, live("A"), live("B"), ready("C"))
	svc, _ := newBanditService(repo)

	rep, err := svc.Report(context.Background(), ownerID, expID)
	if err != nil {
		t.Fatal(err)
	}
	if rep.LeaderID != "" || repo.armsCalls != 0 {
		t.Errorf("у черновика не должно быть лидера и оценок: %+v", rep)
	}

	if _, _, err := svc.Launch(context.Background(), ownerID, expID); err != nil {
		t.Fatal(err)
	}
	repo.stats = []domain.VariantStats{
		{GameID: "game-A", Exposures: 30}, {GameID: "game-B", Exposures: 90}, {GameID: "game-C"},
	}
	repo.arms = []domain.ArmStats{{GameID: "game-A", Trials: 8, Successes: 1}, {GameID: "game-B", Trials: 80, Successes: 40}}

	rep, err = svc.Report(context.Background(), ownerID, expID)
	if err != nil {
		t.Fatal(err)
	}
	if want := *repo.experiment().LaunchedAt; !repo.statsSince.Equal(want) {
		t.Errorf("воронка с %v, ожидалась с момента запуска %v", repo.statsSince, want)
	}
	byGame := map[string]domain.VariantStats{}
	for _, st := range rep.Stats {
		byGame[st.GameID] = st
	}
	if a, b := byGame["game-A"], byGame["game-B"]; a.TrafficShare != 0.25 || b.TrafficShare != 0.75 {
		t.Errorf("доли трафика A=%v B=%v, ожидалось 0.25 и 0.75", a.TrafficShare, b.TrafficShare)
	}
	if p := byGame["game-B"].ProbBest; p < 0.95 {
		t.Errorf("P(B лучший) = %v, ожидалась почти 1", p)
	}
	if byGame["game-C"].ProbBest != 0 {
		t.Error("вариант вне эксперимента получил вероятность быть лучшим")
	}
	if rep.LeaderID != "game-B" {
		t.Errorf("лидер %q, ожидался game-B", rep.LeaderID)
	}
	// у A меньше autoMinRivalTrials учтённых показов: автопобеды ещё нет
	if rep.Significant || rep.Experiment.Status != domain.ExperimentRunning {
		t.Errorf("эксперимент завершён по недостаточным данным: %+v", rep.Experiment)
	}
}

func TestReportFinishesExperimentAutomatically(t *testing.T) {
	repo := newFakeRepo()
	repo.seed(domain.ExperimentRunning, live("A"), live("B"))
	repo.arms = []domain.ArmStats{{GameID: "game-A", Trials: 100, Successes: 10}, {GameID: "game-B", Trials: 100, Successes: 55}}
	svc, _ := newBanditService(repo)

	rep, err := svc.Report(context.Background(), ownerID, expID)
	if err != nil {
		t.Fatal(err)
	}
	if !rep.Significant || rep.Experiment.Status != domain.ExperimentFinished ||
		rep.Experiment.WinnerID != "game-B" || rep.Experiment.WinnerSource != domain.WinnerAuto {
		t.Errorf("отчёт не завершил эксперимент победой B: %+v", rep.Experiment)
	}
}

func pickMany(t *testing.T, svc *Service, players int) map[string]int {
	t.Helper()
	counts := map[string]int{}
	for i := range players {
		got, err := svc.PickVariant(context.Background(), expID, fmt.Sprintf("player-%d", i))
		if err != nil {
			t.Fatal(err)
		}
		counts[got]++
	}
	return counts
}
