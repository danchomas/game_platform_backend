package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"beeline-games/internal/api"
	"beeline-games/internal/collector"
	"beeline-games/internal/config"
	"beeline-games/internal/llm"
	"beeline-games/internal/store"
)

func main() {
	cfg := config.Load()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	st, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("store", "err", err)
		os.Exit(1)
	}
	if cfg.OpenRouterKey == "" {
		slog.Warn("OPENROUTER_API_KEY не задан — генерация игр работать не будет")
	}
	if cfg.JWTSecret == "dev-secret-change-me" {
		slog.Warn("используется dev JWT_SECRET — задайте свой в окружении")
	}

	col := collector.New(st.DB, 100_000)
	colDone := make(chan struct{})
	go func() { col.Run(ctx); close(colDone) }()

	l := llm.NewClient(cfg.OpenRouterKey, cfg.OpenRouterModel, cfg.OpenRouterURL, cfg.ProxyURL, cfg.LLMTimeout+10*time.Second)
	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           api.New(cfg, st, l, col).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      cfg.LLMTimeout + 30*time.Second, // запрос генерации долгий
		IdleTimeout:       120 * time.Second,
	}
	go func() {
		slog.Info("listening", "addr", cfg.Addr, "model", cfg.OpenRouterModel)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")
	sctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = srv.Shutdown(sctx)
	<-colDone // дописываем буфер аналитики
	_ = st.DB.Close()
}
