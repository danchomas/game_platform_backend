package http

import (
	"time"

	"beeline-games/internal/core/domain"
)

type createRequest struct {
	Prompt string `json:"prompt"`
	// Variants — сколько вариантов собрать (2–4); 0 — по умолчанию.
	Variants int `json:"variants"`
}

type winnerRequest struct {
	GameID string `json:"game_id"`
}

type experimentDTO struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Prompt   string `json:"prompt"`
	Status   string `json:"status"`
	WinnerID string `json:"winner_id,omitempty"`
	// WinnerSource — кто выбрал победителя: manual (автор) или auto (бандит).
	WinnerSource string `json:"winner_source,omitempty"`
	// PlayURL — общая ссылка: раздаёт игрокам варианты, после выбора победителя — его.
	PlayURL       string     `json:"play_url"`
	VariantsTotal int        `json:"variants_total"`
	VariantsReady int        `json:"variants_ready"`
	CreatedAt     time.Time  `json:"created_at"`
	LaunchedAt    *time.Time `json:"launched_at,omitempty"`
	FinishedAt    *time.Time `json:"finished_at,omitempty"`
}

type variantDTO struct {
	GameID       string `json:"game_id"`
	Label        string `json:"label"`
	Direction    string `json:"direction"`
	Title        string `json:"title"`
	Genre        string `json:"genre"`
	Status       string `json:"status"`
	StatusDetail string `json:"status_detail,omitempty"`
	Version      int    `json:"version"`
	Published    bool   `json:"published"`
	BuildStatus  string `json:"build_status,omitempty"`
	// Eligible — прошёл автопроверки; RejectReason объясняет, почему нет.
	Eligible     bool    `json:"eligible"`
	RejectReason string  `json:"reject_reason,omitempty"`
	CostUSD      float64 `json:"cost_usd"`
	FixRounds    int     `json:"fix_rounds"`
	PlayURL      string  `json:"play_url"`
	PreviewURL   string  `json:"preview_url"`
}

type experimentViewDTO struct {
	Experiment experimentDTO `json:"experiment"`
	Variants   []variantDTO  `json:"variants"`
}

type variantStatsDTO struct {
	Exposures      int64   `json:"exposures"`
	Sessions       int64   `json:"sessions"`
	Engaged        int64   `json:"engaged"`
	Retained       int64   `json:"retained"`
	ErrorSessions  int64   `json:"error_sessions"`
	Players        int64   `json:"players"`
	Starts         int64   `json:"starts"`
	Ends           int64   `json:"ends"`
	Wins           int64   `json:"wins"`
	AvgSessionSec  float64 `json:"avg_session_sec"`
	LoadRate       float64 `json:"load_rate"`
	EngagementRate float64 `json:"engagement_rate"`
	RetentionRate  float64 `json:"retention_rate"`
	RetentionLow   float64 `json:"retention_low"`
	RetentionHigh  float64 `json:"retention_high"`
	ErrorRate      float64 `json:"error_rate"`
	CompletionRate float64 `json:"completion_rate"`
	WinRate        float64 `json:"win_rate"`
	// TrafficShare — доля показов, которую вариант получил от бандита.
	TrafficShare float64 `json:"traffic_share"`
	// ProbBest — вероятность, что у варианта лучшее удержание.
	ProbBest float64 `json:"prob_best"`
}

type reportVariantDTO struct {
	variantDTO
	Stats variantStatsDTO `json:"stats"`
}

type reportDTO struct {
	Experiment  experimentDTO      `json:"experiment"`
	Variants    []reportVariantDTO `json:"variants"`
	LeaderID    string             `json:"leader_id,omitempty"`
	Significant bool               `json:"significant"`
	// RetainedSeconds — порог удержания, по которому сравниваются варианты.
	RetainedSeconds int `json:"retained_seconds"`
	// Allocation — как ссылка распределяет новых игроков.
	Allocation string `json:"allocation"`
	// AutoWinProb, AutoMinTrials (лидеру) и AutoMinRivalTrials (остальным) — правило,
	// по которому бандит сам выбирает победителя.
	AutoWinProb        float64 `json:"auto_win_prob"`
	AutoMinTrials      int     `json:"auto_min_trials"`
	AutoMinRivalTrials int     `json:"auto_min_rival_trials"`
}

