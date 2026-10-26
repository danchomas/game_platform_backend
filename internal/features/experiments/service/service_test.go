package service

import (
	"context"
	"errors"
	"slices"
	"testing"

	"beeline-games/internal/core/apperr"
	"beeline-games/internal/core/domain"
)

func TestCreateStartsVariantsWithDistinctDirections(t *testing.T) {
	repo, games := newFakeRepo(), &fakeGames{}
	svc := NewService(repo, games)

	x, _, err := svc.Create(context.Background(), ownerID, "  змейка   для бабушки ", 3)
	if err != nil {
		t.Fatalf("эксперимент не создан: %v", err)
	}
	if x.Title != "змейка для бабушки" || x.Prompt != "змейка   для бабушки" {
		t.Errorf("название или запрос не нормализованы: %+v", x)
	}
	if len(games.started) != 3 {
		t.Fatalf("запущено вариантов %d, ожидалось 3", len(games.started))
	}

	seen := map[string]bool{}
	for i, in := range games.started {
		if want := string(rune('A' + i)); in.Label != want {
			t.Errorf("вариант %d: метка %q, ожидалась %q", i, in.Label, want)
		}
		if in.ExperimentID != expID || in.OwnerID != ownerID || in.Prompt != x.Prompt {
			t.Errorf("вариант %s не привязан к эксперименту: %+v", in.Label, in)
		}
		if in.Direction == "" || seen[in.Direction] {
			t.Errorf("вариант %s: направление пустое или повторяется: %q", in.Label, in.Direction)
		}
		seen[in.Direction] = true
	}
}

func TestCreateValidatesInput(t *testing.T) {
	tests := []struct {
		name     string
		prompt   string
		variants int
		field    string
	}{
		{name: "short prompt", prompt: " a ", variants: 3, field: "prompt"},
		{name: "one variant", prompt: "змейка", variants: 1, field: "variants"},
		{name: "too many variants", prompt: "змейка", variants: 5, field: "variants"},
		{name: "negative variants", prompt: "змейка", variants: -2, field: "variants"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			games := &fakeGames{}
			_, _, err := NewService(newFakeRepo(), games).Create(context.Background(), ownerID, tt.prompt, tt.variants)

			var vErr *apperr.ValidationError
			if !errors.As(err, &vErr) || vErr.Field != tt.field {
				t.Fatalf("ожидалась ошибка валидации поля %s, получено: %v", tt.field, err)
			}
			if len(games.started) != 0 {
				t.Errorf("при ошибке валидации варианты запускаться не должны")
			}
		})
	}
}

func TestCreateDefaultsToThreeVariants(t *testing.T) {
	games := &fakeGames{}
	if _, _, err := NewService(newFakeRepo(), games).Create(context.Background(), ownerID, "змейка", 0); err != nil {
		t.Fatal(err)
	}
	if len(games.started) != defaultVariants {
		t.Errorf("запущено %d вариантов, по умолчанию ожидалось %d", len(games.started), defaultVariants)
	}
}

func TestLaunchPublishesOnlyEligibleVariants(t *testing.T) {
	repo, games := newFakeRepo(), &fakeGames{}
	broken := ready("C")
	broken.BuildStatus = domain.GenStatusWithIssues
	building := domain.Variant{GameID: "game-D", Label: "D", Status: domain.StatusGenerating}
	already := ready("B")
	already.Published = true
	repo.seed(domain.ExperimentDraft, ready("A"), already, broken, building)

	x, _, err := NewService(repo, games).Launch(context.Background(), ownerID, expID)
	if err != nil {
		t.Fatalf("эксперимент не запущен: %v", err)
	}
	if x.Status != domain.ExperimentRunning {
		t.Errorf("статус = %q, ожидался %q", x.Status, domain.ExperimentRunning)
	}
	if !slices.Equal(games.published, []string{"game-A"}) {
		t.Errorf("опубликованы %v, ожидался только game-A (B уже опубликован, C и D отсеяны)", games.published)
	}
}

