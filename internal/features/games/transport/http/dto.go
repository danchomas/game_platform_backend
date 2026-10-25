package http

import (
	"encoding/json"
	"time"

	"beeline-games/internal/core/domain"
)

type promptRequest struct {
	Prompt string `json:"prompt"`
	// AutoApprove — сразу собрать игру по плану, без шага подтверждения.
	AutoApprove bool `json:"auto_approve"`
}

type approveRequest struct {
	// Spec — план, отредактированный в клиенте; пусто — собрать по сохранённому.
	Spec *domain.GameSpec `json:"spec"`
}

type messageRequest struct {
	Content string `json:"content"`
}

type patchRequest struct {
	Title string `json:"title"`
}

type gameDTO struct {
	ID        string `json:"id"`
	OwnerID   string `json:"owner_id"`
	OwnerName string `json:"owner_name"`
	Title     string `json:"title"`
	Genre     string `json:"genre"`
	Audience  string `json:"audience"`
	Prompt    string `json:"prompt"`
	HTML      string `json:"html,omitempty"`
	Version   int    `json:"version"`
	Published bool   `json:"published"`
	Plays     int64  `json:"plays"`
	PlayURL   string `json:"play_url"`
	// Status — шаг агентного конвейера: planning, awaiting_approval, generating, validating,
	// testing, fixing, ready, failed.
	Status       string          `json:"status"`
	StatusDetail string          `json:"status_detail,omitempty"`
	Spec         json.RawMessage `json:"spec,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

type messageDTO struct {
	ID        int64     `json:"id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Version   *int      `json:"version,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type versionDTO struct {
	Version     int       `json:"version"`
	Instruction string    `json:"instruction"`
	CreatedAt   time.Time `json:"created_at"`
}

type generationDTO struct {
	Game    gameDTO `json:"game"`
	Reply   string  `json:"reply"`
	Version int     `json:"version"`
}

func (h *Handler) toGameDTO(g domain.Game) gameDTO {
	return gameDTO{
		ID:           g.ID,
		OwnerID:      g.OwnerID,
		OwnerName:    g.OwnerName,
		Title:        g.Title,
		Genre:        g.Genre,
		Audience:     g.Audience,
		Prompt:       g.Prompt,
		HTML:         g.HTML,
		Version:      g.Version,
		Published:    g.Published,
		Plays:        g.Plays,
		PlayURL:      h.publicBaseURL + "/g/" + g.ID,
		Status:       g.Status,
		StatusDetail: g.StatusDetail,
		Spec:         json.RawMessage(g.Spec),
		CreatedAt:    g.CreatedAt,
		UpdatedAt:    g.UpdatedAt,
	}
}

func (h *Handler) toGameDTOs(gs []domain.Game) []gameDTO {
	out := make([]gameDTO, 0, len(gs))
	for _, g := range gs {
		out = append(out, h.toGameDTO(g))
	}
	return out
}

func (h *Handler) toGenerationDTO(g domain.Game, reply string, version int) generationDTO {
	return generationDTO{Game: h.toGameDTO(g), Reply: reply, Version: version}
}

func toMessageDTOs(msgs []domain.Message) []messageDTO {
	out := make([]messageDTO, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, messageDTO{
			ID:        m.ID,
			Role:      m.Role,
			Content:   m.Content,
			Version:   m.Version,
			CreatedAt: m.CreatedAt,
		})
	}
	return out
}

func toVersionDTOs(vs []domain.VersionInfo) []versionDTO {
	out := make([]versionDTO, 0, len(vs))
	for _, v := range vs {
		out = append(out, versionDTO{
			Version:     v.Version,
			Instruction: v.Instruction,
			CreatedAt:   v.CreatedAt,
		})
	}
	return out
}

type planDTO struct {
	Game  gameDTO         `json:"game"`
	Plan  domain.GameSpec `json:"plan"`
	Reply string          `json:"reply"`
}

type acceptedDTO struct {
	Game gameDTO `json:"game"`
}

type statusDTO struct {
	GameID   string       `json:"game_id"`
	Status   string       `json:"status"`
	Detail   string       `json:"detail,omitempty"`
	Version  int          `json:"version,omitempty"`
	At       time.Time    `json:"at"`
	Progress *progressDTO `json:"progress,omitempty"`
}

// progressDTO — ход сборки. Длительности считаются на сервере, чтобы не зависеть от часов клиента.
type progressDTO struct {
	ElapsedMs  int64 `json:"elapsed_ms"`
	ExpectedMs int64 `json:"expected_ms"`
	// Role: coder — пишется код, fixer — исправляется; пусто — проверки без модели
	Role          string `json:"role,omitempty"`
	PhaseMs       int64  `json:"phase_ms,omitempty"`
	Chars         int    `json:"chars"`
	ExpectedChars int    `json:"expected_chars,omitempty"`
	Step          string `json:"step,omitempty"`
}

// statusWithGameDTO — ответ /status: после фоновой сборки клиенту нужна новая карточка игры.
type statusWithGameDTO struct {
	statusDTO
	Game gameDTO `json:"game"`
}

