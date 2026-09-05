package config

import (
	"fmt"
	"os"
)

// Config 是进程启动所需的运行参数。
type Config struct {
	Addr          string
	DatabaseURL   string
	JWTSecret     string
	AdminUsername string
	AdminPassword string
	// AIAPIKey 为 provider 无关的 AI 服务密钥，可留空：留空时服务仍可启动，
	// 仅使用 AI 能力的接口会返回不可用。
	AIAPIKey string
	// AIBaseURL 为 AI 服务的基础地址，默认 DeepSeek 兼容 OpenAI 协议端点。
	AIBaseURL string
	// AIModel 为默认使用的模型名。
	AIModel string
}

const (
	defaultAIBaseURL = "https://api.deepseek.com/v1"
	defaultAIModel   = "deepseek-chat"
)

// Load 从环境变量读取配置。
// ADDR 为空时默认 :8080；数据库/密钥/管理员四项任一为空则报错，进程不得启动。
// AI 三项为 provider 无关的可选配置：AI_API_KEY 可留空（不阻塞启动），
// AI_BASE_URL 与 AI_MODEL 留空时使用默认值。
// 管理台静态资源已 embed，不再读 FRONTEND_DIR。
func Load() (Config, error) {
	cfg := Config{
		Addr:          os.Getenv("ADDR"),
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		JWTSecret:     os.Getenv("JWT_SECRET"),
		AdminUsername: os.Getenv("ADMIN_USERNAME"),
		AdminPassword: os.Getenv("ADMIN_PASSWORD"),
		AIAPIKey:      os.Getenv("AI_API_KEY"),
		AIBaseURL:     os.Getenv("AI_BASE_URL"),
		AIModel:       os.Getenv("AI_MODEL"),
	}
	if cfg.Addr == "" {
		cfg.Addr = ":8080"
	}
	if cfg.AIBaseURL == "" {
		cfg.AIBaseURL = defaultAIBaseURL
	}
	if cfg.AIModel == "" {
		cfg.AIModel = defaultAIModel
	}
	for _, item := range []struct {
		name, val string
	}{
		{"DATABASE_URL", cfg.DatabaseURL},
		{"JWT_SECRET", cfg.JWTSecret},
		{"ADMIN_USERNAME", cfg.AdminUsername},
		{"ADMIN_PASSWORD", cfg.AdminPassword},
	} {
		if item.val == "" {
			return Config{}, fmt.Errorf("missing %s", item.name)
		}
	}
	return cfg, nil
}
