package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"beeline-games/internal/core/domain"
	coreErrs "beeline-games/internal/core/errors"
	"beeline-games/internal/core/llm"
	"beeline-games/internal/core/transport/http/request"
	"beeline-games/internal/core/transport/http/response"
	"beeline-games/internal/features/games/service"
)

type promptReq struct {
	Prompt      string `json:"prompt"`
	AutoApprove bool   `json:"auto_approve"`
}

type chatReq struct {
	Content string `json:"content"`
}

type approveReq struct {
	Spec *domain.GameSpec `json:"spec"`
}

func decode(w http.ResponseWriter, r *http.Request, dst any, allowEmpty bool) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	err := json.NewDecoder(r.Body).Decode(dst)
	if err != nil && !(allowEmpty && errors.Is(err, io.EOF)) {
		response.Error(w, http.StatusBadRequest, "некорректный JSON")
		return false
	}
	return true
}

// CreateGame: промпт → план (синхронно). auto_approve=true сразу запускает сборку.
func (h *Handler) CreateGame(w http.ResponseWriter, r *http.Request) {
	userID, _ := request.UserID(r)
	var in promptReq
	if !decode(w, r, &in, false) {
		return
	}

	game, plan, err := h.svc.CreateGame(r.Context(), userID, in.Prompt, in.AutoApprove)
	if err != nil {
		handleGenErr(w, err, game.ID)
		return
	}
	code := http.StatusCreated
	if game.Status == domain.StatusGenerating {
		code = http.StatusAccepted
	}
	response.JSON(w, code, map[string]any{
		"game":  toDTO(game, h.publicBaseURL),
		"plan":  plan.Spec,
		"reply": plan.Reply,
	})
}

func (h *Handler) RevisePlan(w http.ResponseWriter, r *http.Request) {
	userID, _ := request.UserID(r)
	var in chatReq
	if !decode(w, r, &in, false) {
		return
	}
	game, plan, err := h.svc.RevisePlan(r.Context(), userID, r.PathValue("id"), in.Content)
	if err != nil {
		handleGenErr(w, err, "")
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{
		"game":  toDTO(game, h.publicBaseURL),
		"plan":  plan.Spec,
		"reply": plan.Reply,
	})
}

func (h *Handler) ApprovePlan(w http.ResponseWriter, r *http.Request) {
	userID, _ := request.UserID(r)
	var in approveReq
	if !decode(w, r, &in, true) {
		return
	}
	game, err := h.svc.ApprovePlan(r.Context(), userID, r.PathValue("id"), in.Spec)
	if err != nil {
		handleGenErr(w, err, "")
		return
	}
	response.JSON(w, http.StatusAccepted, map[string]any{"game": toDTO(game, h.publicBaseURL)})
}

// IterateGame: правка готовой игры в чате — фоновая задача, статус через /status или /events.
func (h *Handler) IterateGame(w http.ResponseWriter, r *http.Request) {
	userID, _ := request.UserID(r)
	var in chatReq
	if !decode(w, r, &in, false) {
		return
	}
	game, err := h.svc.IterateGame(r.Context(), userID, r.PathValue("id"), in.Content)
	if err != nil {
		handleGenErr(w, err, "")
		return
	}
	response.JSON(w, http.StatusAccepted, map[string]any{"game": toDTO(game, h.publicBaseURL)})
}

func handleGenErr(w http.ResponseWriter, err error, gameID string) {
	switch {
	case errors.Is(err, coreErrs.ErrInvalidInput):
		response.Error(w, http.StatusBadRequest, "текст: от 3 до 4000 символов (или игра в неподходящем состоянии)")
	case errors.Is(err, coreErrs.ErrNotFound):
		response.Error(w, http.StatusNotFound, "игра не найдена")
	case errors.Is(err, coreErrs.ErrForbidden):
		response.Error(w, http.StatusForbidden, "нет доступа")
	case errors.Is(err, service.ErrBusy):
		response.Error(w, http.StatusConflict, err.Error())
	case errors.Is(err, llm.ErrNoKey), strings.Contains(err.Error(), "сервер занят"):
		response.Error(w, http.StatusServiceUnavailable, err.Error())
	case errors.Is(err, context.Canceled):
		response.Error(w, 499, "запрос отменён")
	default:
		body := map[string]any{"error": "ИИ не смог выполнить запрос: " + err.Error()}
		if gameID != "" {
			body["game_id"] = gameID
		}
		response.JSON(w, http.StatusBadGateway, body)
	}
}
