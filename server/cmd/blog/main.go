package main

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/T-DWAG/blog_build/server/internal/agent"
	"github.com/T-DWAG/blog_build/server/internal/api"
	"github.com/T-DWAG/blog_build/server/internal/auth"
	"github.com/T-DWAG/blog_build/server/internal/config"
	"github.com/T-DWAG/blog_build/server/internal/ratelimit"
	"github.com/T-DWAG/blog_build/server/internal/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	ctx := context.Background()
	st, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer st.Close()

	if err := st.Migrate(ctx); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	hash, err := auth.Hash(cfg.AdminPassword)
	if err != nil {
		log.Fatalf("hash password: %v", err)
	}
	if err := st.SeedAdmin(ctx, cfg.AdminUsername, string(hash)); err != nil {
		log.Fatalf("seed admin: %v", err)
	}
	if err := st.SeedSettings(ctx); err != nil {
		log.Fatalf("seed settings: %v", err)
	}

	// AI 分身：未配置 AI_API_KEY 时不阻塞启动，仅 /api/ai/chat 返回 503 不可用。
	var ag agent.Service
	a, err := agent.New(ctx, cfg, st)
	if errors.Is(err, agent.ErrNoAPIKey) {
		log.Println("AI avatar disabled: AI_API_KEY not set")
	} else if err != nil {
		log.Fatalf("init agent: %v", err)
	} else {
		ag = a
	}

	if err := api.New(cfg, st, ratelimit.NewWindow(time.Minute), ag).Listen(); err != nil {
		log.Fatalf("listen: %v", err)
	}
}
