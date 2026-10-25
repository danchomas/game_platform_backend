// Package service — мультигенерация: несколько вариантов игры по одному запросу,
// общая ссылка, которая раздаёт их игрокам, и сравнение вариантов по поведению игроков.
package service

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"beeline-games/internal/core/apperr"
	"beeline-games/internal/core/domain"
)

const (
	minVariants     = 2
	maxVariants     = 4
	defaultVariants = 3

	minPromptLen = 3
	maxPromptLen = 4000
	maxTitleLen  = 80

	listLimit = 50
)

type direction struct {
	label string
	text  string
}

// directions — чем варианты одного запроса отличаются друг от друга. Они заданы заранее,
// а не придумываются моделью: так варианты гарантированно расходятся, и лишнего вызова нет.
var directions = [maxVariants]direction{
	{"A", "Классика: самая очевидная механика для этого запроса, минимум правил, понятно с первой секунды."},
	{"B", "Драйв: короткие раунды, нарастающий темп, серии и комбо за точные действия."},
	{"C", "Головоломка: спокойный темп, ставка на смекалку, а не на реакцию; уровни постепенно усложняются."},
	{"D", "Неожиданный поворот: другой основной жест управления и необычная механика на ту же тему."},
}

type Service struct {
	repo  ExperimentRepository
	games VariantGames
}

func NewService(repo ExperimentRepository, games VariantGames) *Service {
	return &Service{repo: repo, games: games}
}

// Create заводит эксперимент и запускает сборку вариантов в фоне; ход сборки виден в Get.
func (s *Service) Create(ctx context.Context, ownerID, prompt string, variants int) (domain.Experiment, []domain.Variant, error) {
	prompt = strings.TrimSpace(prompt)
	if n := utf8.RuneCountInString(prompt); n < minPromptLen || n > maxPromptLen {
		return domain.Experiment{}, nil, apperr.NewValidationError("prompt",
			fmt.Sprintf("текст: от %d до %d символов", minPromptLen, maxPromptLen))
	}
	if variants == 0 {
		variants = defaultVariants
	}
	if variants < minVariants || variants > maxVariants {
		return domain.Experiment{}, nil, apperr.NewValidationError("variants",
			fmt.Sprintf("вариантов: от %d до %d", minVariants, maxVariants))
	}

	id, err := s.repo.Create(ctx, ownerID, titleOf(prompt), prompt)
	if err != nil {
		return domain.Experiment{}, nil, err
	}
	for _, d := range directions[:variants] {
		in := domain.VariantInput{OwnerID: ownerID, ExperimentID: id, Label: d.label, Direction: d.text, Prompt: prompt}
		if _, err := s.games.StartVariant(ctx, in); err != nil {
			return domain.Experiment{}, nil, fmt.Errorf("запуск варианта %s: %w", d.label, err)
		}
	}
	return s.load(ctx, id)
}

func (s *Service) Get(ctx context.Context, userID, id string) (domain.Experiment, []domain.Variant, error) {
	if _, err := s.owned(ctx, userID, id); err != nil {
		return domain.Experiment{}, nil, err
	}
	return s.load(ctx, id)
}

func (s *Service) List(ctx context.Context, userID string) ([]domain.Experiment, error) {
	return s.repo.ListOwner(ctx, userID, listLimit)
}

// Launch публикует варианты, прошедшие автопроверки, и открывает ссылку эксперимента.
// Сломанные варианты до игроков не доходят: отсев происходит здесь, а не по статистике.
func (s *Service) Launch(ctx context.Context, userID, id string) (domain.Experiment, []domain.Variant, error) {
	x, err := s.owned(ctx, userID, id)
	if err != nil {
		return domain.Experiment{}, nil, err
	}
	if x.Status != domain.ExperimentDraft {
		return domain.Experiment{}, nil, apperr.NewValidationError("status", "эксперимент уже запущен")
	}

	variants, err := s.repo.Variants(ctx, id)
	if err != nil {
		return domain.Experiment{}, nil, err
	}
	var eligible []domain.Variant
	for _, v := range variants {
		if v.Eligible() {
			eligible = append(eligible, v)
		}
	}
	if len(eligible) < minVariants {
		return domain.Experiment{}, nil, apperr.NewValidationError("variants",
			fmt.Sprintf("для сравнения нужно хотя бы %d рабочих варианта, готово %d", minVariants, len(eligible)))
	}

	for _, v := range eligible {
		if v.Published {
			continue
		}
		if _, err := s.games.PublishGame(ctx, userID, v.GameID, true); err != nil {
			return domain.Experiment{}, nil, fmt.Errorf("публикация варианта %s: %w", v.Label, err)
		}
	}
	ok, err := s.repo.Launch(ctx, id)
	if err != nil {
		return domain.Experiment{}, nil, err
	}
	if !ok {
		return domain.Experiment{}, nil, apperr.NewValidationError("status", "эксперимент уже запущен")
	}
	return s.load(ctx, id)
}

// ChooseWinner закрывает эксперимент: ссылка начинает отдавать всем один вариант,
// и он же появляется на витрине.
func (s *Service) ChooseWinner(ctx context.Context, userID, id, gameID string) (domain.Experiment, []domain.Variant, error) {
	x, err := s.owned(ctx, userID, id)
	if err != nil {
		return domain.Experiment{}, nil, err
	}
	if x.Status == domain.ExperimentDraft {
		return domain.Experiment{}, nil, apperr.NewValidationError("status", "эксперимент ещё не запущен")
	}

	variants, err := s.repo.Variants(ctx, id)
	if err != nil {
		return domain.Experiment{}, nil, err
	}
	live := false
	for _, v := range variants {
		if v.GameID == gameID && v.Published && v.Version > 0 {
			live = true
			break
		}
	}
	if !live {
		return domain.Experiment{}, nil, apperr.NewValidationError("game_id",
			"победителем может быть только вариант, который участвует в эксперименте")
	}

	ok, err := s.repo.Finish(ctx, id, gameID)
	if err != nil {
		return domain.Experiment{}, nil, err
	}
	if !ok {
		return domain.Experiment{}, nil, apperr.NewValidationError("status", "эксперимент ещё не запущен")
	}
	return s.load(ctx, id)
}

func (s *Service) owned(ctx context.Context, userID, id string) (domain.Experiment, error) {
	if uuid.Validate(id) != nil {
		return domain.Experiment{}, apperr.ErrNotFound
	}
	x, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.Experiment{}, err
	}
	if x.OwnerID != userID {
		return domain.Experiment{}, apperr.ErrForbidden
	}
	return x, nil
}

func (s *Service) load(ctx context.Context, id string) (domain.Experiment, []domain.Variant, error) {
	x, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.Experiment{}, nil, err
	}
	variants, err := s.repo.Variants(ctx, id)
	if err != nil {
		return domain.Experiment{}, nil, err
	}
	return x, variants, nil
}

func titleOf(prompt string) string {
	prompt = strings.Join(strings.Fields(prompt), " ")
	if r := []rune(prompt); len(r) > maxTitleLen {
		return string(r[:maxTitleLen-1]) + "…"
	}
	return prompt
}
