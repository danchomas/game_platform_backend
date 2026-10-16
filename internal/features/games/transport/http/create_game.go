package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	coreErrs "beeline-games/internal/core/errors"
	"beeline-games/internal/core/llm"
	"beeline-games/internal/core/transport/http/request"
	"beeline-games/internal/core/transport/http/response"
)

type promptReq struct {
	Prompt string `json:"prompt"`
}

type chatReq struct {
	Content string `json:"content"`
}

func (h *Handler) CreateGame(w http.ResponseWriter, r *http.Request) {
	userID, _ := request.UserID(r)

	var in promptReq
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		response.Error(w, http.StatusBadRequest, "некорректный JSON")
		return
	}

	game, reply, version, err := h.svc.CreateAndGenerate(r.Context(), userID, in.Prompt)
	if err != nil {
		handleGenErr(w, err)
		return
	}

	response.JSON(w, http.StatusCreated, map[string]any{
		"game":    game, // TODO: обернуть в DTO, если нужно скрыть поля
		"reply":   reply,
		"version": version,
	})
}

func (h *Handler) IterateGame(w http.ResponseWriter, r *http.Request) {
	userID, _ := request.UserID(r)
	gameID := r.PathValue("id") // фича Go 1.22

	var in chatReq
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		response.Error(w, http.StatusBadRequest, "некорректный JSON")
		return
	}

	game, reply, version, err := h.svc.IterateGame(r.Context(), userID, gameID, in.Content)
	if err != nil {
		if errors.Is(err, coreErrs.ErrForbidden) && game.ID == "" { // Хак для проверки "занято"
			response.Error(w, http.StatusConflict, "для этой игры уже идёт генерация")
			return
		}
		handleGenErr(w, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"game":    game,
		"reply":   reply,
		"version": version,
	})
}

func handleGenErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, coreErrs.ErrInvalidInput):
		response.Error(w, http.StatusBadRequest, "текст: от 3 до 4000 символов")
	case errors.Is(err, llm.ErrNoKey), strings.Contains(err.Error(), "сервер занят"):
		response.Error(w, http.StatusServiceUnavailable, err.Error())
	case errors.Is(err, context.Canceled):
		response.Error(w, 499, "запрос отменён")
	default:
		response.Error(w, http.StatusBadGateway, "ИИ не смог сгенерировать игру: "+err.Error())
	}
}
