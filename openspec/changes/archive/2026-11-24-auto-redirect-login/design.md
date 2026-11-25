## Context

`add-authentik-sso` 完成後，兩個網站的登入流程是：

- 首頁呼叫 `/api/me`，401 時先以 `prompt=none` 做一次靜默登入（每個分頁一次，記號存在 `sessionStorage` 的 `silentLoginTried`）；失敗才顯示「登入」按鈕
- 後端 `Login` 支援 `?prompt=none`，pending login 帶 `Silent` 旗標；callback 收到靜默登入的 `login_required` 等錯誤時安靜地回到 `/`
- 登出：後端清除本站 session，回傳帶 `id_token_hint` 的 end-session URL；Provider 使用預設的 `default-provider-invalidation-flow`，只顯示「已登出此應用程式」頁面，**不會結束 authentik session**

authentik 版本為 2026.8.6（`authentik/docker-compose.yml`）。依官方文件，Invalidation flow 決定登出時做什麼：

| Flow | 行為 |
|---|---|
| `default-provider-invalidation-flow` | 只結束該應用程式的 session（新 Provider 的預設值） |
| `default-invalidation-flow` | 含 User Logout stage，結束 authentik session |

文件沒有說明 end-session 端點是否支援 `post_logout_redirect_uri`，GitHub issue 上兩種回報都有（例如 goauthentik/authentik#10430、#24012），因此需要實測。

## Goals / Non-Goals

**Goals:**
- 網站不提供登入入口；未登入時自動交給 authentik，登入後回到原站
- 移除靜默登入，讓前後端的登入流程只剩一條路
- 登出後回到原站，再自動導向 authentik 登入頁；登入後回到原站
- 不會出現登入失敗後無限重導

**Non-Goals:**
- 全站同步登出、登入後回到原本的子頁面、blueprint 自動化（見 proposal）

## Decisions

### 1. 401 時直接發一般授權請求，不再使用 `prompt=none`

前端 `/api/me` 回 401 就整頁導向 `/api/auth/login`。不帶 `prompt` 的授權請求，authentik 在已有 session 時會直接帶 code 導回（Provider 使用 implicit consent，不會出現同意頁），沒有 session 時顯示登入頁。

- **理由**：一般請求已經涵蓋「已登入就放行」與「未登入就登入」兩種情況，靜默登入原本只是為了在「有登入按鈕」的設計下避免直接跳去 authentik。按鈕拿掉後，靜默登入就沒有存在的理由。
- **替代方案**：保留 `prompt=none`，失敗後再自動發一般請求。多一次往返、多一組狀態，結果完全相同。

### 2. 「回到原站」由 `redirect_uri` 保證，不另外記錄來源

每站的後端只用自己的 `OIDC_REDIRECT_URL`（`http://localhost:3001/api/auth/callback` 或 `:3002`）發請求，authentik 也只接受該 Provider 登記的 redirect URI，所以從 A 站開始的登入一定回到 A 站。callback 成功後一律導向 `/`。

- **替代方案**：記錄登入前的路徑（`return_to`），登入後導回該頁。兩站目前只有首頁，用不到；之後需要時要注意 open redirect，只接受站內相對路徑。

### 3. 以 `auth_error` 作為停止自動導向的條件

callback 的所有失敗都會導向 `/?auth_error=<code>`。前端在模組載入時讀取並移除這個參數（沿用現有的 `takeAuthError`），有錯誤時不自動導向，改顯示錯誤與「重新登入」按鈕。重新整理頁面時參數已經不在網址上，會再次自動導向，相當於重試。

- **理由**：登入失敗（例如使用者沒有該應用程式的權限，authentik 回 `access_denied`）時，自動重導會在網站與 authentik 之間無限跳轉。`auth_error` 已存在，不需要額外狀態。
- **替代方案**：用 `sessionStorage` 記錄重導次數。又回到原本想移除的記號，而且所有失敗都已經會帶 `auth_error`。

### 4. 後端完全移除靜默登入

`Provider.AuthCodeURL` 移除 `silent` 參數，`Store.CreatePendingLogin` 與 `PendingLogin` 移除 `Silent`，callback 移除 `silentNotSignedInErrors` 的例外處理；`?prompt=` 參數直接忽略。mock 以 `go generate ./internal/auth/` 重新產生。

