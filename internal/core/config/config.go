package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Addr			string
	DatabaseURL		string
	JWTSecret		string
	OpenRouterKey		string
	OpenRouterModel		string
	OpenRouterURL		string
	PublicBaseURL		string
	WebDir			string
	ManagerCode		string
	MaxConcurrentGen	int
	LLMTimeout		time.Duration
	ProxyURL		string

	PlannerModel	string
	CoderModel	string
	FixerModel	string
	FallbackModels	[]string
	LLMMaxTokens	int
	MaxFixRounds	int

	BrowserWSURL		string
	BrowserTimeout		time.Duration
	BrowserMaxParallel	int
}

func Load() Config {
	coder := env("LLM_CODER_MODEL", env("OPENROUTER_MODEL", "anthropic/claude-haiku-5.5"))
	return Config{
		Addr:			env("ADDR", ":8080"),
		DatabaseURL:		env("DATABASE_URL", ""),
		JWTSecret:		env("JWT_SECRET", ""),
		OpenRouterKey:		env("OPENROUTER_API_KEY", ""),
		OpenRouterModel:	coder,
		OpenRouterURL:		env("OPENROUTER_URL", ""),
		PublicBaseURL:		env("PUBLIC_BASE_URL", ""),
		WebDir:			env("WEB_DIR", "./web"),
		ManagerCode:		env("MANAGER_CODE", ""),
		MaxConcurrentGen:	envInt("MAX_CONCURRENT_GEN", 8),
		LLMTimeout:		time.Duration(envInt("LLM_TIMEOUT_SEC", 240)) * time.Second,
		ProxyURL:		env("PROXY_URL", ""),

		PlannerModel:	env("LLM_PLANNER_MODEL", "deepseek/deepseek-v4.1-flash"),
		CoderModel:	coder,
		FixerModel:	env("LLM_FIXER_MODEL", coder),
		FallbackModels:	envList("LLM_FALLBACK_MODELS"),
		LLMMaxTokens:	envInt("LLM_MAX_TOKENS", 32000),
		MaxFixRounds:	envInt("MAX_FIX_ROUNDS", 2),

		BrowserWSURL:		env("BROWSER_WS_URL", ""),
		BrowserTimeout:		time.Duration(envInt("BROWSER_TEST_TIMEOUT_SEC", 40)) * time.Second,
		BrowserMaxParallel:	envInt("BROWSER_MAX_PARALLEL", 2),
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

func envList(k string) []string {
	var out []string
	for _, p := range strings.Split(os.Getenv(k), ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
