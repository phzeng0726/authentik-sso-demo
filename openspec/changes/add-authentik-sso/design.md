## Context

`site-a`、`site-b` 是結構相同、程式碼各自獨立的網站（Go + Gin 後端、React + Vite 前端），前端透過 Vite proxy 以同源方式呼叫 `/api`。authentik 已手動建立兩個 OAuth2/OpenID Connect 應用程式：

| | site-a | site-b |
|---|---|---|
| Issuer | `http://localhost:9000/application/o/site-a/` | `http://localhost:9000/application/o/site-b/` |
| Redirect URI | `http://localhost:3001/api/auth/callback` | `http://localhost:3002/api/auth/callback` |
| Client type | Confidential | Confidential |
| Authorization flow | implicit consent | implicit consent |
| Signing key | authentik Self-signed Certificate（RS256） | 同左 |

瀏覽器與後端都以 `localhost:9000` 連線 authentik，因此 ID token 的 issuer 與後端看到的 issuer 一致。

## Goals / Non-Goals

**Goals:**
- 兩個網站各自完成標準的 OIDC Authorization Code Flow，並示範 SSO：在 site-a 登入後，開啟 site-b 即自動登入，不需按「登入」也不需輸入帳密（決策 10）
- 每個安全相關的步驟（state、nonce、PKCE、ID token 驗證、cookie 屬性）都有明確的實作與測試，方便對照學習
- 後端單元測試遵循 `TESTING_GUIDELINES.md`

**Non-Goals:**
- 全站同步登出、refresh token、session 持久化、角色權限
- authentik blueprint 自動化

## Decisions

### 1. 由後端處理登入流程（BFF 模式），而非前端 SPA 直接做 OIDC

後端是 Confidential client，持有 client secret，負責導向 authentik、換 token、驗證 token，並以 HttpOnly cookie 維持 session。前端只呼叫 `/api/me`、導向 `/api/auth/login`。

- **理由**：token 不會出現在瀏覽器的 JavaScript 中，降低 XSS 竊取 token 的風險；前端也不需要 OIDC 套件。這是目前 SPA 搭配後端時建議的做法（Backend for Frontend）。
- **替代方案**：前端以 Public client + PKCE 自行做 OIDC（如 `oidc-client-ts`），token 存在瀏覽器。實作分散在前後端，且後端仍需驗證 access token，不符合目前 Confidential client 的設定。

### 2. 使用 `coreos/go-oidc` 與 `golang.org/x/oauth2`

- `go-oidc`：讀取 discovery、取得 JWKS 並驗證 ID token 的簽章、issuer、audience、expiry。
- `x/oauth2`：產生授權網址、以授權碼換 token，並內建 PKCE（`GenerateVerifier`、`S256ChallengeOption`、`VerifierOption`）。
- **理由**：兩者是 Go 生態最常用、維護穩定的 OIDC 組合，且不隱藏流程細節，適合學習每個步驟。
- **替代方案**：`zitadel/oidc` 等完整框架，功能多但封裝較深，不利於理解流程。

### 3. 安全參數：state、nonce、PKCE 都要

| 參數 | 防範的問題 | 驗證位置 |
|------|------------|----------|
| `state` | CSRF：攻擊者讓受害者帶著攻擊者的授權碼完成登入 | callback 比對 query 與 pending login |
| `nonce` | ID token 重放 | 驗證後的 ID token `nonce` claim 與 pending login 比對 |
| PKCE（S256） | 授權碼被攔截後遭他人兌換 | authentik 換 token 時比對 code verifier |

三者在 `/api/auth/login` 以 `crypto/rand` 產生，存在 server-side 的 pending login（10 分鐘過期），瀏覽器只拿到一個隨機 ID 的 login-flow cookie（`Path=/api/auth`、HttpOnly、`SameSite=Lax`）。`SameSite=Lax` 讓 authentik 以頂層導覽（GET）回到 callback 時仍會帶上 cookie。

