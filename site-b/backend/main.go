package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"

	"site-b/backend/internal/auth"
	"site-b/backend/internal/config"
	"site-b/backend/internal/handler"
)

func main() {
	// Load .env if present. Variables already in the environment take precedence.
	if err := godotenv.Load(); err != nil && !errors.Is(err, fs.ErrNotExist) {
		fail("failed to read .env: %v", err)
	}

	cfg, err := config.LoadFromEnv()
	if err != nil {
		fail("failed to load config: %v (copy env.example to .env and fill in the authentik client credentials)", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	provider, err := auth.NewOIDCProvider(ctx, cfg.OIDC)
	cancel()
	if err != nil {
		fail("OIDC discovery failed: %v (is authentik running? try `make authentik-up`)", err)
	}

	r := handler.NewRouter(cfg, provider, auth.NewStore(nil))

	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("%s backend listening on %s (issuer %s)", cfg.SiteName, addr, cfg.OIDC.Issuer)
	if err := r.Run(addr); err != nil {
		fail("server stopped: %v", err)
	}
}

func fail(format string, args ...any) {
	log.Printf(format, args...)
	os.Exit(1)
}