- **理由**：前端不再使用，保留只會讓程式碼和測試難以閱讀。
- **替代方案**：保留後端支援供日後使用。沒有使用者的功能不值得維護。

### 5. 登出時加上 `post_logout_redirect_uri`，並改用 `default-invalidation-flow`

- 新增設定 `OIDC_POST_LOGOUT_REDIRECT_URL`（預設 `http://localhost:3001/`、`http://localhost:3002/`），由 `config` 傳入 `OIDCProvider`
- `EndSessionURL(idTokenHint)` 在有 hint 時同時加上 `id_token_hint` 與 `post_logout_redirect_uri`；沒有 hint 時回傳純 end-session URL（有回報指出沒有 `id_token_hint` 卻帶 `post_logout_redirect_uri` 會得到 400）
- `BuildEndSessionURL(endpoint, hint, postLogoutRedirect string)` 維持純函式，方便單元測試
- authentik 手動設定：兩個 Provider 的 Invalidation flow 改為 `default-invalidation-flow`；在 Redirect URIs 加上 `http://localhost:3001/`（site-b：`:3002/`），類型選 post-logout / logout（若該版本有此欄位），比對方式 Strict

預期流程：按「登出」→ 清除本站 session → authentik 結束 session → 導回本站 `/` → `/api/me` 401 → 導向 `/api/auth/login` → authentik 登入頁 → 登入後回到本站。

- **替代方案**：後端登出後直接導回 `/`，不經過 authentik。authentik session 仍在，馬上又被自動登入，等於沒登出。

### 6. `post_logout_redirect_uri` 不生效時的退路

實作完成後先實測。若 authentik 2026.8.6 忽略此參數，依序考慮：

1. **自訂 Invalidation flow**：為每站建立一個 flow（例如 `site-a-invalidation`），依序綁定 User Logout stage 與 Redirect stage（Static 模式，目標 `http://localhost:3001/`），再設為該 Provider 的 Invalidation flow。需確認 2026.8.6 有 Redirect stage；兩站各需一個 flow
2. **接受停在 authentik 登入頁**：只使用 `default-invalidation-flow`。登出後顯示 authentik 登入頁，但登入後會進到 authentik 使用者首頁，不會回到原站；README 註明此限制

不論採用哪個退路，後端仍保留 `post_logout_redirect_uri`（它是標準參數，對 authentik 無害），並把實測結果寫進 README。

## Risks / Trade-offs

- [callback 成功但瀏覽器沒有存下 session cookie（例如被瀏覽器阻擋），`/api/me` 一直回 401] → 會在網站與 authentik 之間無限跳轉。本機 demo 的 cookie 是同源、`SameSite=Lax`，風險低；不另外加防護，於 README 的疑難排解提及
- [登出 A 站後，B 站的本地 session 仍有效] → 即使 authentik session 已結束，B 站在 8 小時內仍顯示已登入。屬 Non-goal，README 註明，之後可用 back-channel logout 處理
- [使用者在 authentik 登出頁面中途離開，authentik session 沒有結束] → 回到網站會被自動登入。這符合「authentik 有 session 就是已登入」的新模型，可接受
- [`post_logout_redirect_uri` 不被支援] → 見決策 6。實測 authentik 2026.8.6 有支援，不需要退路
- [authentik 2026.8.x 的 end-session 遇到未完成的 flow（例如另一站停在登入頁）時回應空白 `200`，不導回網站（goauthentik/authentik#25545）] → 本站 session 已先清除，實際上已登出；README 記為已知問題，等 2026.11.0 升級。不在程式中繞過，因為可行的做法都依賴 authentik 內部行為
- [移除 `?prompt=none` 屬於 API 行為變更] → 只有本專案前端使用，一起修改即可

## Migration Plan

1. 先改 site-a（後端 → 前端），測試通過後依 CLAUDE.md 的對照規則同步到 site-b
2. 在 authentik 調整兩個 Provider 的 Invalidation flow 與 redirect URI
3. 實測登出流程，依結果決定是否需要決策 6 的退路，並更新 README
4. 回滾：還原程式碼並把 Invalidation flow 改回 `default-provider-invalidation-flow`；多登記的 redirect URI 留著不影響

## Open Questions

- authentik 2026.8.6 的 end-session 是否支援 `post_logout_redirect_uri`？Redirect URI 是否需要標示為 logout 類型？（實作時實測）
