package http

import (
	"net/http"

	"beeline-games/internal/core/transport/http/request"
	"beeline-games/internal/core/transport/http/response"
)

func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var in registerRequest
	if err := request.Decode(r, &in); err != nil {
		response.Error(w, r, err)
		return
	}

	res, err := h.svc.CreateUser(r.Context(), in.Name, in.Role, in.ManagerCode)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusCreated, toAuthDTO(res))
}
