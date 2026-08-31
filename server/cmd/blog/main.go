package main

import (
	"log"

	"github.com/T-DWAG/blog_build/server/internal/api"
	"github.com/T-DWAG/blog_build/server/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	srv := api.New(cfg)
	if err := srv.Listen(); err != nil {
		log.Fatalf("listen: %v", err)
	}
}
