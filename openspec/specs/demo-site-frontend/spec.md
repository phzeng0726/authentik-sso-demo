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

### Requirement: Show login state
The frontend home page SHALL call `GET /api/me` on load. When it returns HTTP 401, the page SHALL show a "登入" button, unless it is starting a silent login (see "Automatic silent login"). When it returns HTTP 200, the page SHALL show the user's `name`, `username`, and `email`, and a "登出" button.

#### Scenario: Not logged in
- **WHEN** `GET /api/me` returns HTTP 401 and this tab has already tried a silent login
- **THEN** the page shows a "登入" button and no user information

#### Scenario: Logged in
- **WHEN** `GET /api/me` returns `{"sub":"abc","username":"alice","name":"Alice","email":"alice@example.com"}`
- **THEN** the page shows `Alice`, `alice`, and `alice@example.com`, and a "登出" button

### Requirement: Login action
Clicking the "登入" button SHALL navigate the browser to `/api/auth/login`.

#### Scenario: User clicks login
- **WHEN** the user clicks "登入"
- **THEN** the browser navigates to `/api/auth/login`

### Requirement: Logout action
Clicking the "登出" button SHALL first set the silent-login mark in `sessionStorage` (so returning to the site does not immediately sign in again), then send `POST /api/auth/logout`, and then navigate the browser to the returned `logout_url`.

#### Scenario: User clicks logout
- **WHEN** the user clicks "登出" and the backend returns `{"logout_url":"http://localhost:9000/application/o/site-a/end-session/?id_token_hint=xyz"}`
- **THEN** the browser navigates to that URL

#### Scenario: Returning after logout
- **WHEN** the user logged out in this tab and navigates back to the site while authentik still has a session
- **THEN** the page shows the "登入" button and does not attempt a silent login

### Requirement: Automatic silent login
When `GET /api/me` returns HTTP 401, the URL has no `auth_error`, and this tab has not tried a silent login yet, the frontend SHALL set the silent-login mark in `sessionStorage` and navigate to `/api/auth/login?prompt=none`, showing "檢查登入狀態中…" instead of the "登入" button. When `GET /api/me` returns HTTP 200, the frontend SHALL clear the mark, so a later expired session can be renewed silently.

#### Scenario: Already signed in at authentik
- **WHEN** the user signed in on site-a, then opens site-b in a new tab
- **THEN** site-b navigates to `/api/auth/login?prompt=none` and ends up showing the user without the user clicking "登入"

#### Scenario: Not signed in at authentik
- **WHEN** the silent attempt returns to `/` and `GET /api/me` still returns HTTP 401
- **THEN** the page shows the "登入" button and does not navigate again

#### Scenario: Login error shown
- **WHEN** the page loads with `?auth_error=token_error`
- **THEN** the page shows the error and does not attempt a silent login

#### Scenario: Signed in clears the mark
- **WHEN** `GET /api/me` returns HTTP 200
- **THEN** the silent-login mark is removed from `sessionStorage`

### Requirement: Show login errors
When the page URL contains an `auth_error` query parameter, the page SHALL show an error message describing the failure and SHALL remove the parameter from the address bar.

#### Scenario: Login failed
- **WHEN** the page loads with `?auth_error=token_error`
- **THEN** the page shows a login failure message and the address bar no longer contains `auth_error`

