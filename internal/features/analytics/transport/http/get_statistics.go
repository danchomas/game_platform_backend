package http

import (
	"net/http"

	"beeline-games/internal/core/transport/http/request"
	"beeline-games/internal/core/transport/http/response"
)

func (h *Handler) Overview(w http.ResponseWriter, r *http.Request) {
	userID, err := request.RequireUserID(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}
	role, _ := request.Role(r)

	data, err := h.svc.GetOverview(r.Context(), userID, role,
		request.Str(r, "scope"), request.Int(r, "days", 0))
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, toDashboardDTO(data))
}

func (h *Handler) GameAnalytics(w http.ResponseWriter, r *http.Request) {
	userID, err := request.RequireUserID(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}
	role, _ := request.Role(r)

	gameID, err := request.PathID(r, "id")
	if err != nil {
		response.Error(w, r, err)
		return
	}

	data, err := h.svc.GetGameAnalytics(r.Context(), userID, role, gameID, request.Int(r, "days", 0))
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, toDashboardDTO(data))
}
