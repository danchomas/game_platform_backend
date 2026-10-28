package http

import (
	"net/http"

	"beeline-games/internal/core/transport/http/middleware"
)

type Handler struct {
	svc           Experiments
	publicBaseURL string
}

func NewHandler(svc Experiments, publicBaseURL string) *Handler {
	return &Handler{svc: svc, publicBaseURL: publicBaseURL}
}

// RegisterRoutes: мультигенерация → отсев → запуск ссылки → дашборд → победитель.
// Саму ссылку /e/{id} отдаёт фича play: она же встраивает SDK в игры.
func (h *Handler) RegisterRoutes(mux *http.ServeMux, authMW middleware.Middleware) {
	mux.HandleFunc("POST /api/experiments", authMW(h.Create))
	mux.HandleFunc("GET /api/experiments", authMW(h.List))
	mux.HandleFunc("GET /api/experiments/{id}", authMW(h.Get))
	mux.HandleFunc("POST /api/experiments/{id}/launch", authMW(h.Launch))
	mux.HandleFunc("POST /api/experiments/{id}/winner", authMW(h.ChooseWinner))
	mux.HandleFunc("GET /api/experiments/{id}/report", authMW(h.Report))
}
