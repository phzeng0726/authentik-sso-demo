## ADDED Requirements

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
The backend SHALL read its listen port from `PORT` and its site name from `SITE_NAME`. When a variable is unset or empty, the backend SHALL use the site's default value: site-a uses port `8081` and name `Site A`; site-b uses port `8082` and name `Site B`.

#### Scenario: All variables set
- **WHEN** `PORT=9100` and `SITE_NAME=Custom` are set
- **THEN** the loaded config has port `9100` and site name `Custom`

#### Scenario: Variables unset
- **WHEN** neither `PORT` nor `SITE_NAME` is set for site-a
- **THEN** the loaded config has port `8081` and site name `Site A`

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
