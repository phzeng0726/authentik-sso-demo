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
- `site-a/`, `site-b/` (planned): two demo sites that sign in through authentik.
- `openspec/`: OpenSpec specs and change proposals (schema: spec-driven).

## Notes

- Start authentik: `docker compose -f authentik/docker-compose.yml up -d` (UI on port 9000 / 9443 by default).
- `authentik/data`, `authentik/certs`, `authentik/custom-templates` are runtime volume mounts.
