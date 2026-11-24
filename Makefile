# Dev shortcuts for authentik-sso-demo.
# Commands are kept compatible with both cmd.exe and sh, so `make` works from
# PowerShell and Git Bash alike. Press Ctrl+C to stop running services.

.PHONY: help install dev site-a site-b \
	site-a-backend site-a-frontend site-b-backend site-b-frontend \
	authentik-up authentik-down authentik-logs test

help:
	@echo Usage: make [target]
	@echo   install           Install frontend dependencies for both sites with pnpm
	@echo   dev               Run backend + frontend of site-a and site-b
	@echo   site-a            Run site-a backend on 8081 + frontend on 3001
	@echo   site-b            Run site-b backend on 8082 + frontend on 3002
	@echo   site-a-backend    Run site-a backend only
	@echo   site-a-frontend   Run site-a frontend only
	@echo   site-b-backend    Run site-b backend only
	@echo   site-b-frontend   Run site-b frontend only
	@echo   authentik-up      Start authentik in the background on 9000
	@echo   authentik-down    Stop authentik
	@echo   authentik-logs    Follow authentik logs
	@echo   test              Run backend unit tests for both sites

install:
	cd site-a/frontend && pnpm install
	cd site-b/frontend && pnpm install

dev:
	"$(MAKE)" -j4 site-a-backend site-a-frontend site-b-backend site-b-frontend

site-a:
	"$(MAKE)" -j2 site-a-backend site-a-frontend

site-b:
	"$(MAKE)" -j2 site-b-backend site-b-frontend

site-a-backend:
	cd site-a/backend && go run .

site-a-frontend:
	cd site-a/frontend && pnpm dev

site-b-backend:
	cd site-b/backend && go run .

site-b-frontend:
	cd site-b/frontend && pnpm dev

authentik-up:
	docker compose -f authentik/docker-compose.yml up -d

authentik-down:
	docker compose -f authentik/docker-compose.yml down

authentik-logs:
	docker compose -f authentik/docker-compose.yml logs -f

test:
	cd site-a/backend && go test ./...
	cd site-b/backend && go test ./...
