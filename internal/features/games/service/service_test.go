package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"beeline-games/internal/core/apperr"
	"beeline-games/internal/core/domain"
)

func newTestService(repo GameRepository, ag Agent) *Service {
	return NewService(context.Background(), repo, ag, staticInsights{}, 4)
}

func TestCreateGameReturnsPlanAwaitingApproval(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo, &fakeAgent{spec: snakeSpec()})

	game, plan, err := svc.CreateGame(context.Background(), "owner-1", "змейка для бабушки", false)
	if err != nil {
		t.Fatalf("план не составлен: %v", err)
	}
	if game.Status != domain.StatusAwaitingApproval {
		t.Errorf("статус = %q, ожидался %q", game.Status, domain.StatusAwaitingApproval)
	}
	if plan.Spec.Title != "Змейка" || plan.Reply == "" {
		t.Errorf("план потерян: %+v", plan)
	}
	if len(game.Spec) == 0 {
		t.Error("план не сохранён в игре")
	}
	if game.Version != 0 {
		t.Errorf("до подтверждения кода быть не должно, версия %d", game.Version)
	}

	msgs, _ := repo.Messages(context.Background(), game.ID)
	if len(msgs) != 2 || msgs[0].Role != "user" || msgs[1].Content != plan.Reply {
		t.Errorf("в чате нет запроса и плана: %+v", msgs)
	}
	if gen, ok := repo.lastGen(domain.GenPlan); !ok || gen.Status != domain.GenStatusReady || len(gen.Calls) == 0 {
		t.Errorf("запуск планировщика не записан: %+v", gen)
	}
}

func TestCreateGameRejectsShortPrompt(t *testing.T) {
	svc := newTestService(newFakeRepo(), &fakeAgent{})

	_, _, err := svc.CreateGame(context.Background(), "owner-1", " a ", false)

	var vErr *apperr.ValidationError
	if !errors.As(err, &vErr) || vErr.Field != "prompt" {
		t.Fatalf("ожидалась ошибка валидации поля prompt, получено: %v", err)
	}
}

func TestCreateGameKeepsFailedDraftWhenPlanFails(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo, &fakeAgent{planErr: apperr.ErrLLMFailed})

	_, _, err := svc.CreateGame(context.Background(), "owner-1", "сделай змейку", false)
	if !errors.Is(err, apperr.ErrLLMFailed) {
		t.Fatalf("ожидалась ErrLLMFailed, получено: %v", err)
	}
	g := repo.game("game-a")
	if g.Status != domain.StatusFailed || g.StatusDetail == "" {
		t.Errorf("черновик не помечен как failed: %+v", g)
	}
	if gen, _ := repo.lastGen(domain.GenPlan); gen.Status != domain.GenStatusFailed {
		t.Errorf("неудачный план не попал в статистику: %+v", gen)
	}
}

func TestApprovePlanBuildsInBackground(t *testing.T) {
	repo := newFakeRepo()
	ag := &fakeAgent{spec: snakeSpec(), html: gameHTML("let snake;")}
	svc := newTestService(repo, ag)
	ctx := context.Background()

	game, _, err := svc.CreateGame(ctx, "owner-1", "змейка", false)
	if err != nil {
		t.Fatalf("план: %v", err)
	}
	started, err := svc.ApprovePlan(ctx, "owner-1", game.ID, nil)
	if err != nil {
		t.Fatalf("подтверждение: %v", err)
	}
	if !domain.Busy(started.Status) {
		t.Errorf("после подтверждения сборка должна идти, статус %q", started.Status)
	}

	svc.Wait()

	g := repo.game(game.ID)
	if g.Status != domain.StatusReady || g.Version != 1 || !g.Generated() {
		t.Fatalf("игра не собрана: статус %q, версия %d", g.Status, g.Version)
	}
	if ag.lastBuild.Spec == nil || ag.lastBuild.Spec.Title != "Змейка" {
		t.Error("сборка шла не по утверждённому плану")
	}
	gen, _ := repo.lastGen(domain.GenBuild)
	if gen.Status != domain.GenStatusReady || gen.Version == nil || *gen.Version != 1 {
		t.Errorf("запуск сборки записан неверно: %+v", gen)
	}
}

