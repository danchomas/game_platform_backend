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

// RegisterRoutes регистрирует пути. authMiddleware передаем снаружи (из Core).
func (h *Handler) RegisterRoutes(mux *http.ServeMux, authMiddleware func(http.HandlerFunc) http.HandlerFunc) {
	mux.HandleFunc("POST /api/auth/register", h.CreateUser)
	mux.HandleFunc("POST /api/auth/login", h.LoginUser)

	// Временно регистрируем без миддлвари, пока не напишем ее в Core.
	// Позже заменим на: mux.HandleFunc("GET /api/me", authMiddleware(h.GetUser))
	mux.HandleFunc("GET /api/me", h.GetUser)
}

// helpers (потом унесем в response)
func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func apiErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, errorRes{Error: msg})
}
