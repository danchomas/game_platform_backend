package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"beeline-games/internal/auth"
	"beeline-games/internal/llm"
	"beeline-games/internal/store"
)

type gameDTO struct {
	store.Game
	PlayURL string `json:"play_url"`
}

func (s *Server) dto(g store.Game) gameDTO {
	return gameDTO{Game: g, PlayURL: s.cfg.PublicBaseURL + "/g/" + g.ID}
}

func (s *Server) dtos(gs []store.Game) []gameDTO {
	out := make([]gameDTO, len(gs))
	for i, g := range gs {
		out[i] = s.dto(g)
	}
	return out
}

// owned загружает игру и проверяет, что она принадлежит пользователю.
func (s *Server) owned(w http.ResponseWriter, r *http.Request, c auth.Claims) (store.Game, bool) {
	id := r.PathValue("id")
	if !store.IsUUID(id) {
		apiErr(w, 404, "игра не найдена")
		return store.Game{}, false
	}
	g, err := s.st.GetGame(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		apiErr(w, 404, "игра не найдена")
		return g, false
	}
	if err != nil {
		apiErr(w, 500, "ошибка базы данных")
		return g, false
	}
	if g.OwnerID != c.Sub {
		apiErr(w, 403, "это не ваша игра")
		return g, false
	}
	return g, true
}

// ---------- генерация ----------

var errBusy = errors.New("сервер занят генерациями, попробуйте через минуту")

func (s *Server) generate(ctx context.Context, g store.Game, userText string) (int, error) {
	select {
	case s.genSem <- struct{}{}:
		defer func() { <-s.genSem }()
	case <-time.After(20 * time.Second):
		return 0, errBusy
	case <-ctx.Done():
		return 0, ctx.Err()
	}

	hist, err := s.st.Messages(ctx, g.ID)
	if err != nil {
		return 0, err
	}
	h := make([]llm.Msg, 0, len(hist))
	for _, m := range hist {
		h = append(h, llm.Msg{Role: m.Role, Content: m.Content})
	}
	msgs := llm.BuildMessages(s.insightsHint(ctx), h, g.HTML, userText)

	var res llm.Result
	for attempt := 0; attempt < 2; attempt++ { // один повтор, если формат ответа поломан
		lctx, cancel := context.WithTimeout(ctx, s.cfg.LLMTimeout)
		raw, err := s.llm.Chat(lctx, msgs)
		cancel()
		if err != nil {
			return 0, err
		}
		if res, err = llm.Parse(raw); err == nil {
			break
		} else if attempt == 1 {
			return 0, err
		}
	}
	return s.st.AddVersion(ctx, store.VersionInput{
		GameID: g.ID, UserText: userText, Reply: res.Reply, HTML: res.HTML,
		Title: res.Title, Genre: res.Genre, Audience: res.Audience,
	})
}

func genErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, llm.ErrNoKey), errors.Is(err, errBusy):
		apiErr(w, 503, err.Error())
	case errors.Is(err, context.Canceled):
		apiErr(w, 499, "запрос отменён")
	default:
		apiErr(w, 502, "ИИ не смог сгенерировать игру: "+err.Error())
	}
}

func validText(w http.ResponseWriter, s string, field string) (string, bool) {
	s = strings.TrimSpace(s)
	if n := utf8.RuneCountInString(s); n < 3 || n > 4000 {
		apiErr(w, 400, field+": от 3 до 4000 символов")
		return "", false
	}
	return s, true
}

