package domain

import (
	"encoding/json"
	"time"
)

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
)

type Severity string

const (
	SevError Severity = "error"
	SevWarn  Severity = "warn"
)

// Issue — проблема, найденная статической проверкой или тестом в браузере.
type Issue struct {
	Code     string   `json:"code"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
	// Fixed — проблема была найдена на одном из кругов и исправлена агентом
	Fixed bool `json:"fixed,omitempty"`
}

func HasErrors(issues []Issue) bool {
	for _, i := range issues {
		if i.Severity == SevError && !i.Fixed {
			return true
		}
	}
	return false
}

type StatusEvent struct {
	GameID  string    `json:"game_id"`
	Status  string    `json:"status"`
	Detail  string    `json:"detail,omitempty"`
	Version int       `json:"version,omitempty"`
	At      time.Time `json:"at"`
}

type Plan struct {
	Spec  GameSpec `json:"spec"`
	Reply string   `json:"reply"`
}

// LLMCall — один вызов модели, для подсчёта стоимости и задержек.
type LLMCall struct {
	Role             string  `json:"role"`
	Model            string  `json:"model"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	CostUSD          float64 `json:"cost_usd"`
	LatencyMs        int64   `json:"latency_ms"`
	FinishReason     string  `json:"finish_reason"`
	Error            string  `json:"error,omitempty"`
}

// Generation — один запуск агента (план, сборка или правка) со всей телеметрией.
type Generation struct {
	ID               int64           `json:"id"`
	GameID           string          `json:"game_id"`
	Kind             string          `json:"kind"`
	Status           string          `json:"status"`
	Version          *int            `json:"version,omitempty"`
	PromptTokens     int             `json:"prompt_tokens"`
	CompletionTokens int             `json:"completion_tokens"`
	CostUSD          float64         `json:"cost_usd"`
	DurationMs       int64           `json:"duration_ms"`
	FixRounds        int             `json:"fix_rounds"`
	Issues           []Issue         `json:"issues"`
	Test             json.RawMessage `json:"test,omitempty"`
	Error            string          `json:"error,omitempty"`
	Calls            []LLMCall       `json:"calls,omitempty"`
	HasScreenshot    bool            `json:"has_screenshot"`
	CreatedAt        time.Time       `json:"created_at"`
	FinishedAt       *time.Time      `json:"finished_at,omitempty"`
}

// GenerationResult — то, что сервис записывает по завершении запуска.
type GenerationResult struct {
	ID         int64
	Status     string
	Version    *int
	DurationMs int64
	FixRounds  int
	Issues     []Issue
	Test       json.RawMessage
	Screenshot []byte
	Error      string
	Calls      []LLMCall
}

type ModelStat struct {
	Role         string  `json:"role"`
	Model        string  `json:"model"`
	Calls        int64   `json:"calls"`
	CostUSD      float64 `json:"cost_usd"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
	Truncated    int64   `json:"truncated"`
	Errors       int64   `json:"errors"`
}

type GenerationStats struct {
	Days           int         `json:"days"`
	Total          int64       `json:"total"`
	Ready          int64       `json:"ready"`
	WithIssues     int64       `json:"ready_with_issues"`
	Failed         int64       `json:"failed"`
	SuccessRate    float64     `json:"success_rate"`
	TotalCostUSD   float64     `json:"total_cost_usd"`
	AvgCostUSD     float64     `json:"avg_cost_usd"`
	AvgDurationSec float64     `json:"avg_duration_sec"`
	AvgFixRounds   float64     `json:"avg_fix_rounds"`
	TopIssues      []Bucket    `json:"top_issues"`
	ByModel        []ModelStat `json:"by_model"`
}
