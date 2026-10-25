package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultAddr             = ":8080"
	defaultModel            = "anthropic/claude-sonnet-4.5"
	defaultPublicBaseURL    = "http://localhost:8080"
	defaultMaxConcurrentGen = 8
	defaultLLMTimeoutSec    = 180
)

type Config struct {
	Addr        string
	DatabaseURL string
	JWTSecret   string

	OpenRouterURL   string
	OpenRouterKey   string
	OpenRouterModel string
	ProxyURL        string

	PublicBaseURL    string
	ManagerCode      string
	MaxConcurrentGen int
	LLMTimeout       time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		Addr:        env("ADDR", defaultAddr),
		DatabaseURL: env("DATABASE_URL", ""),
		JWTSecret:   env("JWT_SECRET", ""),

		OpenRouterURL:   env("OPENROUTER_URL", ""),
		OpenRouterKey:   env("OPENROUTER_API_KEY", ""),
		OpenRouterModel: env("OPENROUTER_MODEL", defaultModel),
		ProxyURL:        env("PROXY_URL", ""),

		PublicBaseURL:    strings.TrimRight(env("PUBLIC_BASE_URL", defaultPublicBaseURL), "/"),
		ManagerCode:      env("MANAGER_CODE", ""),
		MaxConcurrentGen: envInt("MAX_CONCURRENT_GEN", defaultMaxConcurrentGen),
		LLMTimeout:       time.Duration(envInt("LLM_TIMEOUT_SEC", defaultLLMTimeoutSec)) * time.Second,
	}

	return cfg, cfg.Validate()
}

func (c Config) Validate() error {
	var missing []string

	if c.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}
	if c.JWTSecret == "" {
		missing = append(missing, "JWT_SECRET")
	}

	if len(missing) > 0 {
		return fmt.Errorf("не заданы обязательные переменные окружения: %s", strings.Join(missing, ", "))
	}

	if c.MaxConcurrentGen < 1 {
		return fmt.Errorf("MAX_CONCURRENT_GEN должен быть положительным, получено %d", c.MaxConcurrentGen)
	}
	if c.LLMTimeout <= 0 {
		return fmt.Errorf("LLM_TIMEOUT_SEC должен быть положительным, получено %s", c.LLMTimeout)
	}

	return nil
}

func (c Config) LLMConfigured() bool { return c.OpenRouterKey != "" }

func env(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
