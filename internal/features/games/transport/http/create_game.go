package http

import (
	"net/http"

	"beeline-games/internal/core/transport/http/request"
	"beeline-games/internal/core/transport/http/response"
)

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

	game, reply, version, err := h.svc.CreateAndGenerate(r.Context(), userID, in.Prompt)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusCreated, h.toGenerationDTO(game, reply, version))
}

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

	game, reply, version, err := h.svc.IterateGame(r.Context(), userID, gameID, in.Content)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, h.toGenerationDTO(game, reply, version))
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
