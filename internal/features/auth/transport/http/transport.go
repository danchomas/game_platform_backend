package http

import (
	"net/http"

	"beeline-games/internal/core/transport/http/middleware"
)

type Handler struct {
	svc Auth
}

func NewHandler(svc Auth) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux, authMW middleware.Middleware) {
	mux.HandleFunc("POST /api/auth/register", h.CreateUser)
	mux.HandleFunc("POST /api/auth/login", h.LoginUser)

	mux.HandleFunc("GET /api/me", authMW(h.GetUser))
}
