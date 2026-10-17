package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Addr             string
	DatabaseURL      string
	JWTSecret        string
	OpenRouterKey    string
	OpenRouterModel  string
	OpenRouterURL    string
	PublicBaseURL    string // используется в ссылках для шаринга и в SDK
	WebDir           string // каталог со сборкой фронтенда (опционально)
	ManagerCode      string // если задан — регистрация менеджера требует этот код
	MaxConcurrentGen int
	LLMTimeout       time.Duration
	ProxyURL         string
}

func Load() Config {
	return Config{
		Addr:             env("ADDR", ":8080"),
		DatabaseURL:      env("DATABASE_URL", "postgres://beeline:beeline@localhost:5432/beeline?sslmode=disable"),
		JWTSecret:        env("JWT_SECRET", "dev-secret-change-me"),
		OpenRouterKey:    env("OPENROUTER_API_KEY", ""),
		OpenRouterModel:  env("OPENROUTER_MODEL", "anthropic/claude-sonnet-4.5"),
		OpenRouterURL:    env("OPENROUTER_URL", "https://openrouter.ai/api/v1/chat/completions"),
		PublicBaseURL:    env("PUBLIC_BASE_URL", "http://localhost:8080"),
		WebDir:           env("WEB_DIR", "./web"),
		ManagerCode:      env("MANAGER_CODE", ""),
		MaxConcurrentGen: envInt("MAX_CONCURRENT_GEN", 8),
		LLMTimeout:       time.Duration(envInt("LLM_TIMEOUT_SEC", 180)) * time.Second,
		ProxyURL:         env("PROXY_URL", ""),
	}
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func envInt(k string, def int) int {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
