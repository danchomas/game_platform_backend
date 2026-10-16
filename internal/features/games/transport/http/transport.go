package http

import (
	"beeline-games/internal/features/games/service"
	"net/http"
)

type Handler struct {
	svc           service.Service
	publicBaseURL string
}

func NewHandler(svc service.Service, publicBaseURL string) *Handler {
	return &Handler{svc: svc, publicBaseURL: publicBaseURL}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux, authMW func(http.HandlerFunc) http.HandlerFunc) {
	// Генерация
	mux.HandleFunc("POST /api/games", authMW(h.CreateGame))
	mux.HandleFunc("POST /api/games/{id}/messages", authMW(h.IterateGame))

	// Чтение
	mux.HandleFunc("GET /api/games/mine", authMW(h.MyGames))
	mux.HandleFunc("GET /api/gallery", h.Gallery)
	mux.HandleFunc("GET /api/games/{id}", h.GetGame) // Публичный/приватный внутри

	// Редактирование
	mux.HandleFunc("PATCH /api/games/{id}", authMW(h.PatchGame))
	mux.HandleFunc("DELETE /api/games/{id}", authMW(h.DeleteGame))
	mux.HandleFunc("POST /api/games/{id}/publish", authMW(h.Publish))
	mux.HandleFunc("POST /api/games/{id}/unpublish", authMW(h.Unpublish))

	// Версии и чат
	mux.HandleFunc("GET /api/games/{id}/messages", authMW(h.Messages))
	mux.HandleFunc("GET /api/games/{id}/versions", authMW(h.Versions))
	mux.HandleFunc("POST /api/games/{id}/versions/{v}/restore", authMW(h.Restore))
}
