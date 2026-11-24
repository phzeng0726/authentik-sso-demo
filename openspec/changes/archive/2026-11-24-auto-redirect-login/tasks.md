## 1. site-a 後端：設定

- [x] 1.1 `internal/config` 新增 `OIDC_POST_LOGOUT_REDIRECT_URL`（預設 `http://localhost:3001/`）
- [x] 1.2 更新 config 測試：全部設定、未設定時的預設值包含新變數
- [x] 1.3 `env.example` 加上 `OIDC_POST_LOGOUT_REDIRECT_URL`

## 2. site-a 後端：auth 套件

- [x] 2.1 `Provider.AuthCodeURL` 移除 `silent` 參數；`Store.CreatePendingLogin` 與 `PendingLogin` 移除 `Silent`，並更新 store 測試
- [x] 2.2 `BuildEndSessionURL` 加上 `postLogoutRedirect` 參數：有 hint 時加上 `id_token_hint` 與 `post_logout_redirect_uri`，沒有 hint 時回傳原始 endpoint；`OIDCProvider` 從 config 取得此網址
- [x] 2.3 `BuildEndSessionURL` 測試：有 hint、沒有 hint、endpoint 原本就帶 query
- [x] 2.4 以 `go generate ./internal/auth/` 重新產生 mock

## 3. site-a 後端：handler

- [x] 3.1 `Login` 不再讀取 `prompt`；測試改為「`?prompt=none` 被忽略，Location 不含 `prompt`」，刪除靜默登入的測試
- [x] 3.2 `Callback` 移除 `silentNotSignedInErrors` 與 silent 例外；測試改為「`error=login_required` 導向 `/?auth_error=access_denied`」，刪除靜默登入相關測試
- [x] 3.3 `Logout` 測試：有 session 時 `logout_url` 含 `id_token_hint` 與 `post_logout_redirect_uri`（mock 回傳值逐字比對）；無 session 時兩者皆無
- [x] 3.4 `go vet ./...` 與 `go test ./...` 全數通過

## 4. site-a 前端

- [x] 4.1 移除 `silentLoginTried` 相關函式、`silent-login` 狀態與「登入」按鈕
- [x] 4.2 `/api/me` 回 401 且沒有 `auth_error` 時，顯示「前往登入頁…」並整頁導向 `/api/auth/login`
- [x] 4.3 有 `auth_error` 且 401 時，顯示錯誤與「重新登入」按鈕（導向 `/api/auth/login`）
- [x] 4.4 登出不再寫入 `sessionStorage`；失敗時仍顯示「登出失敗，請稍後再試。」
- [x] 4.5 `pnpm lint` 與 `pnpm build` 通過

## 5. 同步到 site-b

- [x] 5.1 依 CLAUDE.md 的對照規則把後端變更同步到 site-b（預設值 `http://localhost:3002/`）
- [x] 5.2 同步前端變更（`blue` → `emerald`）
- [x] 5.3 site-b 的 `go vet`、`go test`、`pnpm lint`、`pnpm build` 通過

## 6. authentik 設定與實測（手動）

- [x] 6.1 兩個 Provider 的 Invalidation flow 改為 `default-invalidation-flow`
- [x] 6.2 兩個 Provider 加上 post-logout redirect URI（`http://localhost:3001/`、`http://localhost:3002/`）
- [x] 6.3 實測：未登入開 A 站 → authentik 登入頁 → 登入後回到 A 站；再開 B 站不需輸入帳密即登入
- [x] 6.4 實測：從 B 站開始登入，登入後回到 B 站
- [x] 6.5 實測登出：回到原站並顯示 authentik 登入頁，登入後回到原站；若 `post_logout_redirect_uri` 不生效，依 design 決策 6 採用退路
- [x] 6.6 實測錯誤：讓 callback 失敗（例如清掉 `site_a_login` cookie 後完成登入），確認顯示錯誤與「重新登入」按鈕、沒有無限重導

## 7. 文件

- [x] 7.1 README：更新登入/登出流程與 API 表格、新增環境變數、authentik 設定步驟（Invalidation flow、post-logout redirect URI）並標註可自動化方式（API `PATCH /api/v3/providers/oauth2/{pk}/`、blueprint）、實測結果與已知限制（B 站不會同步登出）
- [x] 7.2 CLAUDE.md：移除 silent login 的說明，改寫為自動導向登入與 `auth_error` 停止條件，並列入 `OIDC_POST_LOGOUT_REDIRECT_URL`
