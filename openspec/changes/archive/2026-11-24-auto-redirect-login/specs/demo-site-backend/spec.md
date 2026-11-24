## MODIFIED Requirements

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