// POST /api/games {prompt} — создаёт игру и сразу генерирует v1.
func (s *Server) createGame(w http.ResponseWriter, r *http.Request, c auth.Claims) {
	var in struct {
		Prompt string `json:"prompt"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	prompt, ok := validText(w, in.Prompt, "prompt")
	if !ok {
		return
	}
	id, err := s.st.CreateGame(r.Context(), c.Sub, prompt)
	if err != nil {
		apiErr(w, 500, "ошибка базы данных")
		return
	}
	s.inflight.Store(id, true)
	defer s.inflight.Delete(id)

	v, err := s.generate(r.Context(), store.Game{ID: id}, prompt)
	if err != nil {
		_ = s.st.DeleteGame(context.WithoutCancel(r.Context()), id) // не копим пустые черновики
		genErr(w, err)
		return
	}
	s.respondGame(w, 201, id, v)
}

// POST /api/games/{id}/messages {content} — итеративная доработка в чате.
func (s *Server) iterate(w http.ResponseWriter, r *http.Request, c auth.Claims) {
	g, ok := s.owned(w, r, c)
	if !ok {
		return
	}
	var in struct {
		Content string `json:"content"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	text, ok := validText(w, in.Content, "content")
	if !ok {
		return
	}
	if _, busy := s.inflight.LoadOrStore(g.ID, true); busy {
		apiErr(w, 409, "для этой игры уже идёт генерация")
		return
	}
	defer s.inflight.Delete(g.ID)

	v, err := s.generate(r.Context(), g, text)
	if err != nil {
		genErr(w, err)
		return
	}
	s.respondGame(w, 200, g.ID, v)
}

func (s *Server) respondGame(w http.ResponseWriter, code int, id string, version int) {
	g, err := s.st.GetGame(context.Background(), id)
	if err != nil {
		apiErr(w, 500, "ошибка базы данных")
		return
	}
	msgs, _ := s.st.Messages(context.Background(), id)
	reply := ""
	if n := len(msgs); n > 0 {
		reply = msgs[n-1].Content
	}
	writeJSON(w, code, map[string]any{"game": s.dto(g), "reply": reply, "version": version})
}

// ---------- CRUD ----------

func (s *Server) myGames(w http.ResponseWriter, r *http.Request, c auth.Claims) {
	gs, err := s.st.ListOwnerGames(r.Context(), c.Sub, qInt(r, "limit", 50, 1, 200), qInt(r, "offset", 0, 0, 1<<20))
	if err != nil {
		apiErr(w, 500, "ошибка базы данных")
		return
	}
	writeJSON(w, 200, s.dtos(gs))
}

func (s *Server) gallery(w http.ResponseWriter, r *http.Request) {
	gs, err := s.st.ListPublished(r.Context(), r.URL.Query().Get("sort"), strings.TrimSpace(r.URL.Query().Get("q")),
		qInt(r, "limit", 24, 1, 100), qInt(r, "offset", 0, 0, 1<<20))
	if err != nil {
		apiErr(w, 500, "ошибка базы данных")
		return
	}
	writeJSON(w, 200, s.dtos(gs))
}

// GET /api/games/{id}: опубликованные — всем, черновики — только владельцу.
func (s *Server) getGame(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !store.IsUUID(id) {
		apiErr(w, 404, "игра не найдена")
		return
	}
	g, err := s.st.GetGame(r.Context(), id)
	if err != nil {
		apiErr(w, 404, "игра не найдена")
		return
	}
	if !g.Published {
		if c, ok := s.claims(r, false); !ok || c.Sub != g.OwnerID {
			apiErr(w, 404, "игра не найдена")
			return
		}
	}
	writeJSON(w, 200, s.dto(g))
}

func (s *Server) patchGame(w http.ResponseWriter, r *http.Request, c auth.Claims) {
	g, ok := s.owned(w, r, c)
	if !ok {
		return
	}
	var in struct {
		Title string `json:"title"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	t := strings.TrimSpace(in.Title)
	if n := utf8.RuneCountInString(t); n < 1 || n > 80 {
		apiErr(w, 400, "title: от 1 до 80 символов")
		return
	}
	if err := s.st.SetTitle(r.Context(), g.ID, t); err != nil {
		apiErr(w, 500, "ошибка базы данных")
		return
	}
	g.Title = t
	writeJSON(w, 200, s.dto(g))
}

func (s *Server) deleteGame(w http.ResponseWriter, r *http.Request, c auth.Claims) {
	g, ok := s.owned(w, r, c)
	if !ok {
		return
	}
	if err := s.st.DeleteGame(r.Context(), g.ID); err != nil {
		apiErr(w, 500, "ошибка базы данных")
		return
	}
	s.setPub(g.ID, false)
	w.WriteHeader(204)
}

func (s *Server) messages(w http.ResponseWriter, r *http.Request, c auth.Claims) {
	g, ok := s.owned(w, r, c)
	if !ok {
		return
	}
	ms, err := s.st.Messages(r.Context(), g.ID)
	if err != nil {
		apiErr(w, 500, "ошибка базы данных")
		return
	}
	writeJSON(w, 200, ms)
}

func (s *Server) versions(w http.ResponseWriter, r *http.Request, c auth.Claims) {
	g, ok := s.owned(w, r, c)
	if !ok {
		return
	}
	vs, err := s.st.Versions(r.Context(), g.ID)
	if err != nil {
		apiErr(w, 500, "ошибка базы данных")
		return
	}
	writeJSON(w, 200, vs)
}

func (s *Server) restore(w http.ResponseWriter, r *http.Request, c auth.Claims) {
	g, ok := s.owned(w, r, c)
	if !ok {
		return
	}
	n, err := strconv.Atoi(r.PathValue("v"))
	if err != nil {
		apiErr(w, 400, "некорректная версия")
		return
	}
	html, err := s.st.VersionHTML(r.Context(), g.ID, n)
	if err != nil {
		apiErr(w, 404, "версия не найдена")
		return
	}
	v, err := s.st.AddVersion(r.Context(), store.VersionInput{
		GameID: g.ID, Reply: fmt.Sprintf("Восстановлена версия %d.", n), HTML: html})
	if err != nil {
		apiErr(w, 500, "ошибка базы данных")
		return
	}
	s.respondGame(w, 200, g.ID, v)
}

func (s *Server) publish(on bool) authedFn {
	return func(w http.ResponseWriter, r *http.Request, c auth.Claims) {
		g, ok := s.owned(w, r, c)
		if !ok {
			return
		}
		if on && g.Version == 0 {
			apiErr(w, 400, "в игре ещё нет сгенерированного кода")
			return
		}
		if err := s.st.SetPublished(r.Context(), g.ID, on); err != nil {
			apiErr(w, 500, "ошибка базы данных")
			return
		}
		s.setPub(g.ID, on)
		g.Published = on
		writeJSON(w, 200, s.dto(g))
	}
}

// ---------- инсайты для промпта ----------

func (s *Server) insightsHint(ctx context.Context) string {
	s.hintMu.Lock()
	defer s.hintMu.Unlock()
	if time.Now().Before(s.hintExp) {
		return s.hint
	}
	s.hintExp = time.Now().Add(5 * time.Minute)
	ins, err := s.st.TopInsights(ctx, 3)
	if err != nil || len(ins) == 0 {
		s.hint = ""
		return ""
	}
	var b strings.Builder
	for _, in := range ins {
		fmt.Fprintf(&b, "- \"%s\" (genre: %s, audience: %s): %d sessions, avg session %.0fs, win rate %.0f%%\n",
			in.Title, in.Genre, in.Audience, in.Sessions, in.AvgSessionSec, in.WinRate*100)
	}
	s.hint = b.String()
	return s.hint
}