### 4. Session：server-side in-memory store

- 以 `map` + `sync.Mutex` 實作 store，存放 session（使用者 claims、原始 ID token、建立時間）與 pending login，皆有過期時間；查詢時遇到過期項目即刪除。
- Session ID 為 32 bytes `crypto/rand`，base64url 編碼。
- Cookie 名稱各網站不同：`site_a_session`、`site_b_session`。同一 host 不同 port 的 cookie 會共用，名稱相同會互相覆蓋。
- Cookie 不設 `Secure`：本機開發走 HTTP。正式環境需改為 HTTPS 並設 `Secure`（記於 Risks）。
- **理由**：示範專案不需要資料庫；server-side session 讓登出能真正讓 session 失效。
- **替代方案**：把 claims 加密簽章後直接放 cookie（stateless），不需要 store，但登出只能清 cookie、無法讓已外流的 cookie 失效，且多一組加密金鑰要管理。

### 5. 後端結構

```
backend/
├── main.go                      # 載入 .env、config、OIDC discovery、組裝 router
└── internal/
    ├── config/                  # 新增 OIDC 設定與 ErrMissingConfig
    ├── auth/
    │   ├── provider.go          # Provider 介面 + go-oidc/oauth2 實作
    │   ├── store.go             # Session / pending login 的 in-memory store
    │   └── store_test.go
    └── handler/
        ├── auth.go              # login、callback、logout、me
        ├── auth_test.go
        ├── middleware.go        # RequireAuth
        ├── middleware_test.go
        └── router.go            # 註冊公開與需登入的路由
```

- `auth.Provider` 介面隔離 authentik：`AuthCodeURL(state, nonce, verifier string, silent bool)`、`Exchange(ctx, code, verifier) (IDToken, error)`、`EndSessionURL(idTokenHint)`。`silent` 為 true 時加上 `prompt=none`；`PendingLogin` 以 `Silent` 欄位記錄，供 callback 判斷。handler 測試以 `go.uber.org/mock` 產生的 mock 替代，不需要真的 authentik。
- `auth.Store` 介面讓測試可以注入時間（`now func() time.Time`），驗證 8 小時、10 分鐘的過期邊界。
- `/api/auth/logout` 只註冊 POST：避免第三方網頁用 `<img src=".../logout">` 之類的 GET 請求讓使用者被登出。

### 6. 登出流程

1. 前端 `POST /api/auth/logout`。
2. 後端刪除 session、清除 cookie，回傳 `logout_url`：authentik 的 `end_session_endpoint`，加上 `id_token_hint`。
3. 前端導向 `logout_url`，authentik 結束自己的登入 session 並顯示登出頁面。

另一個網站的本地 session 不受影響（Non-goal）；但因 authentik session 已結束，該網站 session 到期後就需要重新輸入帳密。

### 7. 環境變數與 `.env`

- 使用 `joho/godotenv` 在啟動時載入後端目錄下的 `.env`（不存在時略過），已存在的環境變數優先。
- 範例檔命名為 `env.example`（而非 `.env.example`），內容只有變數名稱與預設值、client ID / secret 留空。改用這個名稱是因為專案的 commit 流程將 `.env*` 一律視為敏感檔案，不會提交。
- 使用者自行複製為 `.env` 並填入 authentik 的 client ID / secret；`.env` 已在 `.gitignore`。

### 8. 啟動時做 OIDC discovery，失敗即結束

- **理由**：設定錯誤（issuer 打錯、authentik 沒開）在啟動時就明確失敗，比等到使用者按登入才出錯容易排查。
- **代價**：`make dev` 前必須先 `make authentik-up`。README 與錯誤訊息會提示。
- **替代方案**：第一次登入時才延遲 discovery。啟動不依賴 authentik，但錯誤延後出現。

### 9. 前端

