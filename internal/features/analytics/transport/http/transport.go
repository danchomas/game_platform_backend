package http

import (
	"beeline-games/internal/features/analytics/service"
	"context"
	"net/http"
)

// Обратите внимание: нам нужен интерфейс GamesProvider, чтобы проверить,
// имеет ли пользователь права смотреть аналитику конкретной игры.
// Мы не лезем в БД напрямую, а просим фичу Games.
type GamesProvider interface {
	CheckOwnership(ctx context.Context, userID, gameID string) (bool, error)
}

type Handler struct {
	svc   service.Service
	games GamesProvider
}

func NewHandler(svc service.Service, games GamesProvider) *Handler {
	return &Handler{svc: svc, games: games}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux, authMW func(http.HandlerFunc) http.HandlerFunc) {
	mux.HandleFunc("GET /api/analytics/overview", authMW(h.Overview))
	mux.HandleFunc("GET /api/analytics/games/{id}", authMW(h.GameAnalytics))
}
