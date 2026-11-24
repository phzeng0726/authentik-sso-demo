# oidc-auth Specification

## Purpose
Backend sign-in of each demo site through authentik: OIDC Authorization Code Flow with PKCE, state and nonce, server-side sessions with per-site cookies, the current-user endpoint, login-protected routes, and logout.
## Requirements
### Requirement: Start login
The backend SHALL expose `GET /api/auth/login`, which starts an OIDC Authorization Code Flow with PKCE (`S256`) against the configured authentik issuer. It SHALL generate a random `state`, `nonce`, and PKCE code verifier, store them server-side as a pending login that expires after 10 minutes, set an HttpOnly login-flow cookie that identifies the pending login, and respond with HTTP 302 to the authorization endpoint with `scope=openid profile email`. The backend SHALL NOT send a `prompt` parameter to authentik, and SHALL ignore any `prompt` query parameter on the request: authentik itself decides whether to sign the user in at once (existing authentik session) or to show its login page.

#### Scenario: User starts login
- **WHEN** a client sends `GET /api/auth/login`
- **THEN** the backend responds with HTTP 302 whose `Location` is the authorization endpoint with `response_type=code`, the configured `client_id` and `redirect_uri`, `scope=openid profile email`, a `state`, a `nonce`, `code_challenge_method=S256`, and a `code_challenge`, and without `prompt`
- **AND** the response sets the login-flow cookie with `HttpOnly`, `SameSite=Lax`, and `Path=/api/auth`

#### Scenario: Prompt query parameter ignored
- **WHEN** a client sends `GET /api/auth/login?prompt=none`
- **THEN** the redirect `Location` does not contain `prompt`

### Requirement: Handle login callback
The backend SHALL expose `GET /api/auth/callback`. It SHALL accept the callback only when the login-flow cookie refers to an unexpired pending login and the `state` query parameter equals that login's `state`. It SHALL exchange the `code` for tokens using the PKCE code verifier, verify the ID token (signature, issuer, audience, expiry) and that its `nonce` equals the pending login's `nonce`. On success it SHALL create a session, set the session cookie, delete the pending login and the login-flow cookie, and respond with HTTP 302 to `/`. On any failure it SHALL NOT create a session and SHALL respond with HTTP 302 to `/?auth_error=<code>`, so the frontend stops redirecting to login automatically.

#### Scenario: Successful callback
- **WHEN** authentik redirects to `/api/auth/callback?code=<valid>&state=<matching>` with a valid login-flow cookie
- **THEN** the backend creates a session, sets the session cookie, and responds with HTTP 302 to `/`

#### Scenario: State mismatch
- **WHEN** the `state` query parameter differs from the pending login's `state`
- **THEN** the backend responds with HTTP 302 to `/?auth_error=invalid_state` and does not set a session cookie

#### Scenario: Missing or expired pending login
- **WHEN** the login-flow cookie is missing, or refers to a pending login that does not exist or has expired
- **THEN** the backend responds with HTTP 302 to `/?auth_error=invalid_state`

#### Scenario: Authentik returns an error
- **WHEN** the callback contains an `error` query parameter (for example, `access_denied` or `login_required`) with a matching `state`
- **THEN** the backend responds with HTTP 302 to `/?auth_error=access_denied` and does not set a session cookie

#### Scenario: Token exchange or verification fails
- **WHEN** the code exchange fails, or the ID token fails verification
- **THEN** the backend responds with HTTP 302 to `/?auth_error=token_error`

#### Scenario: Nonce mismatch
- **WHEN** the verified ID token's `nonce` differs from the pending login's `nonce`
- **THEN** the backend responds with HTTP 302 to `/?auth_error=token_error`

### Requirement: Session cookie
The session SHALL be stored server-side and identified by a random session ID in an HttpOnly cookie with `SameSite=Lax` and `Path=/`. site-a SHALL name the cookie `site_a_session` and site-b SHALL name it `site_b_session`, so the two sites do not overwrite each other on `localhost`. A session SHALL expire 8 hours after creation.

#### Scenario: Session cookie attributes
- **WHEN** site-a completes a successful login
- **THEN** the response sets cookie `site_a_session` with `HttpOnly`, `SameSite=Lax`, and `Path=/`

#### Scenario: Session expired
- **WHEN** a request carries a session ID whose session was created more than 8 hours ago
- **THEN** the backend treats the request as not logged in

### Requirement: Current user endpoint
The backend SHALL expose `GET /api/me`. For a logged-in request it SHALL respond with HTTP 200 and a JSON body with fields `sub`, `username`, `name`, and `email`, taken from the ID token claims `sub`, `preferred_username`, `name`, and `email`. Otherwise it SHALL respond with HTTP 401 and `{"error":"unauthorized"}`.

#### Scenario: Logged in
- **WHEN** a request with a valid session for user `sub=abc`, `preferred_username=alice`, `name=Alice`, `email=alice@example.com` calls `GET /api/me`
- **THEN** the backend responds with HTTP 200 and body `{"sub":"abc","username":"alice","name":"Alice","email":"alice@example.com"}`

#### Scenario: Not logged in
- **WHEN** a request without a session cookie, or with an unknown or expired session ID, calls `GET /api/me`
- **THEN** the backend responds with HTTP 401 and body `{"error":"unauthorized"}`

### Requirement: Login-protected routes
The backend SHALL provide a middleware that rejects requests without a valid session with HTTP 401 and `{"error":"unauthorized"}`, and otherwise makes the session user available to the handler. `/api/me` SHALL use this middleware. `/api/health`, `/api/info`, and `/api/auth/*` SHALL remain public.

#### Scenario: Public routes stay public
- **WHEN** a request without a session calls `GET /api/health` or `GET /api/info`
- **THEN** the backend responds with HTTP 200

### Requirement: Logout
The backend SHALL expose `POST /api/auth/logout`. It SHALL delete the current session if any, clear the session cookie, and respond with HTTP 200 and `{"logout_url":"<url>"}`, where `<url>` is authentik's `end_session_endpoint`. When a session existed, `<url>` SHALL carry `id_token_hint` set to the session's ID token and `post_logout_redirect_uri` set to the configured `OIDC_POST_LOGOUT_REDIRECT_URL`. Without a session, `<url>` SHALL be the plain end-session endpoint, because authentik may reject `post_logout_redirect_uri` without `id_token_hint`. Logging out of one site SHALL NOT end the other site's local session.

#### Scenario: Logout with a session
- **WHEN** a logged-in request to site-a sends `POST /api/auth/logout`
- **THEN** the session is deleted, the session cookie is cleared (`Max-Age=0`), and the response is HTTP 200 with `logout_url` equal to the end-session endpoint with query parameters `id_token_hint` (the session's ID token) and `post_logout_redirect_uri=http://localhost:3001/`

#### Scenario: Logout without a session
- **WHEN** a request without a session sends `POST /api/auth/logout`
- **THEN** the backend responds with HTTP 200 and `logout_url` equal to the end-session endpoint without `id_token_hint` and without `post_logout_redirect_uri`

#### Scenario: Logout uses POST only
- **WHEN** a client sends `GET /api/auth/logout`
- **THEN** the backend responds with HTTP 404 and does not delete any session
