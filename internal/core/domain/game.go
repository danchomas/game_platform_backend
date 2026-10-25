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
	// Status — шаг агентного конвейера (domain.Status*), StatusDetail — пояснение к нему.
	Status       string
	StatusDetail string
	// Spec — утверждаемый план игры (JSON domain.GameSpec), пусто до планирования.
	Spec      []byte
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (g Game) Generated() bool { return g.Version > 0 && g.HTML != "" }

// PlanStage — план ещё можно править или утверждать: кода пока нет.
func (g Game) PlanStage() bool {
	return g.Version == 0 && (g.Status == StatusAwaitingApproval || g.Status == StatusFailed)
}

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
	// Instruction — подпись версии в истории; пусто — берётся UserText.
	Instruction string
	Reply       string
	HTML        string
	Title       string
	Genre       string
	Audience    string
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
