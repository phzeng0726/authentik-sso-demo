## 1. site-a 後端：設定

- [x] 1.1 加入依賴：`go-oidc/v3`、`x/oauth2`、`godotenv`，以及 `go.uber.org/mock`（`mockgen` 以 `go get -tool` 加入）
- [x] 1.2 擴充 `internal/config`：新增 `OIDC_ISSUER`、`OIDC_REDIRECT_URL`（含 site-a 預設值）、`OIDC_CLIENT_ID`、`OIDC_CLIENT_SECRET`（必填，缺少時回傳 `ErrMissingConfig`）
- [x] 1.3 更新 config 測試：全部設定、只設必填、client ID 缺少、client secret 空字串，並保留原有 port 測試
- [x] 1.4 新增 `env.example`，列出所有變數與預設值，client ID / secret 留空

## 2. site-a 後端：auth 套件

- [x] 2.1 實作 `internal/auth/store.go`：session 與 pending login 的 in-memory store，可注入時間，session 8 小時、pending login 10 分鐘過期
- [x] 2.2 撰寫 store 測試：建立後讀回逐欄位比對、刪除、剛好到期與超過到期的邊界、不存在的 ID
- [x] 2.3 實作 `internal/auth/provider.go`：`Provider` 介面與 go-oidc / oauth2 實作（授權網址含 PKCE 與 nonce、換 token、驗證 ID token、取出 claims、組 end-session URL）
- [x] 2.4 以 `mockgen` 產生 `Provider` 的 mock

## 3. site-a 後端：handler 與 router

- [x] 3.1 實作 `RequireAuth` middleware 與測試：無 cookie、未知 session、過期 session 回 401；有效 session 將使用者傳給 handler
- [x] 3.2 實作 `GET /api/auth/login` 與測試：302 的 Location 參數逐一驗證、login-flow cookie 屬性、pending login 已寫入 store
- [x] 3.3 實作 `GET /api/auth/callback` 與測試：成功、state 不符、缺少或過期的 pending login、authentik 回傳 error、換 token 失敗、nonce 不符；驗證 redirect 目標與 session cookie 是否設定
- [x] 3.4 實作 `GET /api/me` 與測試：已登入回傳 claims 逐欄位比對、未登入回 401
- [x] 3.5 實作 `POST /api/auth/logout` 與測試：有 session（刪除 session、清 cookie、`logout_url` 含 `id_token_hint`）、無 session、`GET` 回 404
- [x] 3.6 更新 router：公開路由與需登入路由分組，cookie 名稱為 `site_a_session`
- [x] 3.7 更新 `main.go`：載入 `.env`、config 失敗或 OIDC discovery 失敗時以非零狀態結束
- [x] 3.8 `go vet ./...` 與 `go test ./...` 全數通過

## 4. site-a 前端

- [x] 4.1 首頁呼叫 `/api/me`：未登入顯示「登入」按鈕；已登入顯示 name、username、email 與「登出」按鈕
- [x] 4.2 實作登入（整頁導向 `/api/auth/login`）與登出（`POST /api/auth/logout` 後導向 `logout_url`）
- [x] 4.3 顯示 `auth_error` 錯誤訊息並從網址移除參數
- [x] 4.4 `pnpm build` 與 `pnpm lint` 通過

## 5. site-b

- [x] 5.1 將 site-a 後端的變更同步到 site-b：預設 issuer、redirect URL 改為 site-b，cookie 名稱為 `site_b_session`，測試同步調整並通過
- [x] 5.2 將 site-a 前端的變更同步到 site-b（保留 `emerald` 主色），`pnpm build` 與 `pnpm lint` 通過

## 6. 工具與文件

- [x] 6.1 `Makefile` 新增 `authentik-check`：檢查兩個網站的 discovery 與 JWKS 端點可正常讀取（recipe 須相容 cmd.exe / sh，無法用 `grep` 比對 issuer 值）
- [x] 6.2 `README.md` 新增 authentik 手動設定步驟（每個設定的用途），並標註各步驟可自動化的方式；說明 `.env` 設定、登入 / 登出行為與 SSO 示範步驟
- [x] 6.3 更新 `CLAUDE.md` 的 Architecture（auth 套件、session cookie 名稱、`env.example`）

## 7. 端對端驗證（與使用者一起進行）

- [x] 7.1 使用者將兩個後端的 `env.example` 複製為 `.env` 並填入 client ID / secret
- [x] 7.2 `make authentik-up` 後 `make dev`，驗證：site-a 登入需輸入帳密 → site-b 按「登入」不需輸入帳密即登入 → site-a 登出後 site-b 仍為登入狀態

## 8. 靜默登入

- [x] 8.1 site-a 後端：`Provider.AuthCodeURL` 新增 silent 參數、`PendingLogin` 記錄 `Silent`、`/api/auth/login` 接受 `prompt=none`（其他值忽略），重新產生 mock
- [x] 8.2 site-a 後端：callback 對靜默嘗試的 `login_required`、`consent_required`、`interaction_required` 導回 `/`；補齊 provider、store、handler 測試（含靜默時其他錯誤、非靜默時 `login_required` 仍為 `access_denied`）
- [x] 8.3 site-a 前端：未登入時自動靜默登入（sessionStorage 標記、`auth_error` 時不嘗試、登出時設標記、`/api/me` 200 時清除），`pnpm build` 與 `pnpm lint` 通過
- [x] 8.4 同步到 site-b，兩站後端測試、前端 build 與 lint 通過
- [x] 8.5 更新 README 的登入流程與 SSO 示範步驟、CLAUDE.md
- [x] 8.6 端對端驗證（與使用者一起進行）：site-a 登入需輸入帳密 → 開 site-b 不需按任何按鈕即自動登入 → site-a 登出後 site-b 仍為登入狀態 → 回到 site-a 不會自動登入 → 未登入時開新分頁只嘗試一次靜默登入、不會無限轉址
