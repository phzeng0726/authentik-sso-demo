## Why

`site-a`、`site-b` 兩個網站骨架已完成，authentik 也已手動設定好兩個 OIDC 應用程式（slug `site-a`、`site-b`）。現在要讓兩個網站透過 authentik 登入，示範 SSO：在其中一個網站登入後，另一個網站不需再輸入帳密即可登入。

## What Changes

- 兩個網站的後端加入 OIDC Authorization Code Flow（搭配 PKCE、state、nonce）：
  - `GET /api/auth/login`：導向 authentik 登入
  - `GET /api/auth/callback`：接收授權碼、換取並驗證 ID token、建立 session
  - `GET /api/me`：回傳目前登入的使用者資訊，未登入回 401
  - `POST /api/auth/logout`：清除本站 session，並回傳 authentik 的登出網址
- 後端以 server-side session 保存登入狀態，兩個網站使用不同名稱的 session cookie，避免在 `localhost` 上互相覆蓋
- 新增需要登入才能存取的 API 保護機制（middleware），`/api/health`、`/api/info` 維持公開
- 後端新增 OIDC 相關環境變數（issuer、client ID、client secret、redirect URL），並支援從後端目錄的 `.env` 檔載入；提供不含真實密鑰的範例檔
- 前端首頁依登入狀態顯示「登入」按鈕，或使用者資訊與「登出」按鈕；登入失敗時顯示錯誤訊息
- `README.md` 補上 authentik 手動設定步驟，並標註哪些步驟之後可以自動化
- `Makefile` 新增 `authentik-check`，檢查兩個網站的 OIDC discovery 是否正常
- 後端單元測試遵循 `TESTING_GUIDELINES.md`

### Non-goals

- 不做全站同步登出（back-channel / front-channel logout）：登出一個網站後，另一個網站的本地 session 仍有效
- 不做 authentik blueprint 自動化設定（使用者選擇手動設定以便學習）
- 不做 refresh token、session 持久化（後端重啟後需重新登入，透過 SSO 只需點一次登入）
- 不做角色 / 權限控管

## Capabilities

### New Capabilities

- `oidc-auth`: 後端的 OIDC 登入、callback、session、`/api/me`、登出與登入保護 middleware

### Modified Capabilities

- `demo-site-backend`: 環境變數設定新增 OIDC 相關變數與必填檢查
- `demo-site-frontend`: 首頁新增登入狀態顯示、登入與登出操作

## Impact

- 修改：`site-a/backend`、`site-b/backend`（config、handler、router、main）；`site-a/frontend`、`site-b/frontend`（`App.tsx`）
- 新增依賴（Go）：`github.com/coreos/go-oidc/v3`、`golang.org/x/oauth2`、`github.com/joho/godotenv`、`go.uber.org/mock`（測試用）
- 新增各後端的環境變數範例檔；實際含 client secret 的 `.env` 已被 `.gitignore` 排除
- 後端啟動時需要 authentik 正在執行（啟動時讀取 OIDC discovery）
- 更新 `README.md`、`Makefile`、`CLAUDE.md`
