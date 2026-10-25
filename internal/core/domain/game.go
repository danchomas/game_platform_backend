package domain

import "time"

type Game struct {
	ID        string
	OwnerID   string
	OwnerName string
	Title     string
	Genre     string
	Audience  string
	Prompt    string
	HTML      string
	Version   int
	Published bool
	Plays     int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (g Game) Generated() bool { return g.Version > 0 && g.HTML != "" }

type Message struct {
	ID        int64
	Role      string
	Content   string
	Version   *int
	CreatedAt time.Time
}

type VersionInfo struct {
	Version     int
	Instruction string
	CreatedAt   time.Time
}

type VersionInput struct {
	GameID   string
	UserText string
	Reply    string
	HTML     string
	Title    string
	Genre    string
	Audience string
}

type GameQuery struct {
	Sort   string
	Search string
	Limit  int
	Offset int
}

const (
	SortNew     = "new"
	SortPopular = "popular"
)
