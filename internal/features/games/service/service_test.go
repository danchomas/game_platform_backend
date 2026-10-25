package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"beeline-games/internal/core/apperr"
	"beeline-games/internal/core/domain"
)

func newTestService(repo GameRepository, model LLMProvider) *Service {
	return NewService(repo, model, staticInsights{}, 4, testTimeout)
}

func TestCreateAndGenerateSavesVersion(t *testing.T) {
	repo := newFakeRepo()
	model := &scriptedLLM{replies: []string{goodReply("Змейка", "let snake;")}}
	svc := newTestService(repo, model)

	game, reply, version, err := svc.CreateAndGenerate(context.Background(), "owner-1", "сделай змейку")
	if err != nil {
		t.Fatalf("генерация не удалась: %v", err)
	}
	if version != 1 {
		t.Errorf("версия = %d, ожидалась 1", version)
	}
	if game.Title != "Змейка" {
		t.Errorf("название = %q, ожидалось %q", game.Title, "Змейка")
	}
	if reply == "" {
		t.Error("реплика модели пуста")
	}
	if !game.Generated() {
		t.Error("игра помечена как несгенерированная")
	}
}

func TestCreateAndGenerateRejectsShortPrompt(t *testing.T) {
	svc := newTestService(newFakeRepo(), &scriptedLLM{})

	_, _, _, err := svc.CreateAndGenerate(context.Background(), "owner-1", " a ")
	if !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("ожидалась ошибка валидации, получено: %v", err)
	}

	var vErr *apperr.ValidationError
	if !errors.As(err, &vErr) || vErr.Field != "prompt" {
		t.Errorf("ошибка не указывает поле prompt: %v", err)
	}
}

func TestCreateAndGenerateDropsGameWhenModelFails(t *testing.T) {
	repo := newFakeRepo()
	model := &scriptedLLM{replies: []string{"я не умею делать игры", "и сейчас тоже"}}
	svc := newTestService(repo, model)

	_, _, _, err := svc.CreateAndGenerate(context.Background(), "owner-1", "сделай змейку")
	if !errors.Is(err, apperr.ErrLLMFailed) {
		t.Fatalf("ожидалась ErrLLMFailed, получено: %v", err)
	}
	if len(repo.deleted) != 1 {
		t.Fatalf("карточка не удалена, удалено записей: %d", len(repo.deleted))
	}
	if len(repo.games) != 0 {
		t.Errorf("в каталоге осталось %d игр", len(repo.games))
	}
}

func TestGenerateRetriesUnparsableReply(t *testing.T) {
	repo := newFakeRepo()
	model := &scriptedLLM{replies: []string{"ответ без кода", goodReply("Тетрис", "let board;")}}
	svc := newTestService(repo, model)

	_, _, version, err := svc.CreateAndGenerate(context.Background(), "owner-1", "сделай тетрис")
	if err != nil {
		t.Fatalf("повторная попытка не сработала: %v", err)
	}
	if version != 1 {
		t.Errorf("версия = %d, ожидалась 1", version)
	}
	if model.calls != 2 {
		t.Errorf("модель вызвана %d раз, ожидалось 2", model.calls)
	}
}

func TestGenerateRejectsHTMLWithExternalResource(t *testing.T) {
	repo := newFakeRepo()
	bad := strings.Replace(goodReply("Игра", "let x;"), "<canvas></canvas>",
		`<script src="https://cdn.example.com/engine.js"></script>`, 1)
	model := &scriptedLLM{replies: []string{bad, bad}}
	svc := newTestService(repo, model)

	_, _, _, err := svc.CreateAndGenerate(context.Background(), "owner-1", "сделай игру")
	if !errors.Is(err, apperr.ErrLLMFailed) {
		t.Fatalf("внешний ресурс не отклонён: %v", err)
	}
}

