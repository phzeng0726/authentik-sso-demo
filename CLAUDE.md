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
- `site-a/`, `site-b/`: two independent demo sites with the same layout (code is duplicated on purpose, not shared) that sign in through authentik via OIDC.
  - `backend/`: Go + Gin; BFF-style OIDC Authorization Code Flow with PKCE, state and nonce. site-a: 8081, site-b: 8082.
    - `internal/config`: env `PORT`, `SITE_NAME`, `OIDC_ISSUER`, `OIDC_REDIRECT_URL`, `OIDC_POST_LOGOUT_REDIRECT_URL`, `OIDC_CLIENT_ID`/`OIDC_CLIENT_SECRET` (required). `main.go` loads `.env` via godotenv; `env.example` is the committed template (not `.env.example`, which `/commit` treats as sensitive).
    - `internal/auth`: `Provider` interface (go-oidc + x/oauth2) with a mockgen mock in `auth/mock` (`go generate ./internal/auth/`), and an in-memory `Store` for sessions (8h) and pending logins (10m) with an injectable clock.
    - `internal/handler`: routes under `/api`; `RequireAuth` middleware protects `/api/me`. Cookies are named per site (`site_a_session`/`site_a_login`, `site_b_*`) because localhost cookies are shared across ports.
    - The backend exits at startup if OIDC discovery fails, so authentik must be running first.
    - No `prompt` is sent: authentik signs in at once with an existing session or shows its login page. Every callback failure redirects to `/?auth_error=<code>`.
    - Logout URL carries `id_token_hint` and `post_logout_redirect_uri` only when a session existed; providers use `default-invalidation-flow` so authentik's session ends.
  - Frontend has no login button: on 401 from `/api/me` it redirects to `/api/auth/login`, except when the page loaded with `auth_error`, where it shows the error and a "重新登入" button (prevents redirect loops).
  - When syncing code from site-a to site-b, swap `site-a`/`site-b` and `site_a_`/`site_b_` both ways (tests reference the other site's cookie), and change 8081→8082, 3001→3002, `Site A`→`Site B`, `blue`→`emerald`.
  - `frontend/`: React + TS + Vite + Tailwind v4; Vite proxies `/api` to its own backend. site-a: 3001, site-b: 3002.
- `tools/create-users/`: standalone Go module (stdlib HTTP + godotenv) that bulk-creates authentik users from a CSV via the REST API (`AUTHENTIK_TOKEN` in its `.env`). Skips existing usernames, creates missing groups, generates passwords for blank ones. `users.csv` is gitignored.
- `openspec/`: OpenSpec specs and change proposals (schema: spec-driven).

## Notes

- Frontend uses pnpm only (never npm).
- Dev shortcuts live in the root `Makefile` (`make help`): `make dev`, `make site-a`, `make test`, `make authentik-up`, `make authentik-check`. Recipes must stay compatible with both cmd.exe and sh (no parentheses in `echo`, quote `"$(MAKE)"`, discard output with `-o NUL` not `/dev/null`, no `grep`, avoid curl `-w` format strings).
- authentik is configured by hand through its UI (the user is learning); README lists each step and how it could be automated.
- authentik UI on port 9000 / 9443 by default.
- `authentik/data`, `authentik/certs`, `authentik/custom-templates` are runtime volume mounts.
