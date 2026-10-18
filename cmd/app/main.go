package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"beeline-games/internal/core/collector"
	"beeline-games/internal/core/config"
	"beeline-games/internal/core/llm"
	"beeline-games/internal/core/repository/postgres/pool/pgx"
	"beeline-games/internal/core/transport/http/middleware"

	authRepo "beeline-games/internal/features/auth/repository/postgres"
	authSvc "beeline-games/internal/features/auth/service"
	authHttp "beeline-games/internal/features/auth/transport/http"

	gamesRepo "beeline-games/internal/features/games/repository/postgres"
	gamesSvc "beeline-games/internal/features/games/service"
	gamesHttp "beeline-games/internal/features/games/transport/http"

	analyticsRepo "beeline-games/internal/features/analytics/repository/postgres"
	analyticsSvc "beeline-games/internal/features/analytics/service"
	analyticsHttp "beeline-games/internal/features/analytics/transport/http"

	playSvc "beeline-games/internal/features/play/service"
	playHttp "beeline-games/internal/features/play/transport/http"
)

func main() {
	cfg := config.Load()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := pgx.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	llmClient := llm.NewClient(cfg.OpenRouterKey, cfg.OpenRouterModel, cfg.OpenRouterURL, cfg.ProxyURL, cfg.LLMTimeout+10*time.Second)

	eventCollector := collector.New(pool, 100_000)
	colDone := make(chan struct{})
	go func() {
		eventCollector.Run(ctx)
		close(colDone)
	}()

	aRepo := authRepo.NewRepository(pool)
	gRepo := gamesRepo.NewRepository(pool)
	anRepo := analyticsRepo.NewRepository(pool)

	aSvc := authSvc.NewService(aRepo, cfg.JWTSecret, cfg.ManagerCode)
	anSvc := analyticsSvc.NewService(anRepo)
	gSvc := gamesSvc.NewService(gRepo, llmClient, anSvc, cfg.MaxConcurrentGen, cfg.LLMTimeout)
	pSvc := playSvc.NewService(gSvc, eventCollector, cfg.PublicBaseURL)

	mux := http.ServeMux{}
	authMW := middleware.Auth(cfg.JWTSecret)

	authHandler := authHttp.NewHandler(aSvc)
	authHandler.RegisterRoutes(&mux, authMW)

	gamesHandler := gamesHttp.NewHandler(gSvc, cfg.PublicBaseURL)
	gamesHandler.RegisterRoutes(&mux, authMW)

	analyticsHandler := analyticsHttp.NewHandler(anSvc, gSvc)
	analyticsHandler.RegisterRoutes(&mux, authMW)

	playHandler := playHttp.NewHandler(pSvc)
	playHandler.RegisterRoutes(&mux, authMW)

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Ping(r.Context()); err != nil {
			http.Error(w, "db down", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	srv := &http.Server{
		Addr:			cfg.Addr,
		Handler:		&mux,
		ReadHeaderTimeout:	10 * time.Second,
		ReadTimeout:		30 * time.Second,
		WriteTimeout:		cfg.LLMTimeout + 30*time.Second,
		IdleTimeout:		120 * time.Second,
	}

	go func() {
		slog.Info("server starting", "addr", cfg.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("http server failed", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	_ = srv.Shutdown(shutdownCtx)
	<-colDone
	slog.Info("shutdown complete")
}
