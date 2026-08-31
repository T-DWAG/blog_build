package main

import (
	"context"
	"log"

	"golang.org/x/crypto/bcrypt"

	"github.com/T-DWAG/blog_build/server/internal/api"
	"github.com/T-DWAG/blog_build/server/internal/config"
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

	hash, err := bcrypt.GenerateFromPassword([]byte(cfg.AdminPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("hash password: %v", err)
	}
	if err := st.SeedAdmin(ctx, cfg.AdminUsername, string(hash)); err != nil {
		log.Fatalf("seed admin: %v", err)
	}
	if err := st.SeedSettings(ctx); err != nil {
		log.Fatalf("seed settings: %v", err)
	}

	if err := api.New(cfg, st).Listen(); err != nil {
		log.Fatalf("listen: %v", err)
	}
}
