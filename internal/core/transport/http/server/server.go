package api

import (
	"compress/gzip"
	"context"
	_ "embed"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"beeline-games/internal/auth"
	"beeline-games/internal/collector"
	"beeline-games/internal/config"
	"beeline-games/internal/llm"
	"beeline-games/internal/store"
)

var sdkJS string

type Server struct {
	cfg	config.Config
	st	*store.Store
	llm	*llm.Client
	col	*collector.Collector
	genSem	chan struct{}

	inflight	sync.Map

	pubMu		sync.RWMutex
	pubCache	map[string]pubEntry

	hintMu	sync.Mutex
	hint	string
	hintExp	time.Time
}

type pubEntry struct {
	ok	bool
	exp	time.Time
}

func New(cfg config.Config, st *store.Store, l *llm.Client, col *collector.Collector) *Server {
	return &Server{cfg: cfg, st: st, llm: l, col: col,
		genSem:	make(chan struct{}, cfg.MaxConcurrentGen), pubCache: map[string]pubEntry{}}
}

func (s *Server) Handler() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("GET /healthz", s.health)

	m.HandleFunc("POST /api/auth/register", s.register)
	m.HandleFunc("POST /api/auth/login", s.login)
	m.HandleFunc("GET /api/me", s.authed(s.me))

	m.HandleFunc("POST /api/games", s.authed(s.createGame))
	m.HandleFunc("GET /api/games/mine", s.authed(s.myGames))
	m.HandleFunc("GET /api/games/{id}", s.getGame)
	m.HandleFunc("PATCH /api/games/{id}", s.authed(s.patchGame))
	m.HandleFunc("DELETE /api/games/{id}", s.authed(s.deleteGame))
	m.HandleFunc("GET /api/games/{id}/messages", s.authed(s.messages))
	m.HandleFunc("POST /api/games/{id}/messages", s.authed(s.iterate))
	m.HandleFunc("GET /api/games/{id}/versions", s.authed(s.versions))
	m.HandleFunc("POST /api/games/{id}/versions/{v}/restore", s.authed(s.restore))
	m.HandleFunc("POST /api/games/{id}/publish", s.authed(s.publish(true)))
	m.HandleFunc("POST /api/games/{id}/unpublish", s.authed(s.publish(false)))
	m.HandleFunc("GET /api/games/{id}/export", s.authed(s.export))
	m.HandleFunc("GET /api/gallery", s.gallery)

	m.HandleFunc("GET /g/{id}", s.play)
	m.HandleFunc("POST /api/events", s.events)

	m.HandleFunc("GET /api/analytics/overview", s.authed(s.overview))
	m.HandleFunc("GET /api/analytics/games/{id}", s.authed(s.gameAnalytics))

	m.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) { apiErr(w, 404, "не найдено") })
	if fi, err := os.Stat(filepath.Join(s.cfg.WebDir, "index.html")); err == nil && !fi.IsDir() {
		m.HandleFunc("GET /{$}", s.static)
	}
	return recoverMW(logMW(corsMW(gzipMW(m))))
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func apiErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		apiErr(w, 400, "некорректный JSON")
		return false
	}
	return true
}

func qInt(r *http.Request, key string, def, min, max int) int {
	n, err := strconv.Atoi(r.URL.Query().Get(key))
	if err != nil {
		return def
	}
	if n < min {
		return min
	}
	if n > max {
		return max
	}
	return n
}

func (s *Server) claims(r *http.Request, allowQuery bool) (auth.Claims, bool) {
	tok := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if tok == "" && allowQuery {
		tok = r.URL.Query().Get("token")
	}
	if tok == "" {
		return auth.Claims{}, false
	}
	c, err := auth.Parse(s.cfg.JWTSecret, tok)
	return c, err == nil
}

type authedFn func(w http.ResponseWriter, r *http.Request, c auth.Claims)

func (s *Server) authed(h authedFn) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, ok := s.claims(r, false)
		if !ok {
			apiErr(w, 401, "нужна авторизация")
			return
		}
		h(w, r, c)
	}
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.st.DB.PingContext(ctx); err != nil {
		apiErr(w, 503, "db down")
		return
	}
	writeJSON(w, 200, map[string]any{"status": "ok", "events_dropped": s.col.Dropped.Load()})
}

func (s *Server) static(w http.ResponseWriter, r *http.Request) {
	p := filepath.Join(s.cfg.WebDir, filepath.Clean("/"+r.URL.Path))
	if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
		http.ServeFile(w, r, p)
		return
	}
	http.ServeFile(w, r, filepath.Join(s.cfg.WebDir, "index.html"))
}

type statusWriter struct {
	http.ResponseWriter
	code	int
}

func (w *statusWriter) WriteHeader(c int)	{ w.code = c; w.ResponseWriter.WriteHeader(c) }

func recoverMW(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				slog.Error("panic", "err", v, "path", r.URL.Path)
				apiErr(w, 500, "внутренняя ошибка")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func logMW(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}
		t := time.Now()
		sw := &statusWriter{ResponseWriter: w, code: 200}
		next.ServeHTTP(sw, r)
		slog.Info("http", "m", r.Method, "path", r.URL.Path, "status", sw.code, "ms", time.Since(t).Milliseconds())
	})
}

func corsMW(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Access-Control-Allow-Origin", "*")
		h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		h.Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		h.Set("Access-Control-Max-Age", "86400")
		if r.Method == http.MethodOptions {
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r)
	})
}

var gzPool = sync.Pool{New: func() any { return gzip.NewWriter(nil) }}

type gzWriter struct {
	http.ResponseWriter
	zw		*gzip.Writer
	wroteHeader	bool
}

func (g *gzWriter) WriteHeader(code int) {
	if g.wroteHeader {
		return
	}
	g.wroteHeader = true
	h := g.Header()
	ct := h.Get("Content-Type")
	if code != 204 && code != 304 && h.Get("Content-Encoding") == "" &&
		(strings.HasPrefix(ct, "text/") || strings.HasPrefix(ct, "application/json") || strings.HasPrefix(ct, "application/javascript")) {
		h.Del("Content-Length")
		h.Set("Content-Encoding", "gzip")
		h.Add("Vary", "Accept-Encoding")
		g.zw = gzPool.Get().(*gzip.Writer)
		g.zw.Reset(g.ResponseWriter)
	}
	g.ResponseWriter.WriteHeader(code)
}

func (g *gzWriter) Write(b []byte) (int, error) {
	if !g.wroteHeader {
		if g.Header().Get("Content-Type") == "" {
			g.Header().Set("Content-Type", http.DetectContentType(b))
		}
		g.WriteHeader(200)
	}
	if g.zw != nil {
		return g.zw.Write(b)
	}
	return g.ResponseWriter.Write(b)
}

func (g *gzWriter) close() {
	if g.zw != nil {
		_ = g.zw.Close()
		gzPool.Put(g.zw)
	}
}

func gzipMW(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.Header.Get("Range") != "" || !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}
		gw := &gzWriter{ResponseWriter: w}
		defer gw.close()
		next.ServeHTTP(gw, r)
	})
}
