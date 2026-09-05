package config

import (
	"os"
	"testing"
)

// clearEnv 清掉可能影响测试的环境变量，保证每个用例从干净状态出发。
func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"ADDR", "DATABASE_URL", "JWT_SECRET", "ADMIN_USERNAME", "ADMIN_PASSWORD",
		"AI_API_KEY", "AI_BASE_URL", "AI_MODEL",
	} {
		if err := os.Unsetenv(k); err != nil {
			t.Fatalf("unsetenv %s: %v", k, err)
		}
	}
}

// setRequired 补上非 AI 的必需环境变量，让 Load 能走到 AI 配置分支。
func setRequired(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://test")
	t.Setenv("JWT_SECRET", "secret")
	t.Setenv("ADMIN_USERNAME", "admin")
	t.Setenv("ADMIN_PASSWORD", "pass")
}

func TestLoadDefaults(t *testing.T) {
	clearEnv(t)
	setRequired(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Addr != ":8080" {
		t.Errorf("Addr = %q, want %q", cfg.Addr, ":8080")
	}
	if cfg.AIBaseURL != "https://api.deepseek.com/v1" {
		t.Errorf("AIBaseURL = %q, want default deepseek endpoint", cfg.AIBaseURL)
	}
	if cfg.AIModel != "deepseek-chat" {
		t.Errorf("AIModel = %q, want %q", cfg.AIModel, "deepseek-chat")
	}
	if cfg.AIAPIKey != "" {
		t.Errorf("AIAPIKey = %q, want empty", cfg.AIAPIKey)
	}
}

func TestLoadAIEnvOverrides(t *testing.T) {
	clearEnv(t)
	setRequired(t)
	t.Setenv("AI_API_KEY", "sk-test-123")
	t.Setenv("AI_BASE_URL", "https://example.com/v1")
	t.Setenv("AI_MODEL", "my-model")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.AIAPIKey != "sk-test-123" {
		t.Errorf("AIAPIKey = %q, want %q", cfg.AIAPIKey, "sk-test-123")
	}
	if cfg.AIBaseURL != "https://example.com/v1" {
		t.Errorf("AIBaseURL = %q, want %q", cfg.AIBaseURL, "https://example.com/v1")
	}
	if cfg.AIModel != "my-model" {
		t.Errorf("AIModel = %q, want %q", cfg.AIModel, "my-model")
	}
}

func TestLoadEmptyAPIKeyStillStarts(t *testing.T) {
	clearEnv(t)
	setRequired(t)
	t.Setenv("AI_API_KEY", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() with empty AI_API_KEY should not fail, got %v", err)
	}
	if cfg.AIAPIKey != "" {
		t.Errorf("AIAPIKey = %q, want empty", cfg.AIAPIKey)
	}
}

func TestLoadMissingRequiredFails(t *testing.T) {
	clearEnv(t)

	if _, err := Load(); err == nil {
		t.Fatal("Load() with missing required vars should fail")
	}
}
