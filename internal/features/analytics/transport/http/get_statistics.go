package http

import (
	"errors"
	"net/http"
	"strconv"

	coreErrs "beeline-games/internal/core/errors"
	"beeline-games/internal/core/transport/http/request"
	"beeline-games/internal/core/transport/http/response"
)

func (h *Handler) Overview(w http.ResponseWriter, r *http.Request) {
	userID, _ := request.UserID(r)
	role, _ := request.Role(r)

	days := qInt(r, "days", 30, 1, 365)
	scope := r.URL.Query().Get("scope")

	data, err := h.svc.GetOverview(r.Context(), userID, role, scope, days)
	if err != nil {
		if errors.Is(err, coreErrs.ErrForbidden) {
			response.Error(w, http.StatusForbidden, "общая аналитика доступна только менеджерам")
			return
		}
		response.Error(w, http.StatusInternalServerError, "ошибка аналитики")
		return
	}

	response.JSON(w, http.StatusOK, data)
}

func (h *Handler) GameAnalytics(w http.ResponseWriter, r *http.Request) {
	userID, _ := request.UserID(r)
	role, _ := request.Role(r)
	gameID := r.PathValue("id")
	days := qInt(r, "days", 30, 1, 365)

	// Проверяем права через модуль Games (если не менеджер)
	if role != "manager" {
		isOwner, err := h.games.CheckOwnership(r.Context(), userID, gameID)
		if err != nil || !isOwner {
			response.Error(w, http.StatusForbidden, "нет доступа к аналитике этой игры")
			return
		}
	}

	data, err := h.svc.GetGameAnalytics(r.Context(), userID, role, gameID, days)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "ошибка аналитики")
		return
	}

	response.JSON(w, http.StatusOK, data)
}

func qInt(r *http.Request, key string, def, min, max int) int {
	n, err := strconv.Atoi(r.URL.Query().Get(key))
	if err != nil {
		return def
	}
	if n < min {
		return min
	}
	if n > max {
		return max
	}
	return n
}
