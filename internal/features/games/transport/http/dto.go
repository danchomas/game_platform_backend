package http

import "beeline-games/internal/core/domain"

type gameDTO struct {
	domain.Game
	PlayURL string `json:"play_url"`
}

func toDTO(g domain.Game, publicBaseURL string) gameDTO {
	return gameDTO{Game: g, PlayURL: publicBaseURL + "/g/" + g.ID}
}

func toDTOs(gs []domain.Game, publicBaseURL string) []gameDTO {
	out := make([]gameDTO, len(gs))
	for i, g := range gs {
		out[i] = toDTO(g, publicBaseURL)
	}
	return out
}
