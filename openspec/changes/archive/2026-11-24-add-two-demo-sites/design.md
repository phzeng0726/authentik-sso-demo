## Context

專案目前只有 `authentik/`（Docker Compose 執行的 authentik server、worker、PostgreSQL），用於後續的 SSO 示範。這次要建立兩個前後端分離的網站 `site-a`、`site-b`，作為之後接入 authentik 的 OIDC client。

本機環境：Go 1.26、Node 24、pnpm 11（前端一律使用 pnpm，不使用 npm）。authentik 佔用 port 9000 / 9443。

## Goals / Non-Goals

**Goals:**
- 兩個可獨立啟動的網站，各自有 Gin 後端與 React 前端
- 後端結構預留 middleware 與 route group 的擴充點，之後加入 OIDC 登入時不需重構
- 後端有符合 `TESTING_GUIDELINES.md` 的單元測試
- 提供 make 快捷指令，一個指令就能啟動所有服務

**Non-Goals:**
- authentik / OIDC 登入、session 管理
- 資料庫、Docker 化、正式環境部署
- 前端單元測試

## Decisions

### 1. 兩個網站各自擁有完整程式碼，不共用

`site-a`、`site-b` 是兩份結構相同但彼此獨立的程式碼（各自的 `go.mod`、`package.json`）。

- **理由**：SSO 示範的重點是「兩個不同的應用」，各自會註冊成 authentik 中不同的 OIDC client，後續可能有不同的 callback、scope 或頁面。獨立程式碼最貼近真實情境，也讓每個網站可以單獨啟動與修改。
- **替代方案**：單一 codebase 用環境變數切換網站身分。重複程式碼較少，但兩個網站的差異會變成到處的條件判斷，不利於示範。

### 2. 後端目錄結構

```
site-x/backend/
├── main.go                    # 載入 config、建立 router、啟動 server
├── go.mod
└── internal/
    ├── config/
    │   ├── config.go          # Load() 讀取 PORT、SITE_NAME，ErrInvalidPort
    │   └── config_test.go
    └── handler/
        ├── router.go          # NewRouter(cfg)：註冊 /api route group
        ├── handler.go         # Health、Info handler
        └── handler_test.go
```

- `config.Load()` 透過可注入的 lookup 函式（`func(string) (string, bool)`）讀取環境變數，測試時不需修改真實環境變數。
- 所有 API 掛在 `/api` route group 下，之後的 auth middleware 可直接套用在 group 上。
- 沒有 service / repository 層：目前沒有業務邏輯，加入只會是空殼。需要時再引入。
- **替代方案**：`cmd/server/main.go` 的標準佈局。本專案每個後端只有一個執行檔，放在根目錄較簡單。

### 3. 後端測試方式

- handler 測試使用 `net/http/httptest` 對 `NewRouter` 發送請求，驗證 status code 並 unmarshal body 逐欄位比對。
- config 測試涵蓋 port 邊界值（1、65535、0、65536）、非數字，以及未設定時的預設值；錯誤路徑用 `errors.Is(err, config.ErrInvalidPort)` 驗證。
- 使用 `testify/suite` 組織測試。

### 4. 前端技術選擇

- **Vite + React + TypeScript**：以 `create-vite` 的 `react-ts` 範本建立。
- **Tailwind CSS v4**：使用 `@tailwindcss/vite` plugin，CSS 只需 `@import "tailwindcss";`，不需要 `tailwind.config.js` 與 PostCSS 設定。
- 不引入 router、狀態管理或 HTTP client 套件，直接用 `fetch`。目前只有一個頁面。

### 5. 前端呼叫後端：開發時用 Vite proxy

前端一律呼叫相對路徑 `/api/...`，開發時由 Vite dev server 轉發到對應後端。

- **理由**：瀏覽器只看到同一個 origin，不需要處理 CORS；之後做 OIDC 時，session cookie 也能維持 same-origin，最單純。
- **替代方案**：後端開 CORS、前端直接呼叫 `http://localhost:8081`。會多出 CORS 與跨站 cookie 的設定，不划算。

### 6. Port 配置

| 網站 | Backend | Frontend |
|------|---------|----------|
| site-a | 8081 | 3001 |
| site-b | 8082 | 3002 |

Vite 設定 `strictPort: true`，port 被佔用時直接失敗，避免自動換 port 後與之後在 authentik 設定的 redirect URI 對不上。

### 7. 網站外觀區別

site-a 使用藍色（`blue`）、site-b 使用綠色（`emerald`）作為主色，在 SSO 示範時一眼就能分辨目前在哪個網站。

### 8. 以 Makefile 提供開發快捷指令

根目錄 `Makefile` 統一提供啟動、測試、authentik 啟停等指令，`make help` 列出所有 target。`make dev` 以 `make -j4` 平行啟動四個服務，`Ctrl+C` 一次全部停止。

- **理由**：四個服務加上 authentik，手動開多個終端機切目錄很繁瑣；make 不需額外安裝語言執行環境，且能用 `-j` 平行啟動。
- **相容性限制**：Windows 上的 GNU Make 會依 PATH 決定用 cmd.exe 或 sh 執行指令（從 PowerShell 執行時通常是 cmd.exe，從 Git Bash 執行時是 sh），因此 recipe 必須兩者都能執行：
  - 只用 `cd <dir> && <cmd>` 這種兩邊語法相同的寫法，不用 shell 專屬語法
  - `echo` 文字不含括號等 sh 特殊字元，也不用 cmd 專屬的 `echo.`
  - 遞迴呼叫寫成 `"$(MAKE)"`，因為 make 路徑可能含空格與括號（如 `C:/Program Files (x86)/...`）
- **替代方案**：
  - 根目錄 `package.json` 搭配 `concurrently`：需要多一層 Node 依賴，且後端是 Go，不適合由 Node 管理
  - 各自的 shell / PowerShell 腳本：要維護兩份，且 PowerShell 腳本在 Git Bash 不方便執行

## Risks / Trade-offs

- [兩份程式碼重複，修改需同步兩邊] → 目前程式碼量很小；若之後差異變少、重複變多，再考慮抽出共用模組。
- [`localhost` 不同 port 屬於 same-site，cookie 會互相可見] → 本次不涉及 cookie；之後做 SSO 時於該 change 的 design 中處理 cookie 名稱與範圍。
- [Vite proxy 只存在於開發環境] → 本次不處理正式部署；之後若要部署，需改由 reverse proxy 負責 `/api` 轉發。
- [本機 GNU Make 為 3.81 舊版，且 cmd.exe / sh 行為不同] → recipe 只使用兩者共通的寫法，並在 PowerShell 與 Git Bash 實測；sh 下 `make help` 的欄位不會對齊，屬已知的外觀問題
