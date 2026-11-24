## ADDED Requirements

### Requirement: Show login state
The frontend home page SHALL call `GET /api/me` on load. When it returns HTTP 401, the page SHALL show a "登入" button. When it returns HTTP 200, the page SHALL show the user's `name`, `username`, and `email`, and a "登出" button.

#### Scenario: Not logged in
- **WHEN** `GET /api/me` returns HTTP 401
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
Clicking the "登出" button SHALL send `POST /api/auth/logout` and then navigate the browser to the returned `logout_url`.

#### Scenario: User clicks logout
- **WHEN** the user clicks "登出" and the backend returns `{"logout_url":"http://localhost:9000/application/o/site-a/end-session/?id_token_hint=xyz"}`
- **THEN** the browser navigates to that URL

### Requirement: Show login errors
When the page URL contains an `auth_error` query parameter, the page SHALL show an error message describing the failure and SHALL remove the parameter from the address bar.

#### Scenario: Login failed
- **WHEN** the page loads with `?auth_error=token_error`
- **THEN** the page shows a login failure message and the address bar no longer contains `auth_error`
