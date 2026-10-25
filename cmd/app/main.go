package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"beeline-games/internal/core/agent"
	"beeline-games/internal/core/agent/browser"
	"beeline-games/internal/core/collector"
	"beeline-games/internal/core/config"
	"beeline-games/internal/core/llm"
	"beeline-games/internal/core/repository/postgres/pool/pgx"
	"beeline-games/internal/core/transport/http/middleware"

	authRepo "beeline-games/internal/features/auth/repository/postgres"
	authSvc "beeline-games/internal/features/auth/service"
	authHTTP "beeline-games/internal/features/auth/transport/http"

	gamesRepo "beeline-games/internal/features/games/repository/postgres"
	gamesSvc "beeline-games/internal/features/games/service"
	gamesHTTP "beeline-games/internal/features/games/transport/http"

	analyticsRepo "beeline-games/internal/features/analytics/repository/postgres"
	analyticsSvc "beeline-games/internal/features/analytics/service"
	analyticsHTTP "beeline-games/internal/features/analytics/transport/http"

	playSvc "beeline-games/internal/features/play/service"
	playHTTP "beeline-games/internal/features/play/transport/http"
)

const eventBuffer = 100_000

const (
	readHeaderTimeout = 10 * time.Second
	readTimeout       = 30 * time.Second
	writeTimeoutSlack = 30 * time.Second
	idleTimeout       = 120 * time.Second
	shutdownTimeout   = 15 * time.Second

	llmClientSlack = 10 * time.Second
)

func main() {
	if err := run(); err != nil {
		slog.Error("сервер остановлен с ошибкой", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := pgx.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	eventCollector := collector.New(pool, eventBuffer)
	collectorDone := make(chan struct{})
	go func() {
		defer close(collectorDone)
		eventCollector.Run(ctx)
	}()

	model := llm.NewClient(cfg.OpenRouterKey, cfg.OpenRouterModel, cfg.OpenRouterBaseURL,
		cfg.LLMTimeout+llmClientSlack)

	slog.Info("модель настроена",
		"base_url", model.BaseURL(),
		"model", model.Model(),
		"key_set", cfg.LLMConfigured())
	if !cfg.LLMConfigured() {
		slog.Warn("OPENROUTER_API_KEY не задан: генерация игр будет отвечать «модель недоступна»")
	}

	var tester agent.Tester
	if cfg.BrowserWSURL != "" {
		tester = browser.New(cfg.BrowserWSURL, cfg.BrowserTimeout, cfg.BrowserMaxParallel)
	} else {
		slog.Warn("BROWSER_WS_URL не задан: игры проверяются только статически, без запуска в браузере")
	}
	gameAgent := agent.New(model, tester, agent.Config{
		Models: agent.Models{
			Planner:   cfg.PlannerModel,
			Coder:     cfg.OpenRouterModel,
			Fixer:     cfg.FixerModel,
			Fallbacks: cfg.FallbackModels,
			MaxTokens: cfg.LLMMaxTokens,
		},
		MaxFixRounds: cfg.MaxFixRounds,
		CallTimeout:  cfg.LLMTimeout,
	})
	slog.Info("агент настроен",
		"planner", cfg.PlannerModel, "coder", cfg.OpenRouterModel, "fixer", cfg.FixerModel,
		"fix_rounds", cfg.MaxFixRounds, "browser_test", tester != nil)

	users := authRepo.NewRepository(pool)
	games := gamesRepo.NewRepository(pool)
	stats := analyticsRepo.NewRepository(pool)

	auth := authSvc.NewService(users, cfg.JWTSecret, cfg.ManagerCode)
	gameCatalog := gamesSvc.NewService(ctx, games, gameAgent, nil, cfg.MaxConcurrentGen)
	if err := gameCatalog.RecoverInterrupted(ctx); err != nil {
		slog.Error("прерванные генерации не сброшены", "err", err)
	}
	analytics := analyticsSvc.NewService(stats, gameCatalog)
	play := playSvc.NewService(gameCatalog, eventCollector, cfg.PublicBaseURL)

	gameCatalog.SetInsights(analytics)

	mux := http.NewServeMux()
	authMW := middleware.Auth(cfg.JWTSecret)
	optionalAuthMW := middleware.OptionalAuth(cfg.JWTSecret)

	authHTTP.NewHandler(auth).RegisterRoutes(mux, authMW)
	gamesHTTP.NewHandler(gameCatalog, cfg.PublicBaseURL).RegisterRoutes(mux, authMW, optionalAuthMW)
	analyticsHTTP.NewHandler(analytics).RegisterRoutes(mux, authMW)
	playHTTP.NewHandler(play).RegisterRoutes(mux, authMW, optionalAuthMW)

	mux.HandleFunc("GET /healthz", healthz(pool))

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           mux,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      cfg.LLMTimeout + writeTimeoutSlack,
		IdleTimeout:       idleTimeout,
	}

	serverErr := make(chan error, 1)
	go func() {
		slog.Info("сервер запускается", "addr", cfg.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		return err
	case <-ctx.Done():
		slog.Info("получен сигнал, останавливаемся")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("сервер не остановился мягко", "err", err)
	}
	// фоновые сборки отменены вместе с ctx: дожидаемся, пока они запишут итоговый статус
	gameCatalog.Wait()

	<-collectorDone
	slog.Info("остановка завершена")
	return nil
}

func healthz(pool interface {
	Ping(context.Context) error
},
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Ping(r.Context()); err != nil {
			http.Error(w, `{"status":"db down"}`, http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}
}
