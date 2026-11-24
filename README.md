# authentik-sso-demo
讓兩個不同網站透過 authentik 實現單點登入

## 目錄結構

```
.
├── authentik/        # authentik（server、worker、PostgreSQL），以 Docker Compose 執行
├── site-a/
│   ├── backend/      # Go + Gin API
│   └── frontend/     # React + TypeScript + Vite + Tailwind CSS
├── site-b/           # 與 site-a 結構相同的另一個網站
└── openspec/         # OpenSpec 規格與變更提案
```

## Port 配置

| 服務 | Port |
|------|------|
| authentik | 9000（HTTP）/ 9443（HTTPS） |
| site-a backend | 8081 |
| site-a frontend | 3001 |
| site-b backend | 8082 |
| site-b frontend | 3002 |

## 環境需求

- Go 1.26+
- Node.js 24+、pnpm 11+
- Docker（執行 authentik 用）
- GNU Make（PowerShell 與 Git Bash 皆可執行）

## 快捷指令

在專案根目錄執行，`make help` 可列出所有指令。

| 指令 | 說明 |
|------|------|
| `make install` | 安裝兩個網站的前端依賴 |
| `make dev` | 同時啟動 site-a、site-b 的前後端 |
| `make site-a` / `make site-b` | 只啟動單一網站的前後端 |
| `make site-a-backend` 等 | 只啟動單一服務（`site-{a,b}-{backend,frontend}`） |
| `make authentik-up` / `make authentik-down` | 背景啟動 / 停止 authentik |
| `make authentik-logs` | 追蹤 authentik log |
| `make authentik-check` | 檢查兩個網站在 authentik 的 OIDC discovery 與 JWKS 端點是否正常 |
| `make test` | 執行兩個網站的後端單元測試 |

按 `Ctrl+C` 停止前景執行的服務。

## 啟動網站

第一次執行前，先完成下方「authentik 設定」並建立兩個後端的 `.env`。

```bash
make install          # 第一次執行前
make authentik-up     # 後端啟動時會連線 authentik，必須先啟動
make authentik-check  # 選用：確認 authentik 設定正常
make dev
```

開啟 http://localhost:3001（site-a）與 http://localhost:3002（site-b）。前端的 `/api` 請求會由 Vite dev server 轉發到對應的後端。後端首次 `go run` 需要編譯，期間前端會短暫出現 proxy error，屬正常現象。

後端啟動失敗時，log 會說明原因：
- `missing required config: OIDC_CLIENT_ID is not set`：尚未建立 `.env` 或未填 client ID / secret
- `OIDC discovery failed`：authentik 沒有啟動，或 `OIDC_ISSUER` 的 slug 不正確

不使用 make 時，可在各網站的 `backend/` 執行 `go run .`、`frontend/` 執行 `pnpm dev`。

### 後端環境變數

後端啟動時會讀取所在目錄的 `.env`（不存在則略過），已存在的環境變數優先。將各後端的 `env.example` 複製為 `.env` 後填入 client ID / secret；`.env` 已被 gitignore，請勿提交。

| 變數 | 說明 | site-a 預設 | site-b 預設 |
|------|------|-------------|-------------|
| `PORT` | 監聽 port，須為 1–65535 的整數 | `8081` | `8082` |
| `SITE_NAME` | 網站名稱，由 `GET /api/info` 回傳 | `Site A` | `Site B` |
| `OIDC_ISSUER` | authentik 應用程式的 issuer | `http://localhost:9000/application/o/site-a/` | `http://localhost:9000/application/o/site-b/` |
| `OIDC_REDIRECT_URL` | 登入完成後的 callback，須與 authentik 設定一致 | `http://localhost:3001/api/auth/callback` | `http://localhost:3002/api/auth/callback` |
| `OIDC_CLIENT_ID` | **必填**，authentik provider 的 Client ID | — | — |
| `OIDC_CLIENT_SECRET` | **必填**，authentik provider 的 Client Secret | — | — |

### 後端 API

| Method | Path | 需登入 | 回應 |
|--------|------|--------|------|
| GET | `/api/health` | 否 | `{"status":"ok"}` |
| GET | `/api/info` | 否 | `{"name":"<SITE_NAME>"}` |
| GET | `/api/auth/login` | 否 | 302 導向 authentik 登入 |
| GET | `/api/auth/callback` | 否 | 登入完成後由 authentik 導回；成功 302 到 `/`，失敗 302 到 `/?auth_error=<code>` |
| POST | `/api/auth/logout` | 否 | 清除本站 session，回傳 `{"logout_url":"..."}` |
| GET | `/api/me` | 是 | `{"sub","username","name","email"}`；未登入回 401 `{"error":"unauthorized"}` |

