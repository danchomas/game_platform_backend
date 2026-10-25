package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"beeline-games/internal/core/domain"
	"beeline-games/internal/core/transport/http/request"
	"beeline-games/internal/core/transport/http/response"
)

const ssePing = 20 * time.Second

// Status — для опроса без SSE: статус конвейера и карточка игры без HTML.
func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	userID, gameID, err := h.callerAndGame(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	g, progress, err := h.svc.GetStatus(r.Context(), userID, gameID)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	st := gameStatus(g)
	st.Progress = progress
	response.JSON(w, http.StatusOK, statusWithGameDTO{statusDTO: toStatusDTO(st), Game: h.toGameDTO(g)})
}

// Events — SSE-поток смен статуса. EventSource не умеет заголовки: токен через ?token=.
func (h *Handler) Events(w http.ResponseWriter, r *http.Request) {
	userID, gameID, err := h.callerAndGame(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	g, events, cancel, err := h.svc.Subscribe(r.Context(), userID, gameID)
	if err != nil {
		response.Error(w, r, err)
		return
	}
	defer cancel()

	rc := http.NewResponseController(w)
	// поток живёт дольше WriteTimeout сервера
	_ = rc.SetWriteDeadline(time.Time{})
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	send := func(e domain.StatusEvent) bool {
		b, _ := json.Marshal(toStatusDTO(e))
		if _, err := fmt.Fprintf(w, "event: status\ndata: %s\n\n", b); err != nil {
			return false
		}
		return rc.Flush() == nil
	}
	if !send(gameStatus(g)) {
		return
	}

	ping := time.NewTicker(ssePing)
	defer ping.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case e := <-events:
			if !send(e) {
				return
			}
		case <-ping.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil || rc.Flush() != nil {
				return
			}
		}
	}
}

func (h *Handler) Generations(w http.ResponseWriter, r *http.Request) {
	userID, gameID, err := h.callerAndGame(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	gens, err := h.svc.ListGenerations(r.Context(), userID, gameID)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, toGenerationDTOs(gens))
}

// Preview — скриншот игры из последнего теста в браузере (карточка в галерее).
func (h *Handler) Preview(w http.ResponseWriter, r *http.Request) {
	gameID, err := request.PathID(r, "id")
	if err != nil {
		response.Error(w, r, err)
		return
	}
	userID, _ := request.UserID(r)

	img, err := h.svc.Screenshot(r.Context(), userID, gameID)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "private, max-age=60")
	_, _ = w.Write(img)
}

func (h *Handler) GenerationStats(w http.ResponseWriter, r *http.Request) {
	if _, err := request.RequireUserID(r); err != nil {
		response.Error(w, r, err)
		return
	}
	role, _ := request.Role(r)

	st, err := h.svc.GenerationStats(r.Context(), role, request.Int(r, "days", 30))
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, toGenerationStatsDTO(st))
}

func (h *Handler) Benchmarks(w http.ResponseWriter, _ *http.Request) {
	response.JSON(w, http.StatusOK, toBenchmarkDTOs(domain.Benchmarks))
}
