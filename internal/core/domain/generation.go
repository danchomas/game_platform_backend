package domain

import "time"

// Статусы игры в агентном конвейере.
const (
	StatusPlanning         = "planning"
	StatusAwaitingApproval = "awaiting_approval"
	StatusGenerating       = "generating"
	StatusValidating       = "validating"
	StatusTesting          = "testing"
	StatusFixing           = "fixing"
	StatusReady            = "ready"
	StatusFailed           = "failed"
)

// Busy — идёт работа агента, новые запросы на генерацию для игры не принимаются.
func Busy(status string) bool {
	switch status {
	case StatusPlanning, StatusGenerating, StatusValidating, StatusTesting, StatusFixing:
		return true
	}
	return false
}

// Виды запусков агента.
const (
	GenPlan   = "plan"
	GenRevise = "revise"
	GenBuild  = "build"
	GenEdit   = "edit"
	// GenReview — фоновая проверка логики готовой игры и, если нужно, её улучшение.
	GenReview = "review"
)

// Итог запуска агента в таблице generations.
const (
	GenStatusReady      = "ready"
	GenStatusWithIssues = "ready_with_issues"
	GenStatusFailed     = "failed"
)

type Severity string

const (
	SevError Severity = "error"
	SevWarn  Severity = "warn"
)

// Issue — проблема из статической проверки или теста в браузере.
// json-теги — формат хранения в generations.issues (JSONB).
type Issue struct {
	Code     string   `json:"code"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
	// Fixed — найдена на одном из кругов и исправлена агентом.
	Fixed bool `json:"fixed,omitempty"`
}

// HasErrors — остались ли блокирующие (неисправленные) проблемы.
func HasErrors(issues []Issue) bool {
	for _, i := range issues {
		if i.Severity == SevError && !i.Fixed {
			return true
		}
	}
	return false
}

type StatusEvent struct {
	GameID  string
	Status  string
	Detail  string
	Version int
	At      time.Time
	// Progress — только пока идёт сборка
	Progress *BuildProgress
}

// BuildProgress — ход фоновой сборки для экрана ожидания. Живёт в памяти процесса.
type BuildProgress struct {
	StartedAt time.Time
	// ExpectedMs — обычная длительность такой сборки по истории
	ExpectedMs int64
	// Role — кто сейчас пишет код: coder или fixer; пусто — идут проверки
	Role string
	// PhaseStartedAt — когда модель начала писать текущий ответ: по нему считается скорость
	PhaseStartedAt time.Time
	Chars          int
	ExpectedChars  int
	// Step — последняя метка «// @step …» из кода, который пишет модель
	Step string
}

// ReviewState — фоновая проверка готовой игры: идёт, пока игрок уже играет и может править.
type ReviewState struct {
	StartedAt time.Time
	// Phase: reviewing — ревьюер читает код; improving — исправление и тест найденного
	Phase string
}

const (
	ReviewPhaseReviewing = "reviewing"
	ReviewPhaseImproving = "improving"
)

// BuildEstimate — обычные длительность сборки и размер кода по прошлым удачным сборкам.
type BuildEstimate struct {
	Samples    int
	DurationMs int64
	CoderChars int
}

type Plan struct {
	Spec  GameSpec
	Reply string
}

// LLMCall — один вызов модели: для подсчёта стоимости и задержек.
type LLMCall struct {
	Role             string
	Model            string
	PromptTokens     int
	CompletionTokens int
	// ReasoningTokens — часть CompletionTokens, ушедшая на скрытые рассуждения модели.
	ReasoningTokens int
	CostUSD         float64
	LatencyMs       int64
	FinishReason    string
	Error           string
	// OutputChars — длина ответа в байтах: по ней оценивается прогресс следующих сборок
	OutputChars int
}

// Generation — один запуск агента (план, сборка или правка) со всей телеметрией.
type Generation struct {
	ID               int64
	GameID           string
	Kind             string
	Status           string
	Version          *int
	PromptTokens     int
	CompletionTokens int
	CostUSD          float64
	DurationMs       int64
	FixRounds        int
	Issues           []Issue
	TestReport       []byte
	Error            string
	Calls            []LLMCall
	HasScreenshot    bool
	CreatedAt        time.Time
	FinishedAt       *time.Time
}

// GenerationResult — то, что сервис записывает по завершении запуска.
type GenerationResult struct {
	ID         int64
	Status     string
	Version    *int
	DurationMs int64
	FixRounds  int
	Issues     []Issue
	TestReport []byte
	Screenshot []byte
	Error      string
	Calls      []LLMCall
}

type ModelStat struct {
	Role         string
	Model        string
	Calls        int64
	CostUSD      float64
	AvgLatencyMs float64
	Truncated    int64
	Errors       int64
}

type GenerationStats struct {
	Days           int
	Total          int64
	Ready          int64
	WithIssues     int64
	Failed         int64
	SuccessRate    float64
	TotalCostUSD   float64
	AvgCostUSD     float64
	AvgDurationSec float64
	AvgFixRounds   float64
	TopIssues      []Bucket
	ByModel        []ModelStat
}
