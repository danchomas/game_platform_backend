package http

import (
	"net/http"

	"beeline-games/internal/core/transport/http/middleware"
)

type Handler struct {
	svc Analytics
}

func NewHandler(svc Analytics) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux, authMW middleware.Middleware) {
	mux.HandleFunc("GET /api/analytics/overview", authMW(h.Overview))
	mux.HandleFunc("GET /api/analytics/games/{id}", authMW(h.GameAnalytics))
}