## 登入流程

兩個網站都使用 OIDC Authorization Code Flow，由後端處理（BFF 模式），token 不會出現在瀏覽器的 JavaScript 中：

1. 前端按「登入」→ 整頁導向後端 `/api/auth/login`。
2. 後端產生 `state`（防 CSRF）、`nonce`（防 ID token 重放）、PKCE verifier（防授權碼被攔截後兌換），存在 server 端，再導向 authentik。
3. 使用者在 authentik 登入後，authentik 帶著授權碼導回 `/api/auth/callback`。
4. 後端比對 `state`、用 PKCE verifier 換 token、驗證 ID token（簽章、issuer、audience、到期時間、nonce），建立 session 並設定 cookie。

Session 存在後端記憶體，8 小時到期，後端重啟會清空。兩個網站的 cookie 名稱不同（`site_a_session`、`site_b_session`），因為同一個 `localhost` 不同 port 會共用 cookie。

## SSO 示範

1. 開啟 site-a（http://localhost:3001），按「登入」，在 authentik 輸入帳密 → 回到 site-a 顯示使用者資訊。
2. 開啟 site-b（http://localhost:3002），按「登入」→ **不需輸入帳密**，直接回到 site-b 並顯示使用者資訊。這是因為瀏覽器在 authentik 已有登入 session。
3. 在 site-a 按「登出」→ site-a 的 session 被清除，並導向 authentik 結束 authentik 的登入 session。
4. 回到 site-b 重新整理 → **仍是登入狀態**。登出只會清除當前網站的 session，不會同步到另一個網站；但 authentik 的 session 已結束，site-b 下次登入需要重新輸入帳密。

## 測試

```bash
make test
```

單元測試撰寫規範見 [TESTING_GUIDELINES.md](TESTING_GUIDELINES.md)。

## authentik 設定

以下為手動設定步驟，每一步都標註了之後可以如何自動化。

### 1. 啟動並建立管理員

```bash
make authentik-up
```

開啟 http://localhost:9000/if/flow/initial-setup/ 建立管理員 `akadmin`。

> 可自動化：在 `authentik/.env` 設定 `AUTHENTIK_BOOTSTRAP_PASSWORD`（與 `AUTHENTIK_BOOTSTRAP_EMAIL`），首次啟動時會自動建立管理員。

### 2. 建立測試使用者

**Directory → Users** 建立一個測試帳號，用來登入兩個網站。

> 可自動化：authentik blueprint 定義使用者，或呼叫 authentik REST API。

### 3. 為每個網站建立 Application 與 Provider

**Applications → Applications → Create with Provider**，site-a、site-b 各做一次：

| 欄位 | site-a | site-b | 用途 |
|------|--------|--------|------|
| Application name / Slug | `Site A` / `site-a` | `Site B` / `site-b` | slug 決定 issuer 網址 `/application/o/<slug>/` |
| Provider type | OAuth2/OpenID Connect | 同左 | 網站以 OIDC 登入 |
| Authorization flow | `default-provider-authorization-implicit-consent` | 同左 | 登入後不顯示授權確認頁，第二個網站可直接登入 |
| Client type | Confidential | 同左 | 後端持有 client secret 換 token |
| Redirect URI（Strict） | `http://localhost:3001/api/auth/callback` | `http://localhost:3002/api/auth/callback` | 經前端 port 走 Vite proxy，session cookie 落在網站自己的網域 |
| Signing key | `authentik Self-signed Certificate` | 同左 | ID token 以 RS256 簽章，後端用公開金鑰（JWKS）驗證 |
| Scopes | `openid`、`email`、`profile` | 同左 | 取得使用者帳號、名稱、email |

建立後將 **Client ID**、**Client Secret** 填入對應後端的 `.env`。

> 可自動化：authentik blueprint 定義 provider 與 application；client secret 可從環境變數讀取，與網站的 `.env` 共用同一份設定，就不需手動複製。需要在 `authentik/docker-compose.yml` 額外掛載 blueprints 資料夾。

### 4. 存取控制（選用）

應用程式未綁定 policy 時，所有使用者都能登入。要限制特定使用者或群組，在應用程式的 **Policy / Group / User Bindings** 綁定。

> 可自動化：blueprint 定義 policy binding。

### 5. 確認設定

```bash
make authentik-check
```

或在瀏覽器開啟 `http://localhost:9000/application/o/site-a/.well-known/openid-configuration`，確認 `issuer` 為 `http://localhost:9000/application/o/site-a/`。

> 可自動化：登入流程的端對端驗證（上方「SSO 示範」）可用 Playwright 等瀏覽器自動化工具做成測試。
