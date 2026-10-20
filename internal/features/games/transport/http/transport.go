package http

import (
	"beeline-games/internal/features/games/service"
	"net/http"
)

type Handler struct {
	svc		service.Service
	publicBaseURL	string
}

func NewHandler(svc service.Service, publicBaseURL string) *Handler {
	return &Handler{svc: svc, publicBaseURL: publicBaseURL}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux, authMW func(http.HandlerFunc) http.HandlerFunc) {

	// агентный конвейер: план → правка плана → подтверждение → фоновая сборка → правки в чате
	mux.HandleFunc("POST /api/games", authMW(h.CreateGame))
	mux.HandleFunc("POST /api/games/{id}/plan", authMW(h.RevisePlan))
	mux.HandleFunc("POST /api/games/{id}/approve", authMW(h.ApprovePlan))
	mux.HandleFunc("POST /api/games/{id}/messages", authMW(h.IterateGame))
	mux.HandleFunc("GET /api/games/{id}/status", authMW(h.Status))
	mux.HandleFunc("GET /api/games/{id}/events", authMW(h.Events))
	mux.HandleFunc("GET /api/games/{id}/generations", authMW(h.Generations))
	mux.HandleFunc("GET /api/games/{id}/preview.jpg", h.Preview)
	mux.HandleFunc("GET /api/generations/stats", authMW(h.GenerationStats))
	mux.HandleFunc("GET /api/benchmarks", h.Benchmarks)

	mux.HandleFunc("GET /api/games/mine", authMW(h.MyGames))
	mux.HandleFunc("GET /api/gallery", h.Gallery)
	mux.HandleFunc("GET /api/games/{id}", h.GetGame)

	mux.HandleFunc("PATCH /api/games/{id}", authMW(h.PatchGame))
	mux.HandleFunc("DELETE /api/games/{id}", authMW(h.DeleteGame))
	mux.HandleFunc("POST /api/games/{id}/publish", authMW(h.Publish))
	mux.HandleFunc("POST /api/games/{id}/unpublish", authMW(h.Unpublish))

	mux.HandleFunc("GET /api/games/{id}/messages", authMW(h.Messages))
	mux.HandleFunc("GET /api/games/{id}/versions", authMW(h.Versions))
	mux.HandleFunc("POST /api/games/{id}/versions/{v}/restore", authMW(h.Restore))
}
