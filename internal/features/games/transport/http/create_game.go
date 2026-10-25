package http

import (
	"net/http"

	"beeline-games/internal/core/domain"
	"beeline-games/internal/core/transport/http/request"
	"beeline-games/internal/core/transport/http/response"
)

// CreateGame: промпт → план (201). auto_approve=true сразу запускает сборку (202).
func (h *Handler) CreateGame(w http.ResponseWriter, r *http.Request) {
	userID, err := request.RequireUserID(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	var in promptRequest
	if err := request.Decode(r, &in); err != nil {
		response.Error(w, r, err)
		return
	}

	game, plan, err := h.svc.CreateGame(r.Context(), userID, in.Prompt, in.AutoApprove)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	status := http.StatusCreated
	if game.Status == domain.StatusGenerating {
		status = http.StatusAccepted
	}
	response.JSON(w, status, h.toPlanDTO(game, plan))
}

func (h *Handler) RevisePlan(w http.ResponseWriter, r *http.Request) {
	userID, gameID, err := h.callerAndGame(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	var in messageRequest
	if err := request.Decode(r, &in); err != nil {
		response.Error(w, r, err)
		return
	}

	game, plan, err := h.svc.RevisePlan(r.Context(), userID, gameID, in.Content)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, h.toPlanDTO(game, plan))
}

// ApprovePlan запускает фоновую сборку (202); тело запроса необязательно (все поля опциональны).
func (h *Handler) ApprovePlan(w http.ResponseWriter, r *http.Request) {
	userID, gameID, err := h.callerAndGame(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	var in approveRequest
	if err := request.Decode(r, &in); err != nil && r.ContentLength != 0 {
		response.Error(w, r, err)
		return
	}

	game, err := h.svc.ApprovePlan(r.Context(), userID, gameID, in.Spec)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusAccepted, acceptedDTO{Game: h.toGameDTO(game)})
}

// IterateGame — правка в чате: сборка идёт в фоне (202), ход — через /status или /events.
func (h *Handler) IterateGame(w http.ResponseWriter, r *http.Request) {
	userID, gameID, err := h.callerAndGame(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	var in messageRequest
	if err := request.Decode(r, &in); err != nil {
		response.Error(w, r, err)
		return
	}

	game, err := h.svc.IterateGame(r.Context(), userID, gameID, in.Content)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusAccepted, acceptedDTO{Game: h.toGameDTO(game)})
}

func (h *Handler) callerAndGame(r *http.Request) (userID, gameID string, err error) {
	if userID, err = request.RequireUserID(r); err != nil {
		return "", "", err
	}
	if gameID, err = request.PathID(r, "id"); err != nil {
		return "", "", err
	}
	return userID, gameID, nil
}
