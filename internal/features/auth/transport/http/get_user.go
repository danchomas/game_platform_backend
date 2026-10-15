package http

import (
	"encoding/json"
	"errors"
	"net/http"

	coreErrs "beeline-games/internal/core/errors"
	"beeline-games/internal/core/transport/http/request"
	"beeline-games/internal/core/transport/http/response"
)

func (h *Handler) LoginUser(w http.ResponseWriter, r *http.Request) {
	var in loginReq
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		response.Error(w, http.StatusBadRequest, "некорректный JSON")
		return
	}

	res, err := h.svc.LoginUser(r.Context(), in.AccessKey)
	if err != nil {
		if errors.Is(err, coreErrs.ErrUnauthorized) || errors.Is(err, coreErrs.ErrNotFound) {
			response.Error(w, http.StatusUnauthorized, "неверный код доступа")
			return
		}
		response.Error(w, http.StatusInternalServerError, "внутренняя ошибка")
		return
	}

	response.JSON(w, http.StatusOK, res)
}

func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
	userID, ok := request.UserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "неавторизован")
		return
	}

	u, err := h.svc.GetUser(r.Context(), userID)
	if err != nil {
		response.Error(w, http.StatusUnauthorized, "пользователь не найден")
		return
	}
	response.JSON(w, http.StatusOK, u)
}
