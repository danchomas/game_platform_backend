package config

import "testing"

func TestCoderModelPrefersLLMCoderModel(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("JWT_SECRET", "s")

	t.Setenv("OPENROUTER_MODEL", "anthropic/claude-haiku-5.5")
	t.Setenv("LLM_CODER_MODEL", "deepseek/deepseek-v4.1-flash")
	t.Setenv("LLM_FIXER_MODEL", "")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.OpenRouterModel != "deepseek/deepseek-v4.1-flash" {
		t.Errorf("кодер = %q: LLM_CODER_MODEL должен быть важнее OPENROUTER_MODEL", cfg.OpenRouterModel)
	}
	if cfg.FixerModel != cfg.OpenRouterModel {
		t.Errorf("исправитель по умолчанию должен совпадать с кодером, получено %q", cfg.FixerModel)
	}

	t.Setenv("LLM_CODER_MODEL", "")
	if cfg, _ = Load(); cfg.OpenRouterModel != "anthropic/claude-haiku-5.5" {
		t.Errorf("без LLM_CODER_MODEL кодер берётся из OPENROUTER_MODEL, получено %q", cfg.OpenRouterModel)
	}
}