func (h *Handler) toExperimentDTO(x domain.Experiment) experimentDTO {
	return experimentDTO{
		ID:            x.ID,
		Title:         x.Title,
		Prompt:        x.Prompt,
		Status:        x.Status,
		WinnerID:      x.WinnerID,
		WinnerSource:  x.WinnerSource,
		PlayURL:       h.publicBaseURL + "/e/" + x.ID,
		VariantsTotal: x.Variants,
		VariantsReady: x.Ready,
		CreatedAt:     x.CreatedAt,
		LaunchedAt:    x.LaunchedAt,
		FinishedAt:    x.FinishedAt,
	}
}

func (h *Handler) toVariantDTO(v domain.Variant) variantDTO {
	return variantDTO{
		GameID:       v.GameID,
		Label:        v.Label,
		Direction:    v.Direction,
		Title:        v.Title,
		Genre:        v.Genre,
		Status:       v.Status,
		StatusDetail: v.StatusDetail,
		Version:      v.Version,
		Published:    v.Published,
		BuildStatus:  v.BuildStatus,
		Eligible:     v.Eligible(),
		RejectReason: v.RejectReason(),
		CostUSD:      v.CostUSD,
		FixRounds:    v.FixRounds,
		PlayURL:      h.publicBaseURL + "/g/" + v.GameID,
		PreviewURL:   h.publicBaseURL + "/api/games/" + v.GameID + "/preview.jpg",
	}
}

func (h *Handler) toViewDTO(x domain.Experiment, variants []domain.Variant) experimentViewDTO {
	out := experimentViewDTO{Experiment: h.toExperimentDTO(x), Variants: make([]variantDTO, 0, len(variants))}
	for _, v := range variants {
		out.Variants = append(out.Variants, h.toVariantDTO(v))
	}
	return out
}

func (h *Handler) toReportDTO(rep domain.ExperimentReport) reportDTO {
	byGame := make(map[string]domain.VariantStats, len(rep.Stats))
	for _, st := range rep.Stats {
		byGame[st.GameID] = st
	}

	out := reportDTO{
		Experiment:         h.toExperimentDTO(rep.Experiment),
		Variants:           make([]reportVariantDTO, 0, len(rep.Variants)),
		LeaderID:           rep.LeaderID,
		Significant:        rep.Significant,
		RetainedSeconds:    domain.RetainedSeconds,
		Allocation:         "thompson",
		AutoWinProb:        rep.AutoWinProb,
		AutoMinTrials:      rep.AutoMinTrials,
		AutoMinRivalTrials: rep.AutoMinRivalTrials,
	}
	for _, v := range rep.Variants {
		out.Variants = append(out.Variants, reportVariantDTO{
			variantDTO: h.toVariantDTO(v),
			Stats:      toStatsDTO(byGame[v.GameID]),
		})
	}
	return out
}

func toStatsDTO(st domain.VariantStats) variantStatsDTO {
	if st.GameID == "" {
		// варианта нет в статистике: показов не было, интервал — весь отрезок [0, 1]
		st.RetentionLow, st.RetentionHigh = domain.Wilson(0, 0)
	}
	return variantStatsDTO{
		Exposures:      st.Exposures,
		Sessions:       st.Sessions,
		Engaged:        st.Engaged,
		Retained:       st.Retained,
		ErrorSessions:  st.ErrorSessions,
		Players:        st.Players,
		Starts:         st.Starts,
		Ends:           st.Ends,
		Wins:           st.Wins,
		AvgSessionSec:  st.AvgSessionSec,
		LoadRate:       st.LoadRate,
		EngagementRate: st.EngagementRate,
		RetentionRate:  st.RetentionRate,
		RetentionLow:   st.RetentionLow,
		RetentionHigh:  st.RetentionHigh,
		ErrorRate:      st.ErrorRate,
		CompletionRate: st.CompletionRate,
		WinRate:        st.WinRate,
		TrafficShare:   st.TrafficShare,
		ProbBest:       st.ProbBest,
	}
}
