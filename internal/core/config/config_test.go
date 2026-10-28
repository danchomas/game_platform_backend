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

func TestReviewerAndPerRoleReasoningDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("JWT_SECRET", "s")
	t.Setenv("LLM_PLANNER_MODEL", "deepseek/deepseek-v4.1-flash")
	t.Setenv("LLM_REASONING", "off")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PlannerReasoning != "medium" || cfg.ReviewerReasoning != "low" || cfg.Reasoning != "off" {
		t.Errorf("reasoning: planner=%q reviewer=%q coder=%q", cfg.PlannerReasoning, cfg.ReviewerReasoning, cfg.Reasoning)
	}
	if cfg.ReviewerModel != cfg.PlannerModel {
		t.Errorf("ревьюер по умолчанию — планировщик, получено %q", cfg.ReviewerModel)
	}

	t.Setenv("LLM_REVIEWER_MODEL", "OFF")
	if cfg, _ = Load(); cfg.ReviewerModel != "" {
		t.Errorf("off выключает ревьюера, получено %q", cfg.ReviewerModel)
	}
	t.Setenv("LLM_PLANNER_REASONING", "max")
	if _, err := Load(); err == nil {
		t.Error("неверный LLM_PLANNER_REASONING должен отклоняться")
	}
}
