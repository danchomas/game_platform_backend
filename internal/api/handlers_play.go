package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"beeline-games/internal/auth"
	"beeline-games/internal/collector"
	"beeline-games/internal/store"
)

var headRe = regexp.MustCompile(`(?i)<head(\s[^>]*)?>`)

// injectSDK вставляет аналитику до кода игры. Хранится в БД «чистый» html, SDK добавляется при отдаче.
func (s *Server) injectSDK(html, id string, enabled bool) string {
	js := strings.NewReplacer(
		"__GAME_ID__", id,
		"__ENDPOINT__", s.cfg.PublicBaseURL+"/api/events",
		"__ENABLED__", strconv.FormatBool(enabled),
	).Replace(sdkJS)
	tag := "<script>" + js + "</script>"
	if loc := headRe.FindStringIndex(html); loc != nil {
		return html[:loc[1]] + tag + html[loc[1]:]
	}
	if i := strings.Index(strings.ToLower(html), "<body"); i >= 0 {
		return html[:i] + tag + html[i:]
	}
	return tag + html
}

// GET /g/{id} — self-contained HTML для WebView. ?uid=... — идентификатор пользователя приложения (для аналитики).
// Черновики доступны только владельцу (?token=JWT — чтобы работало в iframe-превью).
func (s *Server) play(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !store.IsUUID(id) {
		http.NotFound(w, r)
		return
	}
	g, err := s.st.GetGame(r.Context(), id)
	if err != nil || g.HTML == "" {
		http.NotFound(w, r)
		return
	}
	if !g.Published {
		if c, ok := s.claims(r, true); !ok || c.Sub != g.OwnerID {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
	} else {
		w.Header().Set("Cache-Control", "public, max-age=30")
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.WriteString(w, s.injectSDK(g.HTML, g.ID, g.Published))
}

// GET /api/games/{id}/export — скачать готовый файл (с аналитикой, если игра опубликована).
func (s *Server) export(w http.ResponseWriter, r *http.Request, c auth.Claims) {
	g, ok := s.owned(w, r, c)
	if !ok {
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="game-`+g.ID+`.html"`)
	_, _ = io.WriteString(w, s.injectSDK(g.HTML, g.ID, g.Published))
}

// ---------- приём событий ----------

type eventIn struct {
	Type       string          `json:"type"`
	Result     string          `json:"result"`
	Score      *float64        `json:"score"`
	DurationMs *int64          `json:"duration_ms"`
	Payload    json.RawMessage `json:"payload"`
}

type batchIn struct {
	GameID    string    `json:"game_id"`
	SessionID string    `json:"session_id"`
	PlayerID  string    `json:"player_id"`
	Events    []eventIn `json:"events"`
}

var typeRe = regexp.MustCompile(`^[a-z0-9_]{1,40}$`)

// POST /api/events — публичный, принимает text/plain (sendBeacon без preflight). Отвечает 204 сразу.
func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<10))
	if err != nil {
		apiErr(w, 413, "слишком большой запрос")
		return
	}
	var in batchIn
	if json.Unmarshal(body, &in) != nil || !store.IsUUID(in.GameID) ||
		len(in.SessionID) == 0 || len(in.SessionID) > 64 || len(in.Events) == 0 || len(in.Events) > 50 {
		apiErr(w, 400, "некорректный пакет событий")
		return
	}
	if !s.isPublished(in.GameID) {
		w.WriteHeader(204) // превью/черновики и неизвестные игры молча игнорируем
		return
	}
	if len(in.PlayerID) > 64 {
		in.PlayerID = in.PlayerID[:64]
	}
	for _, e := range in.Events {
		if !typeRe.MatchString(e.Type) {
			continue
		}
		ev := collector.Event{GameID: in.GameID, SessionID: in.SessionID, PlayerID: in.PlayerID,
			Type: e.Type, Score: e.Score, DurationMs: e.DurationMs}
		switch e.Result {
		case "win", "lose", "quit":
			ev.Result = e.Result
		}
		if len(e.Payload) > 0 && len(e.Payload) <= 2048 && string(e.Payload) != "null" {
			ev.Payload = e.Payload
		}
		s.col.Add(ev)
	}
	w.WriteHeader(204)
}

// кэш «опубликована ли игра», чтобы не ходить в БД на каждый пакет событий
func (s *Server) isPublished(id string) bool {
	now := time.Now()
	s.pubMu.RLock()
	e, ok := s.pubCache[id]
	s.pubMu.RUnlock()
	if ok && now.Before(e.exp) {
		return e.ok
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	p, err := s.st.IsPublished(ctx, id)
	if err != nil {
		return e.ok // при сбое БД используем последнее известное значение
	}
	s.setPub(id, p)
	return p
}

func (s *Server) setPub(id string, p bool) {
	s.pubMu.Lock()
	if len(s.pubCache) > 50000 {
		s.pubCache = map[string]pubEntry{}
	}
	s.pubCache[id] = pubEntry{ok: p, exp: time.Now().Add(30 * time.Second)}
	s.pubMu.Unlock()
}
