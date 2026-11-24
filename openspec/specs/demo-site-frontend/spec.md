# demo-site-frontend Specification

## Purpose
Frontend single-page app of each demo site (site-a, site-b): shows the site name and backend health from its own backend through the dev proxy, with distinct accent colors per site.
## Requirements
### Requirement: Home page shows site name from backend
The frontend home page SHALL call `GET /api/info` on load and display the returned `name` as the page heading.

#### Scenario: Backend returns site info
- **WHEN** the user opens the home page and `GET /api/info` returns `{"name":"Site A"}`
- **THEN** the page heading shows `Site A`

### Requirement: Home page shows backend health
The frontend home page SHALL call `GET /api/health` on load and display the backend status.

#### Scenario: Backend is healthy
- **WHEN** `GET /api/health` returns `{"status":"ok"}`
- **THEN** the page shows the backend status as `ok`

### Requirement: Loading and error states
The frontend SHALL show a loading indicator while API requests are pending, and SHALL show an error message instead of data when a request fails (network error or non-2xx response).

#### Scenario: Request pending
- **WHEN** the API requests have not completed yet
- **THEN** the page shows a loading indicator

#### Scenario: Backend unavailable
- **WHEN** the backend is not running and the request fails
- **THEN** the page shows an error message stating the backend could not be reached

### Requirement: API proxy in development
In development, the frontend dev server SHALL forward requests under `/api` to its own site's backend, so the browser calls the same origin. site-a's frontend runs on port `3001` and forwards to `http://localhost:8081`; site-b's frontend runs on port `3002` and forwards to `http://localhost:8082`.

#### Scenario: site-a calls its backend
- **WHEN** the site-a frontend at `http://localhost:3001` requests `/api/info`
- **THEN** the request is forwarded to `http://localhost:8081/api/info`

#### Scenario: site-b calls its backend
- **WHEN** the site-b frontend at `http://localhost:3002` requests `/api/info`
- **THEN** the request is forwarded to `http://localhost:8082/api/info`

### Requirement: Visually distinguishable sites
Each site SHALL use a distinct accent color so users can tell site-a and site-b apart during the SSO demo.

#### Scenario: Comparing both sites
- **WHEN** the user opens site-a and site-b side by side
- **THEN** the two pages use different accent colors

