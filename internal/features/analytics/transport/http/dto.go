package http

import "beeline-games/internal/core/domain"

type dashboardDTO struct {
	Scope           string          `json:"scope"`
	Days            int             `json:"days"`
	Summary         summaryDTO      `json:"summary"`
	Daily           []dayPointDTO   `json:"daily"`
	Games           []gameRowDTO    `json:"games,omitempty"`
	CustomEvents    []eventCountDTO `json:"custom_events"`
	SessionDuration []bucketDTO     `json:"session_duration"`
}

type summaryDTO struct {
	Sessions         int64   `json:"sessions"`
	Players          int64   `json:"players"`
	ReturningPlayers int64   `json:"returning_players"`
	Engaged          int64   `json:"engaged_sessions"`
	Starts           int64   `json:"game_starts"`
	Ends             int64   `json:"game_ends"`
	Wins             int64   `json:"wins"`
	Losses           int64   `json:"losses"`
	AvgSessionSec    float64 `json:"avg_session_sec"`
	AvgScore         float64 `json:"avg_score"`
	MaxScore         float64 `json:"max_score"`
	EngagementRate   float64 `json:"engagement_rate"`
	CompletionRate   float64 `json:"completion_rate"`
	WinRate          float64 `json:"win_rate"`
}

type dayPointDTO struct {
	Day      string `json:"day"`
	Sessions int64  `json:"sessions"`
	Starts   int64  `json:"game_starts"`
	Wins     int64  `json:"wins"`
	Losses   int64  `json:"losses"`
}

type gameRowDTO struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Genre     string `json:"genre"`
	Published bool   `json:"published"`
	Plays     int64  `json:"plays_total"`
	summaryDTO
}

type eventCountDTO struct {
	Type     string `json:"type"`
	Count    int64  `json:"count"`
	Sessions int64  `json:"sessions"`
}

type bucketDTO struct {
	Label string `json:"label"`
	Count int64  `json:"count"`
}

func toDashboardDTO(d domain.Dashboard) dashboardDTO {
	return dashboardDTO{
		Scope:           d.Scope,
		Days:            d.Days,
		Summary:         toSummaryDTO(d.Summary),
		Daily:           toDayPointDTOs(d.Daily),
		Games:           toGameRowDTOs(d.Games),
		CustomEvents:    toEventCountDTOs(d.CustomEvents),
		SessionDuration: toBucketDTOs(d.SessionDuration),
	}
}

func toSummaryDTO(s domain.Summary) summaryDTO {
	return summaryDTO{
		Sessions:         s.Sessions,
		Players:          s.Players,
		ReturningPlayers: s.ReturningPlayers,
		Engaged:          s.Engaged,
		Starts:           s.Starts,
		Ends:             s.Ends,
		Wins:             s.Wins,
		Losses:           s.Losses,
		AvgSessionSec:    s.AvgSessionSec,
		AvgScore:         s.AvgScore,
		MaxScore:         s.MaxScore,
		EngagementRate:   s.EngagementRate,
		CompletionRate:   s.CompletionRate,
		WinRate:          s.WinRate,
	}
}

func toDayPointDTOs(ps []domain.DayPoint) []dayPointDTO {
	out := make([]dayPointDTO, 0, len(ps))
	for _, p := range ps {
		out = append(out, dayPointDTO{
			Day:      p.Day,
			Sessions: p.Sessions,
			Starts:   p.Starts,
			Wins:     p.Wins,
			Losses:   p.Losses,
		})
	}
	return out
}

func toGameRowDTOs(rows []domain.GameRow) []gameRowDTO {
	if len(rows) == 0 {
		return nil
	}
	out := make([]gameRowDTO, 0, len(rows))
	for _, r := range rows {
		out = append(out, gameRowDTO{
			ID:         r.ID,
			Title:      r.Title,
			Genre:      r.Genre,
			Published:  r.Published,
			Plays:      r.Plays,
			summaryDTO: toSummaryDTO(r.Summary),
		})
	}
	return out
}

func toEventCountDTOs(cs []domain.EventCount) []eventCountDTO {
	out := make([]eventCountDTO, 0, len(cs))
	for _, c := range cs {
		out = append(out, eventCountDTO{Type: c.Type, Count: c.Count, Sessions: c.Sessions})
	}
	return out
}

func toBucketDTOs(bs []domain.Bucket) []bucketDTO {
	out := make([]bucketDTO, 0, len(bs))
	for _, b := range bs {
		out = append(out, bucketDTO{Label: b.Label, Count: b.Count})
	}
	return out
}
