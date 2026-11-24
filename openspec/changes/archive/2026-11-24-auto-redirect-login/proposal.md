## Why

目前網站上有「登入」按鈕，為了讓已在另一站登入的使用者不必按按鈕，又多做了一層靜默登入（`prompt=none` 加上 `sessionStorage` 記號），流程繞、程式碼也多。登出後則停在 authentik 的「已登出此應用程式」頁面，使用者不知道下一步該做什麼。

比較合理的流程是：網站本身不提供登入入口，未登入就直接交給 authentik 處理，authentik 已登入就直接放行，未登入就顯示 authentik 登入頁，登入後回到原本的網站；登出後也回到同樣的流程。

## What Changes

- **BREAKING** 前端移除「登入」按鈕與靜默登入：`GET /api/me` 回 401 時直接整頁導向 `/api/auth/login`（不帶 `prompt`）。authentik 已有 session 時直接放行（SSO），否則顯示 authentik 登入頁；登入後依各站的 `redirect_uri` 回到原站
- **BREAKING** 後端 `GET /api/auth/login` 不再支援 `prompt=none`；callback 移除對靜默登入錯誤的特殊處理
- 移除前端的 `silentLoginTried`（`sessionStorage`）記號與「檢查登入狀態中…」狀態
- 防止無限導向：網址帶 `auth_error` 時不自動導向，改顯示錯誤訊息與「重新登入」按鈕
- 登出：後端回傳的 `logout_url` 在有 `id_token_hint` 時加上 `post_logout_redirect_uri`（本站首頁），由新的環境變數 `OIDC_POST_LOGOUT_REDIRECT_URL` 設定
- authentik 設定：兩個 Provider 的 Invalidation flow 改為 `default-invalidation-flow`（結束 authentik session），並登記 post-logout redirect URI；README 補上步驟與可自動化方式
- `post_logout_redirect_uri` 在 authentik 2026.8.6 是否生效需實測，不支援時的退路寫在 design
- site-a 與 site-b 都要修改

### Non-goals

- 不做全站同步登出（back-channel / front-channel logout）：登出 A 站後，B 站的本地 session 仍有效
- 不做「登入後回到原本的子頁面」：兩站目前都只有首頁，登入後一律回到 `/`
- 不做 authentik blueprint 自動化設定（使用者手動設定以便學習）

## Capabilities

### New Capabilities

（無）

### Modified Capabilities

- `oidc-auth`：「Start login」移除 `prompt=none`；「Handle login callback」移除靜默登入的例外；「Logout」的 `logout_url` 加上 `post_logout_redirect_uri`
- `demo-site-frontend`：「Show login state」不再顯示「登入」按鈕；移除「Login action」與「Automatic silent login」，新增「Automatic login redirect」；「Logout action」不再設定靜默登入記號；「Show login errors」加上「重新登入」按鈕
- `demo-site-backend`：「Environment-based configuration」新增 `OIDC_POST_LOGOUT_REDIRECT_URL`

## Impact

- `site-a/backend`、`site-b/backend`：`internal/config`（新變數）、`internal/auth`（`AuthCodeURL` 移除 silent 參數、`Store` 移除 pending login 的 `Silent`、`EndSessionURL` 加上 post-logout redirect）、`internal/handler`（login、callback、logout）、mock 重新產生、相關測試、`env.example`
- `site-a/frontend`、`site-b/frontend`：`src/App.tsx`
- authentik：兩個 Provider 的 Invalidation flow 與 redirect URI 設定（手動）
- `README.md`、`CLAUDE.md`（移除 silent login 的說明）
