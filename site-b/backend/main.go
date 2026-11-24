package main

import (
	"fmt"
	"log"
	"os"

	"site-b/backend/internal/config"
	"site-b/backend/internal/handler"
)

func main() {
	cfg, err := config.LoadFromEnv()
	if err != nil {
		log.Printf("failed to load config: %v", err)
		os.Exit(1)
	}

	r := handler.NewRouter(cfg)

	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("%s backend listening on %s", cfg.SiteName, addr)
	if err := r.Run(addr); err != nil {
		log.Printf("server stopped: %v", err)
		os.Exit(1)
	}
}
