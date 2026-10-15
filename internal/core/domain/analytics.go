package domain

import "time"

type Filter struct {
	OwnerID string
	GameID  string
	From    time.Time
	To      time.Time
}

type Summary struct {
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

func (s *Summary) Derive() {
	if s.Sessions > 0 {
		s.EngagementRate = float64(s.Engaged) / float64(s.Sessions)
	}
	if s.Starts > 0 {
		s.CompletionRate = float64(s.Ends) / float64(s.Starts)
	}
	if s.Ends > 0 {
		s.WinRate = float64(s.Wins) / float64(s.Ends)
	}
}

type DayPoint struct {
	Day      string `json:"day"`
	Sessions int64  `json:"sessions"`
	Starts   int64  `json:"game_starts"`
	Wins     int64  `json:"wins"`
	Losses   int64  `json:"losses"`
}

type GameRow struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Genre     string `json:"genre"`
	Published bool   `json:"published"`
	Plays     int64  `json:"plays_total"`
	Summary
}

type EventCount struct {
	Type     string `json:"type"`
	Count    int64  `json:"count"`
	Sessions int64  `json:"sessions"`
}

type Bucket struct {
	Label string `json:"label"`
	Count int64  `json:"count"`
}

type Insight struct {
	Title         string
	Genre         string
	Audience      string
	Sessions      int64
	AvgSessionSec float64
	WinRate       float64
}
