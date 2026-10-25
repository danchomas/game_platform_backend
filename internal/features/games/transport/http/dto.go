package http

import (
	"time"

	"beeline-games/internal/core/domain"
)

type promptRequest struct {
	Prompt string `json:"prompt"`
}

type messageRequest struct {
	Content string `json:"content"`
}

type patchRequest struct {
	Title string `json:"title"`
}

type gameDTO struct {
	ID        string    `json:"id"`
	OwnerID   string    `json:"owner_id"`
	OwnerName string    `json:"owner_name"`
	Title     string    `json:"title"`
	Genre     string    `json:"genre"`
	Audience  string    `json:"audience"`
	Prompt    string    `json:"prompt"`
	HTML      string    `json:"html,omitempty"`
	Version   int       `json:"version"`
	Published bool      `json:"published"`
	Plays     int64     `json:"plays"`
	PlayURL   string    `json:"play_url"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
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
		ID:        g.ID,
		OwnerID:   g.OwnerID,
		OwnerName: g.OwnerName,
		Title:     g.Title,
		Genre:     g.Genre,
		Audience:  g.Audience,
		Prompt:    g.Prompt,
		HTML:      g.HTML,
		Version:   g.Version,
		Published: g.Published,
		Plays:     g.Plays,
		PlayURL:   h.publicBaseURL + "/g/" + g.ID,
		CreatedAt: g.CreatedAt,
		UpdatedAt: g.UpdatedAt,
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
