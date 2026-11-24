# Dev shortcuts for authentik-sso-demo.
# Commands are kept compatible with both cmd.exe and sh, so `make` works from
# PowerShell and Git Bash alike. Press Ctrl+C to stop running services.

.PHONY: help install dev site-a site-b \
	site-a-backend site-a-frontend site-b-backend site-b-frontend \
	authentik-up authentik-down authentik-logs authentik-check create-users test

AUTHENTIK_URL = http://localhost:9000

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
	@echo   authentik-check   Check the OIDC discovery and JWKS of both sites
	@echo   create-users      Create authentik users from tools/create-users/users.csv
	@echo   test              Run unit tests of both backends and tools

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

# Fails if an application slug is wrong or authentik is down. Output goes to NUL,
# not /dev/null: curl is a native Windows program in both cmd.exe and sh.
authentik-check:
	curl -fsS -m 5 -o NUL $(AUTHENTIK_URL)/application/o/site-a/.well-known/openid-configuration
	curl -fsS -m 5 -o NUL $(AUTHENTIK_URL)/application/o/site-a/jwks/
	@echo site-a: discovery and JWKS OK
	curl -fsS -m 5 -o NUL $(AUTHENTIK_URL)/application/o/site-b/.well-known/openid-configuration
	curl -fsS -m 5 -o NUL $(AUTHENTIK_URL)/application/o/site-b/jwks/
	@echo site-b: discovery and JWKS OK

# Needs AUTHENTIK_TOKEN in tools/create-users/.env, see env.example there.
create-users:
	cd tools/create-users && go run . -csv users.csv

test:
	cd site-a/backend && go test ./...
	cd site-b/backend && go test ./...
	cd tools/create-users && go test ./...
