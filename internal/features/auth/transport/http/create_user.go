package http

import (
	coreErrs "beeline-games/internal/core/errors"
	"encoding/json"
	"errors"
	"net/http"
)

func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var in registerReq

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		apiErr(w, http.StatusBadRequest, "некорректный JSON")
		return
	}

	res, err := h.svc.CreateUser(r.Context(), in.Name, in.Role, in.ManagerCode)
	if err != nil {
		if errors.Is(err, coreErrs.ErrInvalidInput) {
			apiErr(w, http.StatusBadRequest, "имя: от 2 до 40 символов")
			return
		}
		if errors.Is(err, coreErrs.ErrForbidden) {
			apiErr(w, http.StatusForbidden, "неверный код менеджера")
			return
		}
		apiErr(w, http.StatusInternalServerError, "не удалось создать пользователя")
		return
	}

	writeJSON(w, http.StatusCreated, res)
}
