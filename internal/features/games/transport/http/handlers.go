package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	coreErrs "beeline-games/internal/core/errors"
	"beeline-games/internal/core/transport/http/request"
	"beeline-games/internal/core/transport/http/response"
)

func (h *Handler) GetGame(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	// Попытка достать ID, если передан токен, но не падаем, если его нет (аноним)
	userID, _ := request.UserID(r)

	g, err := h.svc.GetGame(r.Context(), userID, id)
	if err != nil {
		response.Error(w, http.StatusNotFound, "игра не найдена")
		return
	}
	response.JSON(w, http.StatusOK, toDTO(g, h.publicBaseURL))
}

func (h *Handler) MyGames(w http.ResponseWriter, r *http.Request) {
	userID, _ := request.UserID(r)
	limit := qInt(r, "limit", 50, 1, 200)
	offset := qInt(r, "offset", 0, 0, 1<<20)

	gs, err := h.svc.GetMyGames(r.Context(), userID, limit, offset)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "внутренняя ошибка")
		return
	}
	response.JSON(w, http.StatusOK, toDTOs(gs, h.publicBaseURL))
}

func (h *Handler) Gallery(w http.ResponseWriter, r *http.Request) {
	sort := r.URL.Query().Get("sort")
	q := r.URL.Query().Get("q")
	limit := qInt(r, "limit", 24, 1, 100)
	offset := qInt(r, "offset", 0, 0, 1<<20)

	gs, err := h.svc.GetGallery(r.Context(), sort, q, limit, offset)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "внутренняя ошибка")
		return
	}
	response.JSON(w, http.StatusOK, toDTOs(gs, h.publicBaseURL))
}

func (h *Handler) PatchGame(w http.ResponseWriter, r *http.Request) {
	userID, _ := request.UserID(r)
	id := r.PathValue("id")

	var in struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		response.Error(w, http.StatusBadRequest, "некорректный JSON")
		return
	}

	g, err := h.svc.PatchGame(r.Context(), userID, id, in.Title)
	if err != nil {
		handleCRUDErr(w, err)
		return
	}
	response.JSON(w, http.StatusOK, toDTO(g, h.publicBaseURL))
}

func (h *Handler) DeleteGame(w http.ResponseWriter, r *http.Request) {
	userID, _ := request.UserID(r)
	id := r.PathValue("id")

	if err := h.svc.DeleteGame(r.Context(), userID, id); err != nil {
		handleCRUDErr(w, err)
		return
	}
	response.NoContent(w)
}

func (h *Handler) Publish(w http.ResponseWriter, r *http.Request) {
	h.setPublishStatus(w, r, true)
}

func (h *Handler) Unpublish(w http.ResponseWriter, r *http.Request) {
	h.setPublishStatus(w, r, false)
}

func (h *Handler) setPublishStatus(w http.ResponseWriter, r *http.Request, status bool) {
	userID, _ := request.UserID(r)
	id := r.PathValue("id")

	g, err := h.svc.PublishGame(r.Context(), userID, id, status)
	if err != nil {
		if errors.Is(err, coreErrs.ErrInvalidInput) {
			response.Error(w, http.StatusBadRequest, "в игре ещё нет сгенерированного кода")
			return
		}
		handleCRUDErr(w, err)
		return
	}
	response.JSON(w, http.StatusOK, toDTO(g, h.publicBaseURL))
}

func (h *Handler) Messages(w http.ResponseWriter, r *http.Request) {
	userID, _ := request.UserID(r)
	id := r.PathValue("id")

	msgs, err := h.svc.GetMessages(r.Context(), userID, id)
	if err != nil {
		handleCRUDErr(w, err)
		return
	}
	response.JSON(w, http.StatusOK, msgs)
}

func (h *Handler) Versions(w http.ResponseWriter, r *http.Request) {
	userID, _ := request.UserID(r)
	id := r.PathValue("id")

	vs, err := h.svc.GetVersions(r.Context(), userID, id)
	if err != nil {
		handleCRUDErr(w, err)
		return
	}
	response.JSON(w, http.StatusOK, vs)
}

func (h *Handler) Restore(w http.ResponseWriter, r *http.Request) {
	userID, _ := request.UserID(r)
	id := r.PathValue("id")

	ver, err := strconv.Atoi(r.PathValue("v"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "некорректная версия")
		return
	}

	game, reply, version, err := h.svc.RestoreVersion(r.Context(), userID, id, ver)
	if err != nil {
		handleCRUDErr(w, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"game":    toDTO(game, h.publicBaseURL),
		"reply":   reply,
		"version": version,
	})
}

// helpers
func handleCRUDErr(w http.ResponseWriter, err error) {
	if errors.Is(err, coreErrs.ErrNotFound) {
		response.Error(w, http.StatusNotFound, "не найдено")
	} else if errors.Is(err, coreErrs.ErrForbidden) {
		response.Error(w, http.StatusForbidden, "нет доступа")
	} else if errors.Is(err, coreErrs.ErrInvalidInput) {
		response.Error(w, http.StatusBadRequest, "некорректные данные")
	} else {
		response.Error(w, http.StatusInternalServerError, "внутренняя ошибка")
	}
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