func TestApprovePlanUsesEditedSpec(t *testing.T) {
	repo := newFakeRepo()
	ag := &fakeAgent{spec: snakeSpec(), html: gameHTML("let x;")}
	svc := newTestService(repo, ag)
	ctx := context.Background()

	game, _, _ := svc.CreateGame(ctx, "owner-1", "змейка", false)
	edited := snakeSpec()
	edited.Controls.Scheme = "buttons"
	if _, err := svc.ApprovePlan(ctx, "owner-1", game.ID, &edited); err != nil {
		t.Fatalf("подтверждение: %v", err)
	}
	svc.Wait()

	if ag.lastBuild.Spec == nil || ag.lastBuild.Spec.Controls.Scheme != "buttons" {
		t.Errorf("отредактированный план не дошёл до сборки: %+v", ag.lastBuild.Spec)
	}
}

func TestApprovePlanRejectsSecondBuild(t *testing.T) {
	repo := newFakeRepo()
	ag := &fakeAgent{spec: snakeSpec(), html: gameHTML("let x;"),
		block: make(chan struct{}), entered: make(chan struct{}, 1)}
	svc := newTestService(repo, ag)
	ctx := context.Background()

	game, _, _ := svc.CreateGame(ctx, "owner-1", "змейка", false)
	if _, err := svc.ApprovePlan(ctx, "owner-1", game.ID, nil); err != nil {
		t.Fatalf("первое подтверждение: %v", err)
	}
	<-ag.entered

	if _, err := svc.ApprovePlan(ctx, "owner-1", game.ID, nil); !errors.Is(err, apperr.ErrConflict) {
		t.Errorf("повторная сборка во время сборки: ожидался ErrConflict, получено %v", err)
	}
	if _, _, err := svc.RevisePlan(ctx, "owner-1", game.ID, "помедленнее"); !errors.Is(err, apperr.ErrConflict) {
		t.Errorf("правка плана во время сборки: ожидался ErrConflict, получено %v", err)
	}

	close(ag.block)
	svc.Wait()
}

func TestBuildWithUnfixedIssuesIsMarked(t *testing.T) {
	repo := newFakeRepo()
	ag := &fakeAgent{spec: snakeSpec(), html: gameHTML("let x;"),
		issues: []domain.Issue{{Code: "not_started", Severity: domain.SevError, Message: "x"}}}
	svc := newTestService(repo, ag)
	ctx := context.Background()

	game, _, _ := svc.CreateGame(ctx, "owner-1", "змейка", false)
	_, _ = svc.ApprovePlan(ctx, "owner-1", game.ID, nil)
	svc.Wait()

	g := repo.game(game.ID)
	if g.Version != 1 || !strings.Contains(g.StatusDetail, "not_started") {
		t.Errorf("игра с проблемами должна сохраниться с пояснением: %+v", g)
	}
	if gen, _ := repo.lastGen(domain.GenBuild); gen.Status != domain.GenStatusWithIssues {
		t.Errorf("статус запуска = %q, ожидался %q", gen.Status, domain.GenStatusWithIssues)
	}
}

func TestFailedFirstBuildKeepsPlanForRetry(t *testing.T) {
	repo := newFakeRepo()
	ag := &fakeAgent{spec: snakeSpec(), buildErr: apperr.ErrLLMFailed}
	svc := newTestService(repo, ag)
	ctx := context.Background()

	game, _, _ := svc.CreateGame(ctx, "owner-1", "змейка", false)
	_, _ = svc.ApprovePlan(ctx, "owner-1", game.ID, nil)
	svc.Wait()

	g := repo.game(game.ID)
	if g.Status != domain.StatusFailed || !g.PlanStage() {
		t.Fatalf("неудачная сборка должна вернуть игру к плану: %+v", g)
	}

	ag.buildErr, ag.html = nil, gameHTML("let x;")
	if _, err := svc.ApprovePlan(ctx, "owner-1", game.ID, nil); err != nil {
		t.Fatalf("повторная сборка недоступна: %v", err)
	}
	svc.Wait()
	if repo.game(game.ID).Version != 1 {
		t.Error("повторная сборка не сохранила версию")
	}
}

