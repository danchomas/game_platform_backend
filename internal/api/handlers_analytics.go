package api

import (
	"net/http"
	"sync"
	"time"

	"beeline-games/internal/auth"
	"beeline-games/internal/store"
)

func dateRange(r *http.Request) (from, to time.Time, days int) {
	days = qInt(r, "days", 30, 1, 365)
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	return today.AddDate(0, 0, -(days - 1)), today.AddDate(0, 0, 1), days
}

// par выполняет запросы параллельно и возвращает первую ошибку.
func par(fns ...func() error) error {
	var wg sync.WaitGroup
	errs := make([]error, len(fns))
	for i, f := range fns {
		wg.Add(1)
		go func() { defer wg.Done(); errs[i] = f() }()
	}
	wg.Wait()
	for _, e := range errs {
		if e != nil {
			return e
		}
	}
	return nil
}

// GET /api/analytics/overview?days=30&scope=mine|all — дашборд менеджера.
func (s *Server) overview(w http.ResponseWriter, r *http.Request, c auth.Claims) {
	from, to, days := dateRange(r)
	f := store.Filter{From: from, To: to}
	scope := "mine"
	if r.URL.Query().Get("scope") == "all" {
		if c.Role != "manager" {
			apiErr(w, 403, "общая аналитика доступна только менеджерам")
			return
		}
		scope = "all"
	} else {
		f.OwnerID = c.Sub
	}
	var (
		sum     store.Summary
		daily   []store.DayPoint
		games   []store.GameRow
		custom  []store.EventCount
		buckets []store.Bucket
	)
	ctx := r.Context()
	err := par(
		func() (e error) { sum, e = s.st.Summary(ctx, f); return },
		func() (e error) { daily, e = s.st.Daily(ctx, f); return },
		func() (e error) { games, e = s.st.PerGame(ctx, f); return },
		func() (e error) { custom, e = s.st.CustomEvents(ctx, f); return },
		func() (e error) { buckets, e = s.st.DurationBuckets(ctx, f); return },
	)
	if err != nil {
		apiErr(w, 500, "ошибка аналитики")
		return
	}
	writeJSON(w, 200, map[string]any{
		"scope": scope, "days": days, "summary": sum, "daily": daily,
		"games": games, "custom_events": custom, "session_duration": buckets,
	})
}

// GET /api/analytics/games/{id}?days=30 — аналитика одной игры (владелец или менеджер).
func (s *Server) gameAnalytics(w http.ResponseWriter, r *http.Request, c auth.Claims) {
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
	if g.OwnerID != c.Sub && c.Role != "manager" {
		apiErr(w, 403, "нет доступа к аналитике этой игры")
		return
	}
	from, to, days := dateRange(r)
	f := store.Filter{From: from, To: to, GameID: id}
	var (
		sum     store.Summary
		daily   []store.DayPoint
		custom  []store.EventCount
		buckets []store.Bucket
	)
	ctx := r.Context()
	err = par(
		func() (e error) { sum, e = s.st.Summary(ctx, f); return },
		func() (e error) { daily, e = s.st.Daily(ctx, f); return },
		func() (e error) { custom, e = s.st.CustomEvents(ctx, f); return },
		func() (e error) { buckets, e = s.st.DurationBuckets(ctx, f); return },
	)
	if err != nil {
		apiErr(w, 500, "ошибка аналитики")
		return
	}
	g.HTML = ""
	writeJSON(w, 200, map[string]any{
		"game": s.dto(g), "days": days, "summary": sum, "daily": daily,
		"custom_events": custom, "session_duration": buckets,
	})
}