type llmCallDTO struct {
	Role             string  `json:"role"`
	Model            string  `json:"model"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	ReasoningTokens  int     `json:"reasoning_tokens"`
	CostUSD          float64 `json:"cost_usd"`
	LatencyMs        int64   `json:"latency_ms"`
	FinishReason     string  `json:"finish_reason"`
	Error            string  `json:"error,omitempty"`
	OutputChars      int     `json:"output_chars"`
}

type generationItemDTO struct {
	ID               int64           `json:"id"`
	Kind             string          `json:"kind"`
	Status           string          `json:"status"`
	Version          *int            `json:"version,omitempty"`
	PromptTokens     int             `json:"prompt_tokens"`
	CompletionTokens int             `json:"completion_tokens"`
	CostUSD          float64         `json:"cost_usd"`
	DurationMs       int64           `json:"duration_ms"`
	FixRounds        int             `json:"fix_rounds"`
	Issues           []domain.Issue  `json:"issues"`
	TestReport       json.RawMessage `json:"test_report,omitempty"`
	Error            string          `json:"error,omitempty"`
	Calls            []llmCallDTO    `json:"calls"`
	HasScreenshot    bool            `json:"has_screenshot"`
	CreatedAt        time.Time       `json:"created_at"`
	FinishedAt       *time.Time      `json:"finished_at,omitempty"`
}

type bucketDTO struct {
	Label string `json:"label"`
	Count int64  `json:"count"`
}

type modelStatDTO struct {
	Role         string  `json:"role"`
	Model        string  `json:"model"`
	Calls        int64   `json:"calls"`
	CostUSD      float64 `json:"cost_usd"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
	Truncated    int64   `json:"truncated"`
	Errors       int64   `json:"errors"`
}

type generationStatsDTO struct {
	Days           int            `json:"days"`
	Total          int64          `json:"total"`
	Ready          int64          `json:"ready"`
	WithIssues     int64          `json:"ready_with_issues"`
	Failed         int64          `json:"failed"`
	SuccessRate    float64        `json:"success_rate"`
	TotalCostUSD   float64        `json:"total_cost_usd"`
	AvgCostUSD     float64        `json:"avg_cost_usd"`
	AvgDurationSec float64        `json:"avg_duration_sec"`
	AvgFixRounds   float64        `json:"avg_fix_rounds"`
	TopIssues      []bucketDTO    `json:"top_issues"`
	ByModel        []modelStatDTO `json:"by_model"`
}

type benchmarkDTO struct {
	Metric string  `json:"metric"`
	Unit   string  `json:"unit"`
	Median float64 `json:"median"`
	Good   float64 `json:"good"`
	Source string  `json:"source"`
	Note   string  `json:"note,omitempty"`
}

func (h *Handler) toPlanDTO(g domain.Game, p domain.Plan) planDTO {
	return planDTO{Game: h.toGameDTO(g), Plan: p.Spec, Reply: p.Reply}
}

func toStatusDTO(e domain.StatusEvent) statusDTO {
	return statusDTO{GameID: e.GameID, Status: e.Status, Detail: e.Detail, Version: e.Version, At: e.At,
		Progress: toProgressDTO(e.Progress)}
}

func toProgressDTO(p *domain.BuildProgress) *progressDTO {
	if p == nil {
		return nil
	}
	now := time.Now()
	out := &progressDTO{
		ElapsedMs: now.Sub(p.StartedAt).Milliseconds(), ExpectedMs: p.ExpectedMs,
		Role: p.Role, Chars: p.Chars, ExpectedChars: p.ExpectedChars, Step: p.Step,
	}
	if p.Role != "" && !p.PhaseStartedAt.IsZero() {
		out.PhaseMs = now.Sub(p.PhaseStartedAt).Milliseconds()
	}
	return out
}

func gameStatus(g domain.Game) domain.StatusEvent {
	return domain.StatusEvent{GameID: g.ID, Status: g.Status, Detail: g.StatusDetail, Version: g.Version, At: g.UpdatedAt}
}

func toGenerationDTOs(gs []domain.Generation) []generationItemDTO {
	out := make([]generationItemDTO, 0, len(gs))
	for _, g := range gs {
		calls := make([]llmCallDTO, 0, len(g.Calls))
		for _, c := range g.Calls {
			calls = append(calls, llmCallDTO(c))
		}
		issues := g.Issues
		if issues == nil {
			issues = []domain.Issue{}
		}
		out = append(out, generationItemDTO{
			ID: g.ID, Kind: g.Kind, Status: g.Status, Version: g.Version,
			PromptTokens: g.PromptTokens, CompletionTokens: g.CompletionTokens, CostUSD: g.CostUSD,
			DurationMs: g.DurationMs, FixRounds: g.FixRounds, Issues: issues,
			TestReport: json.RawMessage(g.TestReport), Error: g.Error, Calls: calls,
			HasScreenshot: g.HasScreenshot, CreatedAt: g.CreatedAt, FinishedAt: g.FinishedAt,
		})
	}
	return out
}

func toGenerationStatsDTO(s domain.GenerationStats) generationStatsDTO {
	top := make([]bucketDTO, 0, len(s.TopIssues))
	for _, b := range s.TopIssues {
		top = append(top, bucketDTO(b))
	}
	models := make([]modelStatDTO, 0, len(s.ByModel))
	for _, m := range s.ByModel {
		models = append(models, modelStatDTO(m))
	}
	return generationStatsDTO{
		Days: s.Days, Total: s.Total, Ready: s.Ready, WithIssues: s.WithIssues, Failed: s.Failed,
		SuccessRate: s.SuccessRate, TotalCostUSD: s.TotalCostUSD, AvgCostUSD: s.AvgCostUSD,
		AvgDurationSec: s.AvgDurationSec, AvgFixRounds: s.AvgFixRounds, TopIssues: top, ByModel: models,
	}
}

func toBenchmarkDTOs(bs []domain.Benchmark) []benchmarkDTO {
	out := make([]benchmarkDTO, 0, len(bs))
	for _, b := range bs {
		out = append(out, benchmarkDTO(b))
	}
	return out
}