func TestStartVariantPlansAndBuildsWithoutApproval(t *testing.T) {
	repo := newFakeRepo()
	ag := &fakeAgent{spec: snakeSpec(), html: gameHTML("variant")}
	svc := newTestService(repo, ag)

	id, err := svc.StartVariant(context.Background(), domain.VariantInput{
		OwnerID: "owner-1", ExperimentID: "exp-1", Label: "B",
		Direction: "Драйв: короткие раунды", Prompt: "  змейка для бабушки ",
	})
	if err != nil {
		t.Fatalf("вариант не запущен: %v", err)
	}
	svc.Wait()

	g := repo.game(id)
	if g.Status != domain.StatusReady || g.Version != 1 {
		t.Errorf("вариант не собран: статус %q, версия %d", g.Status, g.Version)
	}
	if g.ExperimentID != "exp-1" || g.VariantLabel != "B" {
		t.Errorf("вариант не привязан к эксперименту: %+v", g)
	}
	if ag.lastPlan.Direction != "Драйв: короткие раунды" || ag.lastPlan.Prompt != "змейка для бабушки" {
		t.Errorf("планировщик не получил направление варианта: %+v", ag.lastPlan)
	}
	if _, ok := repo.lastGen(domain.GenBuild); !ok {
		t.Error("сборка варианта не записана в телеметрию")
	}
}

func TestStartVariantFailsWhenPlanFails(t *testing.T) {
	repo := newFakeRepo()
	ag := &fakeAgent{planErr: apperr.ErrLLMFailed}
	svc := newTestService(repo, ag)

	id, err := svc.StartVariant(context.Background(), domain.VariantInput{OwnerID: "owner-1", Label: "A", Prompt: "змейка"})
	if err != nil {
		t.Fatalf("вариант не запущен: %v", err)
	}
	svc.Wait()

	if g := repo.game(id); g.Status != domain.StatusFailed {
		t.Errorf("статус = %q, ожидался %q", g.Status, domain.StatusFailed)
	}
	if ag.builds != 0 {
		t.Errorf("без плана сборки быть не должно, было %d", ag.builds)
	}
}

func TestStartVariantRejectsShortPrompt(t *testing.T) {
	_, err := newTestService(newFakeRepo(), &fakeAgent{}).
		StartVariant(context.Background(), domain.VariantInput{Prompt: "a"})

	var vErr *apperr.ValidationError
	if !errors.As(err, &vErr) || vErr.Field != "prompt" {
		t.Fatalf("ожидалась ошибка валидации поля prompt, получено: %v", err)
	}
}

func TestRevisePlanSendsFeedbackAndCurrentPlan(t *testing.T) {
	repo := newFakeRepo()
	ag := &fakeAgent{spec: snakeSpec()}
	svc := newTestService(repo, ag)
	ctx := context.Background()

	game, _, _ := svc.CreateGame(ctx, "owner-1", "змейка", false)
	if _, _, err := svc.RevisePlan(ctx, "owner-1", game.ID, "сделай управление кнопками"); err != nil {
		t.Fatalf("правка плана: %v", err)
	}
	if ag.lastPlan.Current == nil || ag.lastPlan.Feedback != "сделай управление кнопками" {
		t.Errorf("планировщик не получил текущий план и отзыв: %+v", ag.lastPlan)
	}
	if repo.game(game.ID).Status != domain.StatusAwaitingApproval {
		t.Error("после правки план снова должен ждать подтверждения")
	}
}

func TestRevisePlanAfterBuildIsRejected(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo, &fakeAgent{})
	id := repo.seedGame("owner-1", gameHTML("let x;"))

	_, _, err := svc.RevisePlan(context.Background(), "owner-1", id, "поменяй жанр")
	if !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("собранную игру нельзя перепланировать: %v", err)
	}
}

