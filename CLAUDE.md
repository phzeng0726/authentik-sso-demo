# CLAUDE.md

A demo of single sign-on (SSO) between two different websites using authentik.

## Rules

- Always respond in Traditional Chinese (繁體中文).
- Always write Git Commit Message in English.
- Do not read or modify .env\* files unless explicitly requested.
- Always prefer Edit over Write for existing files.
- Always use forward slashes in file paths, not backslashes.
- When running /opsx:apply or any OpenSpec-related task, mark each task as completed immediately after finishing it.
- All unit tests must follow TESTING_GUIDELINES.md.
- If the project has Swagger docs, ensure they match the business logic and follow /write-api-docs.
- In Plan Mode, raise any open questions and ask only one question at a time.

## Architecture

- `authentik/`: authentik identity provider (server, worker, PostgreSQL 16) run with Docker Compose; settings come from `authentik/.env` (gitignored).
- `site-a/`, `site-b/`: two independent demo sites with the same layout (code is duplicated on purpose, not shared); SSO via authentik is not implemented yet.
  - `backend/`: Go + Gin; `internal/config` (env `PORT`, `SITE_NAME`), `internal/handler` (routes under `/api`). site-a: 8081, site-b: 8082.
  - `frontend/`: React + TS + Vite + Tailwind v4; Vite proxies `/api` to its own backend. site-a: 3001, site-b: 3002.
- `openspec/`: OpenSpec specs and change proposals (schema: spec-driven).

## Notes

- Frontend uses pnpm only (never npm).
- Dev shortcuts live in the root `Makefile` (`make help`): `make dev`, `make site-a`, `make test`, `make authentik-up`. Recipes must stay compatible with both cmd.exe and sh (no parentheses in `echo`, quote `"$(MAKE)"`).
- authentik UI on port 9000 / 9443 by default.
- `authentik/data`, `authentik/certs`, `authentik/custom-templates` are runtime volume mounts.
