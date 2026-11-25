# authentik SSO 可行性驗證筆記

本筆記整理這個專案從頭到尾的設定、流程與驗證結論。目的是**確認可行性**，不是建置正式環境，所以部分功能只確認「做得到」而沒有實作。

操作細節（指令、環境變數、API）以 [README](../README.md) 為準；本筆記著重在「為什麼這樣設計」與「驗證了什麼」。

## 結論摘要

| 項目 | 結果 |
|------|------|
| 兩個不同網站（site-a、site-b）透過 authentik 單點登入 | ✅ 已實作並驗證 |
| 未登入自動導向 authentik、登出結束 authentik session | ✅ 已實作並驗證 |
| 從 CSV 透過 API 批次建立使用者 | ✅ 已實作（`tools/create-users`） |
| Synology NAS 的 SMB 與 authentik 共用同一組帳密 | ✅ 已驗證（透過 AD，見[第 7 章](#7-synology-nas-整合)） |
| 在 authentik 改密碼，NAS 自動改密碼 | ✅ 已驗證（LDAP Source 密碼寫回） |
| 在 authentik 建立使用者，NAS 自動產生帳號 | ✅ 確認可行（需自行撰寫自動化，未實作） |
| DSM 網頁版透過 OIDC 單點登入 | ✅ 已驗證（見 7.5 ⑥） |

測試版本：authentik **2026.8.6**。

---

## 1. 架構總覽

```
                     ┌──────────────────────────────┐
                     │ authentik（帳號主資料）        │
                     │ server + worker + PostgreSQL │
                     └──────┬───────────────┬───────┘
                 OIDC       │               │  LDAP Source（LDAPS）
          ┌─────────────────┤               │  同步 + 密碼寫回
          │                 │               │
   ┌──────┴──────┐   ┌──────┴──────┐  ┌─────┴──────────────────────┐
   │ site-a      │   │ site-b      │  │ AD（Synology Directory      │
   │ Go + React  │   │ Go + React  │  │ Server），只放要用 NAS 的人  │
   └─────────────┘   └─────────────┘  └─────┬──────────────────────┘
                                            │ 網域帳號
                                      ┌─────┴─────┐
                                      │ NAS（SMB） │
                                      └───────────┘
```

| 元件 | 技術 | Port |
|------|------|------|
| authentik | Docker Compose（server、worker、PostgreSQL 16） | 9000（HTTP）/ 9443（HTTPS） |
| site-a | 後端 Go + Gin / 前端 React + TS + Vite + Tailwind v4 | 後端 8081 / 前端 3001 |
| site-b | 同 site-a（程式碼刻意複製，不共用） | 後端 8082 / 前端 3002 |
| tools/create-users | Go CLI，呼叫 authentik REST API | — |

## 2. authentik 部署

- 以 `authentik/docker-compose.yml` 啟動，設定來自 `authentik/.env`（已 gitignore）。指令：`make authentik-up`、`make authentik-down`、`make authentik-logs`。
- 首次啟動後開 `http://localhost:9000/if/flow/initial-setup/` 建立管理員 `akadmin`。
  - 可自動化：`.env` 設定 `AUTHENTIK_BOOTSTRAP_PASSWORD`、`AUTHENTIK_BOOTSTRAP_EMAIL`。

### 資料存在哪裡

| 位置 | 內容 |
|------|------|
| PostgreSQL（Docker 具名 volume `database`） | **幾乎所有資料**：使用者、群組、密碼雜湊、Provider、Application、Flow、LDAP Source、session、事件紀錄 |
| `authentik/data` | 上傳的 media（logo、背景圖） |
| `authentik/certs` | 自動匯入的憑證 |
| `authentik/custom-templates` | 自訂頁面模板 |
| `authentik/.env` | 資料庫密碼與 `AUTHENTIK_SECRET_KEY`；secret key 遺失或更換會讓部分資料無法使用，須與資料庫一起備份 |

- 2026.8 版已不使用 Redis，快取與 session 也存在 PostgreSQL。
- ⚠️ `docker compose down -v` 會刪除資料庫 volume。
- 備份：`docker compose exec postgresql pg_dump -U <使用者> <資料庫> > backup.sql`。
- 所有設定目前都是在 UI 手動完成的。正式環境建議改用 **Blueprint**（YAML）定義，資料庫遺失時可重建，也能讓多個環境套用同一份設定。

## 3. authentik 設定

詳細欄位見 README「authentik 設定」。重點如下：

1. **Application 與 Provider**：site-a、site-b 各建一組 OAuth2/OpenID Connect Provider。
   - Client type：Confidential（後端持有 client secret）。
   - Authorization flow：`default-provider-authorization-implicit-consent`，不顯示授權確認頁，第二個網站才能直接登入。
   - Redirect URI：callback（`/api/auth/callback`）與登出後導回的首頁（`/`）都要登記。
   - Invalidation flow：`default-invalidation-flow`。預設的 `default-provider-invalidation-flow` 只結束該應用程式的 session，回到網站會被自動登入。
   - Scopes：`openid`、`email`、`profile`。
2. **存取控制**：Application 沒綁 policy 時所有人都能登入；要限制時在 **Policy / Group / User Bindings** 綁定。
3. **檢查**：`make authentik-check` 確認兩個網站的 OIDC discovery 與 JWKS 端點正常。

每一步可如何自動化：Blueprint 定義 Provider、Application、Policy Binding；或用 REST API（例如 `PATCH /api/v3/providers/oauth2/{pk}/`）。

## 4. 網站登入流程

### 登入（OIDC Authorization Code Flow + PKCE，BFF 模式）

token 只存在後端，不會出現在瀏覽器的 JavaScript 中。

1. 前端呼叫 `/api/me` 得到 401 → 整頁導向後端 `/api/auth/login`。網站本身沒有登入按鈕。
2. 後端產生 `state`（防 CSRF）、`nonce`（防 ID token 重放）、PKCE verifier（防授權碼被攔截後兌換），存在 server 端（10 分鐘有效），導向 authentik。
3. authentik 判斷登入狀態：
   - 已有 authentik session → 不顯示畫面，直接帶授權碼導回。**這就是 SSO 的效果。**
   - 沒有 session → 顯示 authentik 登入頁。
4. 後端在 `/api/auth/callback` 比對 `state`、用 PKCE verifier 換 token、驗證 ID token（簽章、issuer、audience、到期、nonce），建立 session（8 小時）並設定 cookie。

設計細節：
- **不送 `prompt` 參數**：由 authentik 自行決定直接登入或顯示登入頁。
- **cookie 依網站命名**（`site_a_session`、`site_b_session`）：同一個 `localhost` 不同 port 會共用 cookie。
- **避免無限轉址**：callback 任何失敗都導向 `/?auth_error=<code>`，前端看到 `auth_error` 時不自動導向，改顯示錯誤與「重新登入」按鈕。
- 後端啟動時若 OIDC discovery 失敗會直接結束，所以 authentik 要先啟動。

### 登出

1. 前端呼叫 `POST /api/auth/logout`，後端清除本站 session，回傳 authentik 的 end-session 網址。有 session 時帶 `id_token_hint` 與 `post_logout_redirect_uri`。
2. authentik 執行 `default-invalidation-flow` 結束 authentik session，再導回網站首頁 → 自動導向登入頁。

限制：
- **登出不會同步到另一個網站**：site-b 自己的 session 在 8 小時內仍有效。同步登出需要 back-channel logout，未實作。
- **已知問題（2026.8.x）**：在 A 登出後停在 authentik 登入頁，再到 B 登出，會停在 end-session 空白頁。原因是 authentik session 裡有未完成的 flow（[goauthentik/authentik#25545](https://github.com/goauthentik/authentik/issues/25545)），預計 2026.11.0 修正。B 的 session 其實已清除，手動開啟網站即可。

## 5. 使用者管理

`tools/create-users`（`make create-users`）從 CSV 呼叫 authentik REST API 建立使用者：

- 需要 API Token（**Directory → Tokens and App passwords**，Intent 選 `API Token`），填在 `tools/create-users/.env`。
- CSV 欄位：`username`（必填）、`name`、`email`、`password`（空白時產生隨機密碼）、`groups`（以 `;` 分隔，不存在會自動建立）。
- 已存在的帳號會略過，不會修改。

預計正式環境改由**排程程式透過 API 定期新增使用者**。

## 6. 已確認可行、但未實作的 authentik 功能

| 功能 | 做法 |
|------|------|
| 只用帳號（不用 email）登入 | 調整 Identification stage 的 user fields |
| 禁止使用者修改帳號、名稱、email | 調整使用者設定 flow / prompt |
| 忘記密碼（密碼重設） | Recovery flow + Email stage |
| 首次登入強制改密碼 | 自訂 stage + Expression Policy 檢查 `force_password_change` 屬性 |

---

## 7. Synology NAS 整合

### 7.1 需求

- NAS 不另外維護一套帳號密碼。
- **SMB（Windows 網路磁碟）與 DSM 網頁版都要能用。**
- **在 authentik 建立使用者 → NAS 自動產生帳號；在 authentik 改密碼 → NAS 自動改密碼。**
- **只有部分使用者**能使用 NAS。
- 有**多個環境**，每個環境各自一套獨立的 authentik，但 NAS 數量有限。

### 7.2 為什麼不能只用 authentik 的 LDAP

直覺做法是用 authentik 的 LDAP Provider + LDAP Outpost，讓 NAS 加入這個 LDAP。但：

- Synology 的 LDAP 帳號要登入 SMB，LDAP 必須提供 Samba schema 的 NT hash（`sambaNTPassword`），NAS 才能做 NTLMv2 驗證。
- authentik 的 LDAP Outpost **不提供** NT hash，也沒有 Kerberos KDC。

結果：**DSM 網頁可以登入，SMB 不行**。所以需要一個真正的 AD 來處理 SMB 驗證。

### 7.3 架構決策歷程

| 方案 | 說明 | 結論 |
|------|------|------|
| authentik LDAP Outpost | NAS 直接接 authentik 的 LDAP | ❌ SMB 不能用 |
| AD 為主，authentik 從 AD 同步 | 所有人建在 AD，authentik 用 LDAP Source 同步 | ❌ 每個環境都需要一個 AD，NAS 不夠 |
| **authentik 為主，AD 只放 NAS 使用者** | 所有人建在 authentik；要用 NAS 的人另外在 AD 建同名帳號，靠 LDAP Source 綁定與密碼寫回 | ✅ **採用** |

採用方案的特性：
- 每個環境的 authentik 各自是帳號主資料，互相獨立。
- **沒有 NAS 的環境不需要 AD**，只有接 NAS 的環境才需要。
- 只有 AD 裡的人能用 NAS，自然達成「部分使用者」的需求。

```
排程程式 / 管理員
  └→ authentik 建立使用者（所有人）
        └→ 要用 NAS 的人：在 AD 建同名帳號、加入 nas-users
              └→ LDAP Source 同步，依使用者名稱綁定（username_link）
                    └→ 之後在 authentik 改密碼 → 自動寫回 AD → SMB 用新密碼
```

### 7.4 測試環境建置

**原則：不在正式 NAS 上驗證。** 在 NAS 上安裝 Synology Directory Server 會讓它變成網域控制站：需要固定 IP、安裝後不能改主機名稱、會開啟 DNS / Kerberos / LDAP 服務、移除套件會刪除網域資料，也可能調整 SMB 設定。

本次使用一台**非正式環境的 NAS**，透過 **Virtual Machine Manager（VMM）** 跑 **Virtual DSM** 來隔離。

#### VMM 初次設定
1. 主機檢查：直接下一步。
2. 網路：勾選接區網的網卡，建立 `Default VM Network`（虛擬交換器）。⚠️ 這一步會啟用 Open vSwitch，**NAS 網路會短暫斷線一次**，正式 NAS 要在離峰時段做。
3. 儲存空間：選剩餘空間足夠的 volume（至少 40 GB）。

#### 建立 Virtual DSM
| 設定 | 值 |
|------|----|
| CPU / 記憶體 | 2 核 / 2 GB（Directory Server 用 1 GB 會吃緊） |
| 虛擬磁碟 | 30 GB |
| 網路 | `Default VM Network`（external，橋接到區網） |
| 授權 | 免費授權 |

DSM 安裝精靈：
- **Device name 即主機名稱**，直接填最終名稱（例：`vdsm-dc`），裝了 Directory Server 之後不能改。
- 更新方式選「通知我，讓我決定」，避免測試中自動更新重開機。
- Synology Account、QuickConnect、裝置分析都略過。

#### 安裝 Directory Server 前的準備
| 項目 | 必要性 | 說明 |
|------|--------|------|
| 儲存空間（Storage Pool + Volume） | 必要 | 沒有 volume 不能裝套件 |
| 固定 IP | 必要 | 網域的 DNS 紀錄會綁定 IP；用 DHCP 之後 IP 變了，網域會找不到自己的 DC |
| NTP 時間同步 | 確認即可 | Kerberos 要求時間誤差 5 分鐘以內，DSM 預設通常已啟用 |

#### 建立網域（Synology Directory Server）
- 選 **Create a domain**。
- Domain name：`ssodemo.lan`（不要和公司既有網域同名，不要用 `.local`）；NetBIOS：`SSODEMO`。
- 網域管理員帳號固定是 `Administrator`。
- 建立後 DSM 會通知自動變更的設定，皆屬正常：
  - Virtual DSM 自動加入網域。
  - 安裝 DNS Server 套件並設為主要 DNS，原本的 DNS 變成轉送器（上網不受影響）。
  - 啟用 SMB 簽章（可能降低傳輸效能，測試環境可忽略）。

### 7.5 驗證步驟與結果

#### ① AD 帳號登入 SMB ✅
1. SDS → Users & Computers 建立使用者，**取消勾選「下次登入時必須變更密碼」**（否則 SMB 登入失敗）。新使用者自動屬於 `Domain Users`，不需另外加群組。
2. Control Panel → Shared Folder 建立 `test-share`，權限下拉選單切到 **Domain users** 給 Read/Write。
3. Windows 檔案總管開 `\\<Virtual DSM IP>\test-share`，帳號輸入 `SSODEMO\<帳號>`（一定要帶網域前綴）。Windows 不需要加入網域。

#### ② authentik 接 AD（LDAP Source）✅
1. 在 AD 建立服務帳號 `svc-authentik`，加入 **Domain Admins**（寫回密碼需要重設密碼的權限；**正式環境應只委派必要權限**）。
2. 確認連線：`Test-NetConnection <Virtual DSM IP> -Port 636`。
3. authentik → Directory → Federation and Social login → Create → **LDAP Source**：

| 欄位 | 值 | 說明 |
|------|----|------|
| Name / Slug | `ssodemo-ad` | |
| Server URI | `ldaps://<Virtual DSM IP>` | **必須用 LDAPS**：AD 只接受加密連線改密碼；Samba AD 預設也拒絕在 `ldap://` 上做帳密 bind |
| TLS Verification Certificate | 留空 | DSM 為自簽憑證，留空表示不驗證（僅限測試） |
| Bind CN | `svc-authentik@ssodemo.lan` | |
| Base DN | `DC=ssodemo,DC=lan` | |
| Sync users / Sync groups | 勾選 | |
| User password writeback | 勾選 | |
| User Property Mappings | `authentik default LDAP Mapping` 與 `authentik default Active Directory Mapping` 開頭的項目 | |
| Group Property Mappings | `authentik default LDAP Mapping: Name` | |

4. **Run sync** 後，AD 使用者出現在 authentik。

#### ③ authentik 改密碼 → 寫回 AD ✅
在 authentik 改密碼後，SMB 只能用新密碼登入。

#### ④ authentik 既有使用者綁定 AD 同名帳號 ✅（關鍵）
「authentik 為主」的架構成立與否取決於這一步。

- **預設行為**：LDAP Source 的 **User matching mode** 預設為 `identifier`（只認 AD 的唯一識別碼）。authentik 已有同名使用者時會建立失敗並跳過，事件紀錄為 `Failed to create user; To merge new user with existing user, connect it via the LDAP Source's 'Synced Users' tab`。
- **解法**：把 User matching mode 改為 `username_link`（同名即綁定）。
  - 2026.8 的 LDAP Source 編輯畫面**沒有這個欄位**，但 API 支援（官方測試 `test_sync_users_openldap_link_existing` 即驗證此行為）。
  - 用 API：`PATCH /api/v3/sources/ldap/ssodemo-ad/`，body `{"user_matching_mode": "username_link"}`。
  - 或用 authentik shell（**整段放在同一行**，換行會造成 Python 縮排錯誤）：
    ```
    docker compose exec server ak shell -c "from authentik.sources.ldap.models import LDAPSource; s = LDAPSource.objects.get(slug='ssodemo-ad'); s.user_matching_mode = 'username_link'; s.save()"
    ```
  - 可自動化：Blueprint 的 LDAP Source 定義加上 `user_matching_mode: username_link`。
- **驗證**：先在 authentik 建 `carol`（密碼 A），再在 AD 建同名 `carol`（密碼 B）→ Run sync → carol 被綁定（取得 `distinguishedName`、`ldap_uniq` 屬性與 source connection）→ 在 authentik 改密碼 C → **authentik 與 SMB 都能用密碼 C 登入**。
- 密碼寫回依賴使用者的 `distinguishedName` 屬性，綁定時會一併寫入，所以綁定後寫回就會生效。

#### ⑤ AD 密碼原則 ✅（已確認行為）
綁定 AD 的使用者在 authentik 設定密碼時，AD 會檢查密碼原則，不符合就整個失敗：
- 至少 7 個字元。
- 大寫、小寫、數字、符號四類中至少三類。
- 不可包含帳號名稱。

authentik 介面只會顯示籠統的 `There was an error submitting the form`，實際原因在 **Events → Logs**（例：`check_password_restrictions: the password does not meet the complexity criteria!`）。正式環境應在 authentik 設定**與 AD 一致的 Password Policy**，讓使用者看到明確的錯誤。

#### ⑥ DSM 網頁版 OIDC 單點登入 ✅
DSM 7.2 以上的 SSO Client 支援 OpenID Connect，參考 [authentik 官方整合指南](https://integrations.goauthentik.io/infrastructure/synology-dsm/)。

**前提：DSM 要連得到 authentik。** DSM 的後端會直接向 authentik 換 token，所以不能用 `localhost`，要用 authentik 所在電腦的區網 IP（Windows 防火牆需允許連入 9000 port）。authentik 的登入 session 綁在網址上，所以要讓網站和 DSM 之間 SSO，瀏覽器開網站時也要用同一個 authentik 網址。

**authentik 端**：Applications → Create with Provider

| 欄位 | 值 | 說明 |
|------|----|------|
| Application name / Slug | `Synology DSM` / `synology-dsm` | |
| Provider type | OAuth2/OpenID Connect | |
| Authorization flow | `default-provider-authorization-implicit-consent` | 已登入的人直接進 DSM |
| Client type | Confidential | |
| Redirect URI | Strict / Authorization：`https://<Virtual DSM IP>:5001` | 須與使用者開 DSM 的網址完全一致，**不加** `#/signin`（舊版文件有加） |
| Signing key | `authentik Self-signed Certificate` | |
| Scopes | `openid`、`email`、`profile` | |
| Invalidation flow | `default-invalidation-flow` | |
| Bindings（建議） | 群組 `nas-users`（由 LDAP Source 同步） | 沒有權限的人在 authentik 就被擋下 |

**DSM 端**：Control Panel → Domain/LDAP → **SSO Client** → 啟用 OpenID Connect SSO service

| 欄位 | 值 |
|------|----|
| Profile | OIDC |
| Account type | Domain/LDAP/local |
| Name | `authentik` |
| Well-known URL | `http://<authentik 區網 IP>:9000/application/o/synology-dsm/.well-known/openid-configuration` |
| Application ID / Application key | authentik 的 Client ID / Client Secret |
| Redirect URL | `https://<Virtual DSM IP>:5001`（與 authentik 一致） |
| Authorization scope | `openid profile email` |
| Username claim | `preferred_username` |

測試期間不要把 SSO 設為唯一登入方式，保留帳密登入，設定錯誤時才能用管理員帳號修正。

**結果**：
- DSM 登入頁多出 **SSO Authentication** 選項。
- 已在其他網站登入 authentik → **不用輸入帳密，直接登入 DSM**。
- 尚未登入 → 跳出 authentik 登入視窗，輸入帳密後登入 DSM。
- 預設的 `preferred_username`（authentik 帳號名稱，例如 `carol`）就能對應到 DSM 上的網域帳號，**不需要自訂 Scope Mapping**。這也是 AD 帳號必須與 authentik 同名的另一個原因：DSM 不會在 OIDC 登入時自動建立帳號。

### 7.6 「建立使用者時 NAS 自動產生帳號」的做法（確認可行，未實作）

authentik 沒有內建「建立使用者時寫入 AD」的功能，LDAP Source 只會從 AD 同步到 authentik。有兩種補法：

| | 在排程程式裡做 | 在 authentik 裡自動化 |
|---|---|---|
| 機制 | 程式分別呼叫 authentik REST API 與 AD（LDAPS） | Notification Rule 監聽 `model_created` 事件，觸發 Expression Policy（Python）寫入 AD |
| 適用範圍 | 只有透過這支程式建立的使用者 | 任何建立方式（UI、API、其他系統）都會觸發 |
| 執行 | 同步，失敗馬上知道 | 由 worker 非同步執行，錯誤只出現在 Events |
| 測試與維護 | Go 程式碼，容易寫單元測試 | Python 放在 Blueprint，較難測試 |

在 authentik 裡自動化的可行性已從原始碼確認：Expression Policy 可 import 模組；事件會放在 `request.context["event"]`；可直接沿用 LDAP Source 的連線 `LDAPSource.objects.get(slug=...).connection()`，不必在程式碼裡寫帳密。

排程程式的流程（以 Go 為例，AD 端用 `github.com/go-ldap/ldap/v3`；LDAP 是獨立協定，不是透過 authentik API）：
1. 呼叫 authentik API 建立使用者（所有人）。
2. 要用 NAS 的人：透過 LDAPS 在 AD 建同名帳號（`sAMAccountName`、`userPrincipalName`、`userAccountControl=512`），給一組**隨機、符合 AD 原則的臨時密碼**，並加入 `nas-users`。
   - AD 的密碼寫在 `unicodePwd`：前後加雙引號，再轉成 UTF-16LE，且必須走 LDAPS。
3. 呼叫 authentik API 觸發 LDAP 同步，讓兩邊綁定。
4. 呼叫 authentik API 設定真正的密碼 → authentik 寫回 AD，覆蓋臨時密碼。**密碼只從 authentik 一個入口設定。**

### 7.7 正式環境注意事項

- **限縮同步範圍**：`username_link` 代表 AD 出現同名帳號就會接管 authentik 使用者（例如 AD 有人建了 `akadmin`）。應把 LDAP Source 的 User object filter 限縮到 `nas-users` 群組：
  ```
  (&(objectClass=user)(!(objectClass=computer))(memberOf=CN=nas-users,CN=Users,DC=ssodemo,DC=lan))
  ```
- **限制誰能用 NAS**：共用資料夾權限只給 `nas-users`；Control Panel → Application Privileges 中，SMB、File Station、DSM 取消預設的「允許所有人」，只允許 `nas-users`。⚠️ 不要用「拒絕 `Domain Users`」，DSM 是拒絕優先，會連 `nas-users` 的成員一起擋掉。
- **服務帳號最小權限**：`svc-authentik` 不要放 Domain Admins，只委派重設密碼、建立使用者、修改群組成員等必要權限。
- **驗證 LDAPS 憑證**：authentik 的 TLS Verification Certificate 與排程程式都不要略過憑證驗證；改用公司 CA 簽發的憑證，或讓兩端信任 DSM 的憑證。
- **密碼原則一致**：authentik 的 Password Policy 與 AD 的密碼原則保持一致。
- **不要直接在正式 NAS 上裝 Directory Server**：若公司已有 Windows AD，讓 NAS 加入既有 AD，風險小很多。
- **固定 IP**：DC 的 IP 若在 DHCP 範圍內，租約到期後可能被分配給其他裝置。
- **網路可達性**：authentik 需能連到 AD 的 636 port；NAS 需能連到 authentik。網站、DSM 要用同一個 authentik 網址（不能用 `localhost`），SSO session 才會共用，網站的 `OIDC_ISSUER` 也要一併修改。
- **HTTPS**：測試時 authentik 走 HTTP、DSM 用自簽憑證。正式環境 authentik 與 DSM 都應使用受信任的 HTTPS 憑證。
- 沒有 NAS 的環境若也想測 SMB，可用 Docker 跑 Samba AD DC 代替 Synology Directory Server。

### 7.8 留待實作階段處理

以下屬於權限設定的優化，使用 authentik 與 DSM 的內建功能即可完成，不影響可行性結論，正式實作時再設定與驗證：

- **`nas-users` 群組限縮**：LDAP Source 的 User object filter、DSM 共用資料夾權限與 Application Privileges、authentik `synology-dsm` Application 的群組綁定（做法見 7.7）。
- **各網站的存取控制**：哪些人可以登入 site-a、site-b、DSM，用各 Application 的 Policy / Group / User Bindings 設定。綁定 AD 的使用者本身就是 authentik 使用者，登入網站的方式與其他使用者相同。
- **authentik 的 Password Policy**：與 AD 密碼原則對齊（見 7.5 ⑤）。

### 7.9 疑難排解紀錄

| 現象 | 原因 | 解法 |
|------|------|------|
| VMM 的「連線」按鈕不能按 | VMM 介面沒有更新狀態 | 完整關掉 VMM 再重開 |
| 連到的 DSM 裡面有別人的資料 | 連到了同 IP 的另一台設備 | 先不要操作；用 `arp -a` 對照 MAC 確認是哪台 |
| VM 的 IP 是 `169.254.x.x`，`ERR_ADDRESS_UNREACHABLE` | 沒有從 DHCP 拿到 IP | 確認網路用 external；private 網路是隔離的，電腦連不到 |
| DHCP 分到的 IP 和其他設備衝突 | 該 IP 已被其他設備（固定 IP）使用 | VM 關機 → 編輯網路 → 重新產生 MAC → 開機取得新 IP |
| 共用資料夾權限找不到剛建的網域使用者 | DSM 快取了網域使用者清單 | Control Panel → Domain/LDAP → **Update domain data** |
| SMB 換帳號或換密碼後還是用舊的登入 | Windows 快取了連線與憑證 | `net use * /delete /y`、`cmdkey /delete:<IP>`，或改用主機名稱連線 |
| SMB 登入失敗（帳密確定正確） | 帳號沒加 `SSODEMO\` 前綴，或勾了「下次登入須變更密碼」 | 補上前綴，或取消該選項 |
| LDAP 同步時同名使用者沒進來 | User matching mode 為 `identifier` | 改為 `username_link`（見 7.5 ④） |
| 在 authentik 改密碼出現 `There was an error submitting the form` | 不符合 AD 密碼原則 | 查 Events → Logs，換符合原則的密碼 |
| `ak shell -c` 出現 `IndentationError` | 貼上時指令被換行，多了縮排 | 整段指令放在同一行 |