func TestIterateGameRejectsConcurrentGeneration(t *testing.T) {
	repo := newFakeRepo()
	model := &blockingLLM{
		entered: make(chan struct{}),
		release: make(chan struct{}),
		reply:   goodReply("Игра", "let x;"),
	}
	svc := newTestService(repo, model)

	id, err := repo.CreateGame(context.Background(), "owner-1", "первая")
	if err != nil {
		t.Fatalf("подготовка не удалась: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _, _, _ = svc.IterateGame(context.Background(), "owner-1", id, "сделай кнопки больше")
	}()

	<-model.entered

	_, _, _, err = svc.IterateGame(context.Background(), "owner-1", id, "и ещё фон поменяй")
	if !errors.Is(err, apperr.ErrConflict) {
		t.Errorf("ожидался ErrConflict, получено: %v", err)
	}

	close(model.release)
	wg.Wait()
}

func TestIterateGameRejectsForeignGame(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo, &scriptedLLM{})

	id, _ := repo.CreateGame(context.Background(), "owner-1", "первая")

	_, _, _, err := svc.IterateGame(context.Background(), "owner-2", id, "поменяй цвет")
	if !errors.Is(err, apperr.ErrForbidden) {
		t.Fatalf("чужая игра доступна на правку: %v", err)
	}
}

func TestGetGameHidesForeignDraft(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo, &scriptedLLM{})

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
	svc := newTestService(repo, &scriptedLLM{})

	id, _ := repo.CreateGame(context.Background(), "owner-1", "пустая")

	_, err := svc.PublishGame(context.Background(), "owner-1", id, true)
	if !errors.Is(err, apperr.ErrNotGenerated) {
		t.Fatalf("опубликована игра без кода: %v", err)
	}
}

func TestListQueryBoundsAreEnforcedByService(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo, &scriptedLLM{})
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
	svc := newTestService(repo, &scriptedLLM{})

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
	model := &scriptedLLM{replies: []string{
		goodReply("Игра", "let v1;"),
		goodReply("Игра", "let v2;"),
	}}
	svc := newTestService(repo, model)
	ctx := context.Background()

	game, _, _, err := svc.CreateAndGenerate(ctx, "owner-1", "сделай игру")
	if err != nil {
		t.Fatalf("первая генерация: %v", err)
	}
	if _, _, _, err := svc.IterateGame(ctx, "owner-1", game.ID, "поменяй цвет"); err != nil {
		t.Fatalf("вторая генерация: %v", err)
	}

	restored, _, version, err := svc.RestoreVersion(ctx, "owner-1", game.ID, 1)
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

func TestInsightsHintReachesPrompt(t *testing.T) {
	repo := newFakeRepo()
	model := &scriptedLLM{replies: []string{goodReply("Игра", "let x;")}}
	svc := NewService(repo, model, staticInsights{hint: "- \"Змейка\": 120 sessions"}, 4, testTimeout)

	if _, _, _, err := svc.CreateAndGenerate(context.Background(), "owner-1", "сделай игру"); err != nil {
		t.Fatalf("генерация не удалась: %v", err)
	}

	var joined strings.Builder
	for _, m := range model.lastMsgs {
		joined.WriteString(m.Content)
	}
	if !strings.Contains(joined.String(), "Змейка") {
		t.Error("подсказка аналитики не дошла до промпта")
	}
}

func TestGenerateWithoutInsightsProvider(t *testing.T) {
	repo := newFakeRepo()
	model := &scriptedLLM{replies: []string{goodReply("Игра", "let x;")}}
	svc := NewService(repo, model, nil, 4, testTimeout)

	if _, _, _, err := svc.CreateAndGenerate(context.Background(), "owner-1", "сделай игру"); err != nil {
		t.Fatalf("генерация без подсказчика не удалась: %v", err)
	}
}

func TestGenerateReportsCapacityWhenSlotsBusy(t *testing.T) {
	repo := newFakeRepo()
	model := &blockingLLM{
		entered: make(chan struct{}),
		release: make(chan struct{}),
		reply:   goodReply("Игра", "let x;"),
	}
	svc := NewService(repo, model, nil, 1, testTimeout)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _, _, _ = svc.CreateAndGenerate(context.Background(), "owner-1", "первая игра")
	}()

	<-model.entered

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, _, _, err := svc.CreateAndGenerate(ctx, "owner-2", "вторая игра")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("ожидалось истечение ожидания слота, получено: %v", err)
	}

	close(model.release)
	wg.Wait()
}
