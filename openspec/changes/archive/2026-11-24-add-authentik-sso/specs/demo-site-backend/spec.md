## MODIFIED Requirements

### Requirement: Environment-based configuration
The backend SHALL read its settings from environment variables. Before reading them, it SHALL load a `.env` file from its working directory if one exists; variables already set in the environment SHALL take precedence over the file. Optional variables SHALL fall back to the site's default value when unset or empty:

| Variable | site-a default | site-b default |
|----------|----------------|----------------|
| `PORT` | `8081` | `8082` |
| `SITE_NAME` | `Site A` | `Site B` |
| `OIDC_ISSUER` | `http://localhost:9000/application/o/site-a/` | `http://localhost:9000/application/o/site-b/` |
| `OIDC_REDIRECT_URL` | `http://localhost:3001/api/auth/callback` | `http://localhost:3002/api/auth/callback` |

`OIDC_CLIENT_ID` and `OIDC_CLIENT_SECRET` are required and have no default.

#### Scenario: All variables set
- **WHEN** `PORT=9100`, `SITE_NAME=Custom`, `OIDC_ISSUER=http://idp/`, `OIDC_REDIRECT_URL=http://app/cb`, `OIDC_CLIENT_ID=id`, and `OIDC_CLIENT_SECRET=secret` are set
- **THEN** the loaded config has exactly those values

#### Scenario: Variables unset
- **WHEN** only `OIDC_CLIENT_ID=id` and `OIDC_CLIENT_SECRET=secret` are set for site-a
- **THEN** the loaded config has port `8081`, site name `Site A`, issuer `http://localhost:9000/application/o/site-a/`, and redirect URL `http://localhost:3001/api/auth/callback`

## ADDED Requirements

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
