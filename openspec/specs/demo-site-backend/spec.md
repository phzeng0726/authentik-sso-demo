# demo-site-backend Specification

## Purpose
Backend API service of each demo site (site-a, site-b): health check, site info, and environment-based configuration. Serves as the base that authentik SSO will be added to.
## Requirements
### Requirement: Health check endpoint
The backend SHALL expose `GET /api/health`, which responds with HTTP 200 and the JSON body `{"status":"ok"}`.

#### Scenario: Service is running
- **WHEN** a client sends `GET /api/health`
- **THEN** the backend responds with HTTP 200, `Content-Type: application/json`, and body `{"status":"ok"}`

### Requirement: Site info endpoint
The backend SHALL expose `GET /api/info`, which responds with HTTP 200 and a JSON body whose `name` field is the configured site name.

#### Scenario: Site name is configured
- **WHEN** the backend starts with `SITE_NAME=Site A` and a client sends `GET /api/info`
- **THEN** the backend responds with HTTP 200 and body `{"name":"Site A"}`

#### Scenario: Site name is not configured
- **WHEN** the site-a backend starts without `SITE_NAME` and a client sends `GET /api/info`
- **THEN** the backend responds with HTTP 200 and body `{"name":"Site A"}` (the site's default name)

### Requirement: Unknown API routes
The backend SHALL respond with HTTP 404 for routes that are not defined.

#### Scenario: Undefined route
- **WHEN** a client sends `GET /api/does-not-exist`
- **THEN** the backend responds with HTTP 404

### Requirement: Environment-based configuration
The backend SHALL read its settings from environment variables. Before reading them, it SHALL load a `.env` file from its working directory if one exists; variables already set in the environment SHALL take precedence over the file. Optional variables SHALL fall back to the site's default value when unset or empty:

| Variable | site-a default | site-b default |
|----------|----------------|----------------|
| `PORT` | `8081` | `8082` |
| `SITE_NAME` | `Site A` | `Site B` |
| `OIDC_ISSUER` | `http://localhost:9000/application/o/site-a/` | `http://localhost:9000/application/o/site-b/` |
| `OIDC_REDIRECT_URL` | `http://localhost:3001/api/auth/callback` | `http://localhost:3002/api/auth/callback` |
| `OIDC_POST_LOGOUT_REDIRECT_URL` | `http://localhost:3001/` | `http://localhost:3002/` |

`OIDC_CLIENT_ID` and `OIDC_CLIENT_SECRET` are required and have no default.

#### Scenario: All variables set
- **WHEN** `PORT=9100`, `SITE_NAME=Custom`, `OIDC_ISSUER=http://idp/`, `OIDC_REDIRECT_URL=http://app/cb`, `OIDC_POST_LOGOUT_REDIRECT_URL=http://app/`, `OIDC_CLIENT_ID=id`, and `OIDC_CLIENT_SECRET=secret` are set
- **THEN** the loaded config has exactly those values

#### Scenario: Variables unset
- **WHEN** only `OIDC_CLIENT_ID=id` and `OIDC_CLIENT_SECRET=secret` are set for site-a
- **THEN** the loaded config has port `8081`, site name `Site A`, issuer `http://localhost:9000/application/o/site-a/`, redirect URL `http://localhost:3001/api/auth/callback`, and post-logout redirect URL `http://localhost:3001/`

### Requirement: Port validation
The backend SHALL accept `PORT` only when it is an integer from 1 to 65535 inclusive. Otherwise config loading SHALL fail with the sentinel error `ErrInvalidPort` and the process SHALL exit with a non-zero status without starting the server.

#### Scenario: Port at lower bound
- **WHEN** `PORT=1`
- **THEN** config loads successfully with port `1`

#### Scenario: Port at upper bound
- **WHEN** `PORT=65535`
- **THEN** config loads successfully with port `65535`

#### Scenario: Port below range
- **WHEN** `PORT=0`
- **THEN** config loading fails with `ErrInvalidPort`

#### Scenario: Port above range
- **WHEN** `PORT=65536`
- **THEN** config loading fails with `ErrInvalidPort`

#### Scenario: Port is not a number
- **WHEN** `PORT=abc`
- **THEN** config loading fails with `ErrInvalidPort`

### Requirement: Required OIDC credentials
Config loading SHALL fail with the sentinel error `ErrMissingConfig` when `OIDC_CLIENT_ID` or `OIDC_CLIENT_SECRET` is unset or empty, and the process SHALL exit with a non-zero status without starting the server.

#### Scenario: Client ID missing
- **WHEN** `OIDC_CLIENT_SECRET=secret` is set but `OIDC_CLIENT_ID` is unset
- **THEN** config loading fails with `ErrMissingConfig`

#### Scenario: Client secret empty
- **WHEN** `OIDC_CLIENT_ID=id` is set and `OIDC_CLIENT_SECRET` is empty
- **THEN** config loading fails with `ErrMissingConfig`

### Requirement: OIDC provider discovery at startup
At startup the backend SHALL fetch the OIDC discovery document from `OIDC_ISSUER`. If discovery fails, the process SHALL log the error and exit with a non-zero status without starting the server.

#### Scenario: authentik not running
- **WHEN** the backend starts while authentik is not reachable
- **THEN** the process logs that OIDC discovery failed and exits with a non-zero status