func TestLaunchNeedsTwoWorkingVariants(t *testing.T) {
	repo, games := newFakeRepo(), &fakeGames{}
	broken := ready("B")
	broken.Version, broken.Status, broken.BuildStatus = 0, domain.StatusFailed, ""
	repo.seed(domain.ExperimentDraft, ready("A"), broken)

	_, _, err := NewService(repo, games).Launch(context.Background(), ownerID, expID)

	var vErr *apperr.ValidationError
	if !errors.As(err, &vErr) || vErr.Field != "variants" {
		t.Fatalf("ожидалась ошибка валидации поля variants, получено: %v", err)
	}
	if len(games.published) != 0 || repo.experiment().Status != domain.ExperimentDraft {
		t.Error("эксперимент с одним рабочим вариантом запускаться не должен")
	}
}

func TestLaunchRejectsRunningExperiment(t *testing.T) {
	repo := newFakeRepo()
	repo.seed(domain.ExperimentRunning, ready("A"), ready("B"))

	_, _, err := NewService(repo, &fakeGames{}).Launch(context.Background(), ownerID, expID)

	var vErr *apperr.ValidationError
	if !errors.As(err, &vErr) || vErr.Field != "status" {
		t.Fatalf("ожидалась ошибка валидации поля status, получено: %v", err)
	}
}

func TestExperimentAccess(t *testing.T) {
	tests := []struct {
		name   string
		userID string
		id     string
		want   error
	}{
		{name: "foreign experiment", userID: "owner-2", id: expID, want: apperr.ErrForbidden},
		{name: "malformed id", userID: ownerID, id: "not-a-uuid", want: apperr.ErrNotFound},
		{name: "missing experiment", userID: ownerID, id: "00000000-0000-0000-0000-000000000000", want: apperr.ErrNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakeRepo()
			repo.seed(domain.ExperimentDraft, ready("A"), ready("B"))
			svc := NewService(repo, &fakeGames{})

			if _, _, err := svc.Get(context.Background(), tt.userID, tt.id); !errors.Is(err, tt.want) {
				t.Errorf("Get: получено %v, ожидалось %v", err, tt.want)
			}
			if _, _, err := svc.Launch(context.Background(), tt.userID, tt.id); !errors.Is(err, tt.want) {
				t.Errorf("Launch: получено %v, ожидалось %v", err, tt.want)
			}
			if _, err := svc.Report(context.Background(), tt.userID, tt.id); !errors.Is(err, tt.want) {
				t.Errorf("Report: получено %v, ожидалось %v", err, tt.want)
			}
		})
	}
}

func TestChooseWinner(t *testing.T) {
	published := live("A")
	hidden := ready("B")

	tests := []struct {
		name   string
		status string
		gameID string
		field  string
	}{
		{name: "draft experiment", status: domain.ExperimentDraft, gameID: published.GameID, field: "status"},
		{name: "unpublished variant", status: domain.ExperimentRunning, gameID: hidden.GameID, field: "game_id"},
		{name: "foreign game", status: domain.ExperimentRunning, gameID: "game-x", field: "game_id"},
		{name: "live variant", status: domain.ExperimentRunning, gameID: published.GameID},
		{name: "override auto winner", status: domain.ExperimentFinished, gameID: published.GameID},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakeRepo()
			repo.seed(tt.status, published, hidden)

			x, _, err := NewService(repo, &fakeGames{}).ChooseWinner(context.Background(), ownerID, expID, tt.gameID)

			if tt.field != "" {
				var vErr *apperr.ValidationError
				if !errors.As(err, &vErr) || vErr.Field != tt.field {
					t.Fatalf("ожидалась ошибка валидации поля %s, получено: %v", tt.field, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("победитель не выбран: %v", err)
			}
			if x.Status != domain.ExperimentFinished || x.WinnerID != tt.gameID || x.WinnerSource != domain.WinnerManual {
				t.Errorf("эксперимент не завершён победителем: %+v", x)
			}
		})
	}
}
