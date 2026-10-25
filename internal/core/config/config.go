package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"beeline-games/internal/core/llm"
)

const (
	defaultAddr             = ":8080"
	defaultModel            = "anthropic/claude-haiku-5.5"
	defaultPlannerModel     = "deepseek/deepseek-v4.1-flash"
	defaultPublicBaseURL    = "http://localhost:8080"
	defaultMaxConcurrentGen = 8
	defaultLLMTimeoutSec    = 240
	defaultLLMMaxTokens     = 32000
	defaultMaxFixRounds     = 2
	// low: на рассуждения уходит ~20% max_tokens, остальное — на ответ с кодом игры
	defaultReasoning = "low"

	defaultBrowserTimeoutSec  = 40
	defaultBrowserMaxParallel = 2
)

type Config struct {
	Addr        string
	DatabaseURL string
	JWTSecret   string

	OpenRouterBaseURL string
	OpenRouterKey     string
	// OpenRouterModel — модель-кодер (пишет и правит игру).
	OpenRouterModel string

	// Модели по ролям агента: планировщик дешевле кодера; исправитель по умолчанию = кодер.
	PlannerModel   string
	FixerModel     string
	FallbackModels []string
	LLMMaxTokens   int
	MaxFixRounds   int
	// Reasoning — бюджет рассуждений «думающих» моделей: off, low, medium, high.
	Reasoning string

	// BrowserWSURL — DevTools headless Chrome для теста игр; пусто — только статическая проверка.
	BrowserWSURL       string
	BrowserTimeout     time.Duration
	BrowserMaxParallel int

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

		OpenRouterBaseURL: env("OPENROUTER_BASE_URL", llm.DefaultBaseURL),
		OpenRouterKey:     env("OPENROUTER_API_KEY", ""),
		OpenRouterModel:   env("OPENROUTER_MODEL", defaultModel),

		PlannerModel:   env("LLM_PLANNER_MODEL", defaultPlannerModel),
		FallbackModels: envList("LLM_FALLBACK_MODELS"),
		LLMMaxTokens:   envInt("LLM_MAX_TOKENS", defaultLLMMaxTokens),
		MaxFixRounds:   envInt("MAX_FIX_ROUNDS", defaultMaxFixRounds),
		Reasoning:      strings.ToLower(env("LLM_REASONING", defaultReasoning)),

		BrowserWSURL:       env("BROWSER_WS_URL", ""),
		BrowserTimeout:     time.Duration(envInt("BROWSER_TEST_TIMEOUT_SEC", defaultBrowserTimeoutSec)) * time.Second,
		BrowserMaxParallel: envInt("BROWSER_MAX_PARALLEL", defaultBrowserMaxParallel),

		PublicBaseURL:    strings.TrimRight(env("PUBLIC_BASE_URL", defaultPublicBaseURL), "/"),
		ManagerCode:      env("MANAGER_CODE", ""),
		MaxConcurrentGen: envInt("MAX_CONCURRENT_GEN", defaultMaxConcurrentGen),
		LLMTimeout:       time.Duration(envInt("LLM_TIMEOUT_SEC", defaultLLMTimeoutSec)) * time.Second,
	}

	cfg.FixerModel = env("LLM_FIXER_MODEL", cfg.OpenRouterModel)

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
	if c.LLMMaxTokens < 1 {
		return fmt.Errorf("LLM_MAX_TOKENS должен быть положительным, получено %d", c.LLMMaxTokens)
	}
	switch c.Reasoning {
	case "off", "low", "medium", "high":
	default:
		return fmt.Errorf("LLM_REASONING: ожидается off, low, medium или high, получено %q", c.Reasoning)
	}
	if c.MaxFixRounds < 0 {
		return fmt.Errorf("MAX_FIX_ROUNDS не может быть отрицательным, получено %d", c.MaxFixRounds)
	}
	if c.BrowserTimeout <= 0 || c.BrowserMaxParallel < 1 {
		return fmt.Errorf("BROWSER_TEST_TIMEOUT_SEC и BROWSER_MAX_PARALLEL должны быть положительными")
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

func envList(key string) []string {
	var out []string
	for _, part := range strings.Split(os.Getenv(key), ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}
