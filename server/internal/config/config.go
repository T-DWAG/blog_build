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
	FrontendDir   string
}

// Load 从环境变量读取配置。
// ADDR 为空时默认 :8080；其余四项任一为空则报错，进程不得启动。
// FRONTEND_DIR 默认 ../frontend（管理台静态资源用）。
func Load() (Config, error) {
	cfg := Config{
		Addr:          os.Getenv("ADDR"),
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		JWTSecret:     os.Getenv("JWT_SECRET"),
		AdminUsername: os.Getenv("ADMIN_USERNAME"),
		AdminPassword: os.Getenv("ADMIN_PASSWORD"),
		FrontendDir:   os.Getenv("FRONTEND_DIR"),
	}
	if cfg.Addr == "" {
		cfg.Addr = ":8080"
	}
	if cfg.FrontendDir == "" {
		cfg.FrontendDir = "../frontend"
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
