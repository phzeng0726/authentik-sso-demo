## ADDED Requirements

### Requirement: Automatic login redirect
When `GET /api/me` returns HTTP 401 and the page was not loaded with an `auth_error` query parameter, the frontend SHALL navigate the browser to `/api/auth/login` (a full-page navigation, without `prompt`) and SHALL show "前往登入頁…" instead of site content while navigating. The site SHALL NOT offer a "登入" button for this case: authentik either signs the user in at once (it already has a session, e.g. from the other site) or shows its own login page, and after login the backend redirects back to this site's `/`.

#### Scenario: Not signed in anywhere
- **WHEN** the user opens site-a without a site-a session and without an authentik session
- **THEN** the browser navigates to `/api/auth/login`, authentik shows its login page, and after signing in the user is back on site-a showing the user information

#### Scenario: Already signed in at authentik
- **WHEN** the user signed in on site-a, then opens site-b in a new tab
- **THEN** site-b navigates to `/api/auth/login` and ends up showing the user, without the user entering credentials or clicking any button

#### Scenario: Returns to the originating site
- **WHEN** the login was started from site-b
- **THEN** after signing in at authentik the browser returns to site-b, not site-a

#### Scenario: Login error stops the redirect
- **WHEN** the page loads with `?auth_error=token_error` and `GET /api/me` returns HTTP 401
- **THEN** the page does not navigate to `/api/auth/login` and shows the error (see "Show login errors")

## MODIFIED Requirements

### Requirement: Show login state
The frontend home page SHALL call `GET /api/me` on load. When it returns HTTP 200, the page SHALL show the user's `name`, `username`, and `email`, and a "登出" button. When it returns HTTP 401, the page SHALL NOT show a "登入" button; it SHALL either redirect to login (see "Automatic login redirect") or show the login error (see "Show login errors").

#### Scenario: Logged in
- **WHEN** `GET /api/me` returns `{"sub":"abc","username":"alice","name":"Alice","email":"alice@example.com"}`
- **THEN** the page shows `Alice`, `alice`, and `alice@example.com`, and a "登出" button

#### Scenario: Not logged in
- **WHEN** `GET /api/me` returns HTTP 401
- **THEN** the page shows no "登入" button and no user information

### Requirement: Logout action
Clicking the "登出" button SHALL send `POST /api/auth/logout` and then navigate the browser to the returned `logout_url`. The frontend SHALL NOT store any marker in `sessionStorage` for logout. If the request fails, the page SHALL show "登出失敗，請稍後再試。" and keep the user information.

#### Scenario: User clicks logout
- **WHEN** the user clicks "登出" and the backend returns `{"logout_url":"http://localhost:9000/application/o/site-a/end-session/?id_token_hint=xyz&post_logout_redirect_uri=http%3A%2F%2Flocalhost%3A3001%2F"}`
- **THEN** the browser navigates to that URL

#### Scenario: Returning after logout
- **WHEN** authentik ends its session and redirects back to `http://localhost:3001/` after logout
- **THEN** `GET /api/me` returns HTTP 401, the page navigates to `/api/auth/login`, and authentik shows its login page

#### Scenario: Logout request fails
- **WHEN** `POST /api/auth/logout` fails
- **THEN** the page shows "登出失敗，請稍後再試。" and still shows the user information and the "登出" button

### Requirement: Show login errors
When the page URL contains an `auth_error` query parameter, the page SHALL show an error message describing the failure, SHALL remove the parameter from the address bar, and, when `GET /api/me` returns HTTP 401, SHALL show a "重新登入" button that navigates the browser to `/api/auth/login`.

#### Scenario: Login failed
- **WHEN** the page loads with `?auth_error=token_error`
- **THEN** the page shows a login failure message and the address bar no longer contains `auth_error`

#### Scenario: Retry after a login error
- **WHEN** the page shows a login error, `GET /api/me` returned HTTP 401, and the user clicks "重新登入"
- **THEN** the browser navigates to `/api/auth/login`

#### Scenario: Reload after a login error
- **WHEN** the user reloads the page after the error was shown (the address bar no longer has `auth_error`)
- **THEN** the page redirects to login automatically again

## REMOVED Requirements

### Requirement: Login action
**Reason**: The site no longer has a "登入" button; login starts automatically when `GET /api/me` returns HTTP 401.
**Migration**: See "Automatic login redirect". The only manual entry point left is the "重新登入" button shown with a login error (see "Show login errors").

### Requirement: Automatic silent login
**Reason**: A normal authorization request already signs the user in without any page when authentik has a session, so the separate `prompt=none` attempt and its `sessionStorage` marker are no longer needed.
**Migration**: See "Automatic login redirect". The `silentLoginTried` key in `sessionStorage` is no longer read or written.