func TestIterateGameEditsInBackground(t *testing.T) {
	repo := newFakeRepo()
	ag := &fakeAgent{html: gameHTML("let v2;")}
	svc := newTestService(repo, ag)
	id := repo.seedGame("owner-1", gameHTML("let v1;"))

	if _, err := svc.IterateGame(context.Background(), "owner-1", id, "сделай кнопки больше"); err != nil {
		t.Fatalf("правка: %v", err)
	}
	svc.Wait()

	g := repo.game(id)
	if g.Version != 2 || !strings.Contains(g.HTML, "let v2;") || g.Status != domain.StatusReady {
		t.Errorf("правка не применена: версия %d, статус %q", g.Version, g.Status)
	}
	if !strings.Contains(ag.lastBuild.CurrentHTML, "let v1;") || ag.lastBuild.Instruction != "сделай кнопки больше" {
		t.Error("агент не получил текущий код и просьбу")
	}
}

func TestFailedEditKeepsWorkingVersion(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo, &fakeAgent{buildErr: apperr.ErrLLMFailed})
	id := repo.seedGame("owner-1", gameHTML("let v1;"))

	_, _ = svc.IterateGame(context.Background(), "owner-1", id, "сделай кнопки больше")
	svc.Wait()

	g := repo.game(id)
	if g.Status != domain.StatusReady || g.Version != 1 || g.StatusDetail == "" {
		t.Errorf("после неудачной правки должна остаться рабочая версия с пояснением: %+v", g)
	}
}

func TestIterateGameRejectsConcurrentEdit(t *testing.T) {
	repo := newFakeRepo()
	ag := &fakeAgent{html: gameHTML("let x;"), block: make(chan struct{}), entered: make(chan struct{}, 1)}
	svc := newTestService(repo, ag)
	id := repo.seedGame("owner-1", gameHTML("let v1;"))
	ctx := context.Background()

	if _, err := svc.IterateGame(ctx, "owner-1", id, "сделай кнопки больше"); err != nil {
		t.Fatalf("первая правка: %v", err)
	}
	<-ag.entered

	if _, err := svc.IterateGame(ctx, "owner-1", id, "и фон поменяй"); !errors.Is(err, apperr.ErrConflict) {
		t.Errorf("ожидался ErrConflict, получено: %v", err)
	}

	close(ag.block)
	svc.Wait()
}

func TestIterateGameRequiresCode(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo, &fakeAgent{spec: snakeSpec()})

	game, _, _ := svc.CreateGame(context.Background(), "owner-1", "змейка", false)
	if _, err := svc.IterateGame(context.Background(), "owner-1", game.ID, "поменяй цвет"); !errors.Is(err, apperr.ErrNotGenerated) {
		t.Fatalf("правка игры без кода: ожидался ErrNotGenerated, получено %v", err)
	}
}

func TestIterateGameRejectsForeignGame(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo, &fakeAgent{})
	id := repo.seedGame("owner-1", gameHTML("let x;"))

	if _, err := svc.IterateGame(context.Background(), "owner-2", id, "поменяй цвет"); !errors.Is(err, apperr.ErrForbidden) {
		t.Fatalf("чужая игра доступна на правку: %v", err)
	}
}

func TestInsightsHintReachesAgent(t *testing.T) {
	repo := newFakeRepo()
	ag := &fakeAgent{spec: snakeSpec()}
	svc := NewService(context.Background(), repo, ag, staticInsights{hint: "- \"Змейка\": 120 sessions"}, 4)

	if _, _, err := svc.CreateGame(context.Background(), "owner-1", "сделай игру", false); err != nil {
		t.Fatalf("план: %v", err)
	}
	if !strings.Contains(ag.lastPlan.Hint.TopGames, "Змейка") {
		t.Error("подсказка аналитики не дошла до агента")
	}
}

func TestGenerationStatsForManagersOnly(t *testing.T) {
	svc := newTestService(newFakeRepo(), &fakeAgent{})

	if _, err := svc.GenerationStats(context.Background(), "player", 30); !errors.Is(err, apperr.ErrForbidden) {
		t.Errorf("игрок видит статистику генераций: %v", err)
	}
	if st, err := svc.GenerationStats(context.Background(), roleManager, 0); err != nil || st.Days != 30 {
		t.Errorf("менеджер: %+v, %v", st, err)
	}
}

