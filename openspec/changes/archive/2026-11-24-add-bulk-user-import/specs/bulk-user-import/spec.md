## ADDED Requirements

### Requirement: Configuration
The tool SHALL read the authentik API token from `AUTHENTIK_TOKEN` and the authentik base URL from `AUTHENTIK_URL` (default `http://localhost:9000`, overridable with the `-url` flag). It SHALL load a `.env` file from the working directory if present, with real environment variables taking precedence. The CSV path SHALL come from the `-csv` flag (default `users.csv`). Every API request SHALL send `Authorization: Bearer <token>` to `<base URL>/api/v3`, and a trailing slash on the base URL SHALL NOT produce a double slash.

#### Scenario: Token missing
- **WHEN** `AUTHENTIK_TOKEN` is unset or empty
- **THEN** the tool prints `AUTHENTIK_TOKEN is not set; put it in .env or the environment` to stderr and exits with code 2 without calling the API

#### Scenario: Requests use the token
- **WHEN** the tool calls the API with token `tok` and base URL `http://host/`
- **THEN** each request goes to `http://host/api/v3/...` with header `Authorization: Bearer tok`

#### Scenario: CSV file missing
- **WHEN** the file named by `-csv` does not exist
- **THEN** the tool prints the open error to stderr and exits with code 2

### Requirement: CSV format
The tool SHALL read a CSV whose first row is a header naming the columns. Header names SHALL be matched case-insensitively after trimming spaces, and a UTF-8 byte order mark before the first header SHALL be ignored. The `username` column is required; `name`, `email`, `password`, and `groups` are optional. Every field except `password` SHALL be trimmed; `password` SHALL be kept as written. `groups` SHALL be split on `;`, trimming each name and dropping empty ones. An empty `name` SHALL default to the username. Any CSV error SHALL stop the tool before any API call, with exit code 2.

#### Scenario: Only username given
- **WHEN** the CSV is `username\nalice`
- **THEN** the row is username `alice`, name `alice`, no email, no password, no groups

#### Scenario: All columns given
- **WHEN** a row has groups ` site-a; site-b;;` and password ` p@ss word `
- **THEN** the groups are `site-a` and `site-b`, and the password is ` p@ss word `

#### Scenario: Excel CSV with BOM and mixed-case header
- **WHEN** the header is `<BOM> Username ,EMAIL`
- **THEN** the `username` and `email` columns are recognized

#### Scenario: Empty file
- **WHEN** the CSV is empty
- **THEN** parsing fails with `ErrEmptyCSV`

#### Scenario: No username column
- **WHEN** the header has no `username` column
- **THEN** parsing fails with `ErrMissingColumn` and message `missing required column: username`

#### Scenario: Blank username
- **WHEN** line 3 has a blank username
- **THEN** parsing fails with `ErrMissingUsername` and message `line 3: username is required`

#### Scenario: Duplicate username
- **WHEN** `alice` appears on lines 2 and 4
- **THEN** parsing fails with `ErrDuplicateUsername` and message `line 4: duplicate username "alice", first seen on line 2`

#### Scenario: Wrong number of fields
- **WHEN** a row has more fields than the header
- **THEN** parsing fails with `csv.ErrFieldCount`

### Requirement: Create users
For each row the tool SHALL first look up the username with `GET /core/users/?username=<username>`, matching the username exactly. If the user does not exist, it SHALL create it with `POST /core/users/` (username, name, email when not empty, `is_active: true`, `path: users`, `type: internal`, and the group UUIDs, sending `[]` when there are none), then set its password with `POST /core/users/{pk}/set_password/`. A row without a password SHALL get a random password (`crypto/rand.Text`).

#### Scenario: New user with generated password
- **WHEN** a row has username `carol` and no password, and `carol` does not exist
- **THEN** the tool creates `carol` with `groups: []`, sets a generated password, and reports status `created` with that password

#### Scenario: New user with CSV password
- **WHEN** a row has password `Change-Me-123`
- **THEN** the tool sets the password `Change-Me-123` and does not report it

#### Scenario: Look-up matches exactly
- **WHEN** the look-up for `alice` returns only a user named `alice2`
- **THEN** the tool treats `alice` as not existing

### Requirement: Skip existing users
If the username already exists, the tool SHALL NOT change the user, its password, or its groups, SHALL NOT create any group for that row, and SHALL report status `skipped`. Running the same CSV twice SHALL therefore be safe.

#### Scenario: Username exists
- **WHEN** a row's username already exists in authentik
- **THEN** no create, set-password, or group request is sent for that row, and the row is reported as `skipped`

### Requirement: Resolve groups
For each group name the tool SHALL look it up with `GET /core/groups/?name=<name>` (exact match) and create it with `POST /core/groups/` when missing. It SHALL remember resolved names for the rest of the run, so each group is looked up and created at most once per run.

#### Scenario: Existing and missing groups
- **WHEN** a row lists `site-a-users` (exists) and `site-b-users` (missing)
- **THEN** the user is created with the existing group's UUID and the newly created group's UUID

#### Scenario: Same group on several rows
- **WHEN** two new users both list group `team`, which does not exist
- **THEN** `team` is looked up once and created once, and both users get its UUID

### Requirement: Error handling
A failure on one row SHALL NOT stop the other rows. A non-2xx API response SHALL be returned as an `APIError` with the method, path, status code, and response body (trimmed, at most 4 KiB). A failed row SHALL be reported as `failed` with the error and without a generated password. When the user was created but setting the password failed, the error SHALL wrap `ErrPasswordNotSet`. The tool SHALL exit with code 1 when any row failed, and 0 otherwise.

#### Scenario: Create fails on every row
- **WHEN** `POST /core/users/` returns 400 for rows `alice` and `bob`
- **THEN** both rows are reported as `failed` with `create user: POST /core/users/: HTTP 400: <body>`, and the tool exits with code 1

#### Scenario: Password not set
- **WHEN** the user is created but `set_password` returns 400
- **THEN** the row is reported as `failed` with an error matching `ErrPasswordNotSet` and the underlying `APIError`

#### Scenario: Status code boundary
- **WHEN** the API responds with status 299
- **THEN** the request succeeds
- **WHEN** the API responds with status 300
- **THEN** the request fails with an `APIError` whose status code is 300

### Requirement: Result table
After processing all rows the tool SHALL print a table to stdout with columns `USERNAME`, `STATUS`, `PASSWORD`, and `NOTE`, one line per row in CSV order. `PASSWORD` SHALL be the generated password, `(from CSV)` for a created user whose password came from the CSV, or `-` otherwise. `NOTE` SHALL be `already exists` for skipped rows and the error message for failed rows.

#### Scenario: Mixed results
- **WHEN** the results are: created with generated `GENPW`, created with CSV password, skipped, failed with `boom`
- **THEN** the table lines show `GENPW`, `(from CSV)`, `-` with `already exists`, and `-` with `boom`

### Requirement: Keep import data out of git
The repository SHALL ignore `tools/create-users/users.csv` and `.env`, and SHALL ship `users.example.csv` and `env.example` as templates. `make create-users` SHALL run the tool with `users.csv`, and `make test` SHALL run the tool's tests.

#### Scenario: Import file ignored
- **WHEN** `tools/create-users/users.csv` exists
- **THEN** `git check-ignore` reports it as ignored
