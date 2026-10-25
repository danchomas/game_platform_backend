package domain

import "time"

type Filter struct {
	OwnerID string
	GameID  string
	From    time.Time
	To      time.Time
}

const (
	ScopeMine = "mine"
	ScopeAll  = "all"
)

type Summary struct {
	Sessions         int64
	Players          int64
	ReturningPlayers int64
	Engaged          int64
	Starts           int64
	Ends             int64
	Wins             int64
	Losses           int64
	AvgSessionSec    float64
	AvgScore         float64
	MaxScore         float64
	EngagementRate   float64
	CompletionRate   float64
	WinRate          float64
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
	Day      string
	Sessions int64
	Starts   int64
	Wins     int64
	Losses   int64
}

type GameRow struct {
	ID        string
	Title     string
	Genre     string
	Published bool
	Plays     int64
	Summary
}

type EventCount struct {
	Type     string
	Count    int64
	Sessions int64
}

type Bucket struct {
	Label string
	Count int64
}

type Insight struct {
	Title         string
	Genre         string
	Audience      string
	Sessions      int64
	AvgSessionSec float64
	WinRate       float64
}

type Dashboard struct {
	Scope           string
	Days            int
	Summary         Summary
	Daily           []DayPoint
	Games           []GameRow
	CustomEvents    []EventCount
	SessionDuration []Bucket
}
