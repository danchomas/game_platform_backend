package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"beeline-games/internal/core/domain"
	coreErrs "beeline-games/internal/core/errors"
	"beeline-games/internal/core/transport/http/request"
	"beeline-games/internal/core/transport/http/response"
)

// Status — для опроса без SSE. Вместе со статусом отдаёт карточку игры (без HTML):
// после фоновой сборки клиенту нужны новые название, версия и план.
func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	userID, _ := request.UserID(r)
	g, err := h.svc.GetGame(r.Context(), userID, r.PathValue("id"))
	if err != nil || g.OwnerID != userID {
		response.Error(w, http.StatusNotFound, "игра не найдена")
		return
	}
	g.HTML = ""
	response.JSON(w, http.StatusOK, struct {
		domain.StatusEvent
		Game gameDTO `json:"game"`
	}{statusOf(g), toDTO(g, h.publicBaseURL)})
}

func statusOf(g domain.Game) domain.StatusEvent {
	return domain.StatusEvent{GameID: g.ID, Status: g.Status, Detail: g.StatusDetail, Version: g.Version, At: g.UpdatedAt}
}

// Events — SSE-поток смен статуса. EventSource не умеет заголовки: токен через ?token=.
func (h *Handler) Events(w http.ResponseWriter, r *http.Request) {
	userID, _ := request.UserID(r)
	g, ch, cancel, err := h.svc.Subscribe(r.Context(), userID, r.PathValue("id"))
	if err != nil {
		handleCRUDErr(w, err)
		return
	}
	defer cancel()

	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Time{}) // поток живёт дольше WriteTimeout сервера
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	send := func(ev domain.StatusEvent) bool {
		b, _ := json.Marshal(ev)
		if _, err := fmt.Fprintf(w, "event: status\ndata: %s\n\n", b); err != nil {
			return false
		}
		return rc.Flush() == nil
	}
	if !send(statusOf(g)) {
		return
	}

	ping := time.NewTicker(20 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case ev := <-ch:
			if !send(ev) {
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
	userID, _ := request.UserID(r)
	gens, err := h.svc.ListGenerations(r.Context(), userID, r.PathValue("id"))
	if err != nil {
		handleCRUDErr(w, err)
		return
	}
	response.JSON(w, http.StatusOK, gens)
}

// Preview — скриншот игры из последнего теста в браузере (для карточки в галерее).
func (h *Handler) Preview(w http.ResponseWriter, r *http.Request) {
	userID, _ := request.UserID(r)
	img, err := h.svc.Screenshot(r.Context(), userID, r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "public, max-age=60")
	_, _ = w.Write(img)
}

func (h *Handler) GenerationStats(w http.ResponseWriter, r *http.Request) {
	role, _ := request.Role(r)
	st, err := h.svc.GenerationStats(r.Context(), role, qInt(r, "days", 30, 1, 365))
	if err != nil {
		if errors.Is(err, coreErrs.ErrForbidden) {
			response.Error(w, http.StatusForbidden, "статистика генераций доступна только менеджерам")
			return
		}
		response.Error(w, http.StatusInternalServerError, "внутренняя ошибка")
		return
	}
	response.JSON(w, http.StatusOK, st)
}

func (h *Handler) Benchmarks(w http.ResponseWriter, r *http.Request) {
	response.JSON(w, http.StatusOK, domain.Benchmarks)
}
