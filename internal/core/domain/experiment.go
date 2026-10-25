package domain

import (
	"math"
	"time"
)

// Статусы эксперимента: варианты собираются → ссылка раздаёт их игрокам → выбран победитель.
const (
	ExperimentDraft    = "draft"
	ExperimentRunning  = "running"
	ExperimentFinished = "finished"
)

// RetainedSeconds — сколько секунд активной игры делают показ варианта успешным.
// Метрику считает SDK одинаково для всех вариантов, в отличие от событий,
// которые шлёт код игры, написанный моделью.
const RetainedSeconds = 30

type Experiment struct {
	ID      string
	OwnerID string
	Title   string
	Prompt  string
	Status  string
	// WinnerID — пусто, пока победитель не выбран.
	WinnerID   string
	CreatedAt  time.Time
	LaunchedAt *time.Time
	FinishedAt *time.Time
	// Variants и Ready — сводка по вариантам: всего и собранных.
	Variants int
	Ready    int
}

// VariantInput — заявка на один вариант мультигенерации.
type VariantInput struct {
	OwnerID      string
	ExperimentID string
	Label        string
	// Direction — в чём этот вариант должен отличаться от остальных.
	Direction string
	Prompt    string
}

// Variant — игра-вариант эксперимента с итогом последней сборки.
type Variant struct {
	GameID       string
	Label        string
	Direction    string
	Title        string
	Genre        string
	Status       string
	StatusDetail string
	Version      int
	Published    bool
	// BuildStatus — итог последней удачной сборки или правки (GenStatus*), пусто — кода нет.
	BuildStatus string
	CostUSD     float64
	FixRounds   int
}

// Eligible — вариант собран и прошёл автопроверки: его можно показывать игрокам.
func (v Variant) Eligible() bool {
	return v.Version > 0 && v.Status == StatusReady && v.BuildStatus == GenStatusReady
}

// RejectReason объясняет, почему вариант не попадёт в эксперимент; пусто — попадёт.
func (v Variant) RejectReason() string {
	switch {
	case v.Eligible():
		return ""
	case Busy(v.Status):
		return "ещё собирается"
	case v.Version == 0:
		return "сборка не удалась"
	case v.BuildStatus == GenStatusWithIssues:
		return "автопроверка нашла ошибки, которые не удалось исправить"
	default:
		return "вариант не готов"
	}
}

// ExperimentRouting — то, что нужно ссылке эксперимента, чтобы выбрать вариант для игрока.
type ExperimentRouting struct {
	Status   string
	WinnerID string
	// Live — опубликованные варианты в порядке меток.
	Live []string
}

// VariantStats — воронка варианта: показ → загрузка → вовлечение → удержание.
// Учитываются только сессии, пришедшие по ссылке эксперимента.
type VariantStats struct {
	GameID        string
	Exposures     int64
	Sessions      int64
	Engaged       int64
	Retained      int64
	ErrorSessions int64
	Players       int64
	Starts        int64
	Ends          int64
	Wins          int64
	AvgSessionSec float64

	LoadRate       float64
	EngagementRate float64
	// RetentionRate — доля показов, доигранных до RetainedSeconds: главная метрика сравнения.
	RetentionRate float64
	// RetentionLow, RetentionHigh — 95% доверительный интервал Уилсона для RetentionRate.
	RetentionLow   float64
	RetentionHigh  float64
	ErrorRate      float64
	CompletionRate float64
	WinRate        float64
}

// ExperimentReport — дашборд эксперимента: варианты, их воронки и текущий лидер.
type ExperimentReport struct {
	Experiment Experiment
	Variants   []Variant
	Stats      []VariantStats
	// LeaderID — вариант с лучшей нижней границей RetentionRate, пусто — показов ещё не было.
	LeaderID string
	// Significant — интервал лидера не пересекается с интервалами остальных вариантов.
	Significant bool
}

func (s *VariantStats) Derive() {
	s.LoadRate = ratio(s.Sessions, s.Exposures)
	s.EngagementRate = ratio(s.Engaged, s.Sessions)
	s.RetentionRate = ratio(s.Retained, s.Exposures)
	s.RetentionLow, s.RetentionHigh = Wilson(s.Retained, s.Exposures)
	s.ErrorRate = ratio(s.ErrorSessions, s.Sessions)
	s.CompletionRate = ratio(s.Ends, s.Starts)
	s.WinRate = ratio(s.Wins, s.Ends)
}

func ratio(a, b int64) float64 {
	if b <= 0 {
		return 0
	}
	return float64(a) / float64(b)
}

// Wilson — 95% доверительный интервал доли успехов. В отличие от «p ± 2σ» он не выходит
// за [0, 1] и честно широк на малых выборках — а у хакатонного трафика других не бывает.
func Wilson(successes, trials int64) (low, high float64) {
	if trials <= 0 {
		return 0, 1
	}
	const z = 1.96
	n := float64(trials)
	p := float64(successes) / n
	denom := 1 + z*z/n
	center := (p + z*z/(2*n)) / denom
	half := z * math.Sqrt(p*(1-p)/n+z*z/(4*n*n)) / denom
	return max(0, center-half), min(1, center+half)
}