func TestGetGameHidesForeignDraft(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo, &fakeAgent{})

	id, _ := repo.CreateGame(context.Background(), "owner-1", "черновик")

	if _, err := svc.GetGame(context.Background(), "owner-2", id); !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("чужой черновик виден: %v", err)
	}
	if _, err := svc.GetGame(context.Background(), "owner-1", id); err != nil {
		t.Fatalf("владелец не видит свой черновик: %v", err)
	}
}

func TestPublishRequiresGeneratedCode(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo, &fakeAgent{})

	id, _ := repo.CreateGame(context.Background(), "owner-1", "пустая")

	if _, err := svc.PublishGame(context.Background(), "owner-1", id, true); !errors.Is(err, apperr.ErrNotGenerated) {
		t.Fatalf("опубликована игра без кода: %v", err)
	}
}

func TestListQueryBoundsAreEnforcedByService(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo, &fakeAgent{})
	ctx := context.Background()

	tests := []struct {
		name       string
		call       func()
		wantLimit  int
		wantOffset int
	}{
		{
			name:       "свои игры: ноль заменяется значением по умолчанию",
			call:       func() { _, _ = svc.GetMyGames(ctx, "owner-1", 0, 0) },
			wantLimit:  defaultOwnerLimit,
			wantOffset: 0,
		},
		{
			name:       "свои игры: запредельный limit подрезается",
			call:       func() { _, _ = svc.GetMyGames(ctx, "owner-1", 10_000, 5) },
			wantLimit:  maxOwnerLimit,
			wantOffset: 5,
		},
		{
			name:       "витрина: отрицательные значения не доходят до репозитория",
			call:       func() { _, _ = svc.GetGallery(ctx, "", "", -5, -5) },
			wantLimit:  defaultGalleryLimit,
			wantOffset: 0,
		},
		{
			name:       "витрина: запредельный limit подрезается",
			call:       func() { _, _ = svc.GetGallery(ctx, "popular", "змей", 1_000, 7) },
			wantLimit:  maxGalleryLimit,
			wantOffset: 7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.call()
			if repo.lastQuery.Limit != tt.wantLimit {
				t.Errorf("limit = %d, ожидалось %d", repo.lastQuery.Limit, tt.wantLimit)
			}
			if repo.lastQuery.Offset != tt.wantOffset {
				t.Errorf("offset = %d, ожидалось %d", repo.lastQuery.Offset, tt.wantOffset)
			}
		})
	}
}

func TestGalleryNormalizesSort(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo, &fakeAgent{})

	_, _ = svc.GetGallery(context.Background(), "что-то своё", "", 10, 0)
	if repo.lastQuery.Sort != domain.SortNew {
		t.Errorf("sort = %q, ожидалось %q", repo.lastQuery.Sort, domain.SortNew)
	}

	_, _ = svc.GetGallery(context.Background(), domain.SortPopular, "", 10, 0)
	if repo.lastQuery.Sort != domain.SortPopular {
		t.Errorf("sort = %q, ожидалось %q", repo.lastQuery.Sort, domain.SortPopular)
	}
}

func TestRestoreVersionAppendsNewVersion(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo, &fakeAgent{})
	ctx := context.Background()

	id := repo.seedGame("owner-1", gameHTML("let v1;"))
	if _, err := repo.AddVersion(ctx, domain.VersionInput{GameID: id, Reply: "v2", HTML: gameHTML("let v2;")}); err != nil {
		t.Fatalf("подготовка: %v", err)
	}

	restored, _, version, err := svc.RestoreVersion(ctx, "owner-1", id, 1)
	if err != nil {
		t.Fatalf("откат не удался: %v", err)
	}
	if version != 3 {
		t.Errorf("версия после отката = %d, ожидалась 3", version)
	}
	if !strings.Contains(restored.HTML, "let v1;") {
		t.Error("откат вернул не тот код")
	}
}
