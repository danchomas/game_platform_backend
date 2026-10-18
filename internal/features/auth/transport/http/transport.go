package http

import (
	"beeline-games/internal/features/auth/service"
	"encoding/json"
	"net/http"
)

type Handler struct {
	svc service.Service
}

func NewHandler(svc service.Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux, authMiddleware func(http.HandlerFunc) http.HandlerFunc) {
	mux.HandleFunc("POST /api/auth/register", h.CreateUser)
	mux.HandleFunc("POST /api/auth/login", h.LoginUser)

	mux.HandleFunc("GET /api/me", h.GetUser)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func apiErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, errorRes{Error: msg})
}
