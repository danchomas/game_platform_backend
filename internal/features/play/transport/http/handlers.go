package http

import (
	"io"
	"net/http"

	"beeline-games/internal/core/transport/http/middleware"
	"beeline-games/internal/core/transport/http/request"
	"beeline-games/internal/core/transport/http/response"
)

const maxEventBody = 64 << 10

type Handler struct {
	svc Play
}

func NewHandler(svc Play) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux, authMW, optionalAuthMW middleware.Middleware) {
	mux.HandleFunc("GET /g/{id}", optionalAuthMW(h.Play))
	mux.HandleFunc("GET /api/games/{id}/export", authMW(h.Export))

	mux.HandleFunc("POST /api/events", h.Events)
}

func (h *Handler) Play(w http.ResponseWriter, r *http.Request) {
	gameID, err := request.PathID(r, "id")
	if err != nil {
		response.Error(w, r, err)
		return
	}
	userID, _ := request.UserID(r)

	html, published, err := h.svc.GetPlayableHTML(r.Context(), userID, gameID)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	if published {
		w.Header().Set("Cache-Control", "public, max-age=30")
	} else {
		w.Header().Set("Cache-Control", "no-store")
	}
	writeHTML(w, html)
}

func (h *Handler) Export(w http.ResponseWriter, r *http.Request) {
	userID, err := request.RequireUserID(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	gameID, err := request.PathID(r, "id")
	if err != nil {
		response.Error(w, r, err)
		return
	}

	html, _, err := h.svc.GetPlayableHTML(r.Context(), userID, gameID)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	w.Header().Set("Content-Disposition", `attachment; filename="game-`+gameID+`.html"`)
	writeHTML(w, html)
}

func (h *Handler) Events(w http.ResponseWriter, r *http.Request) {
	var in batchRequest
	if err := request.DecodeLimit(r, &in, maxEventBody); err != nil {
		response.Error(w, r, err)
		return
	}

	if err := h.svc.ProcessEvents(r.Context(), in.toDomain()); err != nil {
		response.Error(w, r, err)
		return
	}

	response.NoContent(w)
}

func writeHTML(w http.ResponseWriter, html string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.WriteString(w, html)
}
