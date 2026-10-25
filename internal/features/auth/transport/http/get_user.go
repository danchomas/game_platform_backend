package http

import (
	"net/http"

	"beeline-games/internal/core/transport/http/request"
	"beeline-games/internal/core/transport/http/response"
)

func (h *Handler) LoginUser(w http.ResponseWriter, r *http.Request) {
	var in loginRequest
	if err := request.Decode(r, &in); err != nil {
		response.Error(w, r, err)
		return
	}

	res, err := h.svc.LoginUser(r.Context(), in.AccessKey)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, toAuthDTO(res))
}

func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
	userID, err := request.RequireUserID(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	u, err := h.svc.GetUser(r.Context(), userID)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, toUserDTO(u))
}
