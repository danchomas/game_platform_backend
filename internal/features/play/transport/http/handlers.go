package http

import (
	"encoding/json"
	"io"
	"net/http"

	"beeline-games/internal/core/transport/http/request"
	"beeline-games/internal/core/transport/http/response"
	"beeline-games/internal/features/play/service"
)

type Handler struct {
	svc service.Service
}

func NewHandler(svc service.Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux, authMW func(http.HandlerFunc) http.HandlerFunc) {
	mux.HandleFunc("GET /g/{id}", h.Play)
	mux.HandleFunc("GET /api/games/{id}/export", authMW(h.Export))
	mux.HandleFunc("POST /api/events", h.Events)
}

func (h *Handler) Play(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	userID, _ := request.UserID(r)

	html, published, err := h.svc.GetPlayableHTML(r.Context(), userID, id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if !published {
		w.Header().Set("Cache-Control", "no-store")
	} else {
		w.Header().Set("Cache-Control", "public, max-age=30")
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.WriteString(w, html)
}

func (h *Handler) Export(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	userID, ok := request.UserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "нужна авторизация")
		return
	}

	html, _, err := h.svc.GetPlayableHTML(r.Context(), userID, id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="game-`+id+`.html"`)
	_, _ = io.WriteString(w, html)
}

func (h *Handler) Events(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<10))
	if err != nil {
		response.Error(w, http.StatusRequestEntityTooLarge, "слишком большой запрос")
		return
	}

	var batch service.BatchIn
	if err := json.Unmarshal(body, &batch); err != nil {
		response.Error(w, http.StatusBadRequest, "некорректный пакет событий")
		return
	}

	if err := h.svc.ProcessEvents(r.Context(), batch); err != nil {
		response.Error(w, http.StatusBadRequest, "некорректные данные")
		return
	}

	response.NoContent(w)
}