- 載入時呼叫 `/api/me`，以 401 / 200 決定顯示「登入」或使用者資訊（401 時可能先自動靜默登入，見決策 10）；`/api/info`、`/api/health` 照舊顯示。
- 「登入」以 `window.location.href = '/api/auth/login'` 導向（必須是整頁導覽，fetch 無法跟隨跨站導向到 authentik）。
- 有 `auth_error` 時顯示對應訊息，並以 `history.replaceState` 移除參數，重新整理不會重複顯示。

### 10. 靜默登入（`prompt=none`）

未登入時自動向 authentik 確認登入狀態，已在另一個網站登入的使用者不需按「登入」。

1. 前端 `/api/me` 回 401，且網址沒有 `auth_error`、此分頁尚未嘗試過 → 在 `sessionStorage` 設定標記，整頁導向 `/api/auth/login?prompt=none`，畫面顯示「檢查登入狀態中…」。
2. 後端把 `prompt=none` 帶給 authentik，並在 pending login 記錄 `Silent`。`prompt=none` 要求 authentik 不顯示任何登入或授權畫面。
3. authentik 的回應：
   - **已有 authentik session**：直接帶授權碼導回 callback，照一般流程建立 session。使用者只看到網址快速跳轉。
   - **沒有 authentik session**：導回 callback 並帶 `error=login_required`（OIDC 標準亦定義 `consent_required`、`interaction_required`）。後端判斷這是靜默嘗試，導回 `/` 不帶 `auth_error`，前端因標記已設定而顯示「登入」按鈕。

標記規則：
- **每個分頁只試一次**：防止靜默失敗後 `/api/me` 仍 401 而無限轉址。
- **登出時設定標記**：即使使用者在 authentik 登出頁之前離開、authentik session 仍在，回到網站也不會立刻自動登入。
- **`/api/me` 回 200 時清除**：同一分頁內 session 8 小時到期後，仍能再靜默續登。
- **`auth_error` 時不嘗試**：避免錯誤訊息被立即的轉址蓋掉。

- **為何用 `sessionStorage`**：每個分頁獨立、關閉即清除；不同 port 是不同 origin，site-a 與 site-b 的標記互不影響。
- **為何非靜默登入收到 `login_required` 仍顯示錯誤**：一般登入不會出現這個錯誤，出現即代表異常，應讓使用者看到。
- **替代方案**：隱藏 iframe 做靜默檢查，頁面不跳轉。但 authentik 可能以 `X-Frame-Options` 禁止嵌入，瀏覽器的第三方 cookie 限制也可能讓 iframe 拿不到 authentik session，較易失敗且難除錯。
- **替代方案**：不自動登入、一律按「登入」（原本的做法）。流程最單純，但 SSO 的效果需要多按一次。

## Risks / Trade-offs

- [in-memory session，後端重啟即失效] → 示範用途可接受；因 authentik session 仍在，重新按「登入」即可，不需輸入帳密。
- [Cookie 未設 `Secure`] → 僅限本機 HTTP 開發；部署時需 HTTPS 並開啟 `Secure`。
- [兩網站共用同一把簽章金鑰] → 不影響安全性：token 的 `iss`、`aud` 各自不同，後端只接受自己的 issuer 與 client ID。
- [登出不會同步到另一個網站] → 已列為 Non-goal，README 說明此行為。
- [authentik 的 end-session 是否支援 `post_logout_redirect_uri` 依版本而定] → 本次不依賴它，登出後停在 authentik 的登出頁面。
- [未登入時每個新分頁都會先跳去 authentik 再回來] → 每個分頁只嘗試一次，畫面顯示「檢查登入狀態中…」。
- [authentik 對 `prompt=none` 的實際行為] → 端對端驗證時確認；若回傳的錯誤碼不在預期清單內，會顯示 `access_denied`，方便發現。

## Open Questions

- 登出後是否要自動導回網站首頁：需確認 authentik 2026.8 的 end-session 行為，若支援可之後再加。
