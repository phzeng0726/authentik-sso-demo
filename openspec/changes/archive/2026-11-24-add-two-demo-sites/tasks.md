## 1. 專案設定

- [x] 1.1 在 `.gitignore` 補上前端忽略項目（`node_modules/`、`dist/`、`*.local`）

## 2. site-a 後端

- [x] 2.1 建立 `site-a/backend` Go module，加入 `gin` 與 `testify` 依賴
- [x] 2.2 實作 `internal/config`：`Load(lookup)` 讀取 `PORT`、`SITE_NAME`，預設 `8081` / `Site A`，port 不在 1–65535 或非數字時回傳 `ErrInvalidPort`
- [x] 2.3 撰寫 `internal/config` 單元測試：有設定、未設定、空字串、port 邊界值（1、65535、0、65536）、非數字
- [x] 2.4 實作 `internal/handler`：`NewRouter(cfg)` 建立 `/api` route group，註冊 `GET /api/health`、`GET /api/info`
- [x] 2.5 撰寫 `internal/handler` 單元測試：health、info 的 status code 與 body 逐欄位驗證，未定義路由回傳 404
- [x] 2.6 實作 `main.go`：載入 config（失敗時以非零狀態結束）、啟動 server
- [x] 2.7 執行 `go test ./...` 與 `go vet ./...` 全數通過，並以 `go run .` 確認 API 可回應

## 3. site-a 前端

- [x] 3.1 以 `create-vite` 的 `react-ts` 範本建立 `site-a/frontend`，移除範本中的示範內容
- [x] 3.2 安裝並設定 Tailwind CSS v4（`tailwindcss`、`@tailwindcss/vite`）
- [x] 3.3 設定 `vite.config.ts`：dev server port `3001`、`strictPort: true`、`/api` proxy 到 `http://localhost:8081`
- [x] 3.4 實作首頁：載入時呼叫 `/api/info`、`/api/health`，顯示網站名稱與後端狀態，含 loading 與錯誤狀態，主色使用 `blue`
- [x] 3.5 執行 `pnpm build` 與 `pnpm lint` 通過，並同時啟動前後端確認頁面顯示正確

## 4. site-b

- [x] 4.1 依 site-a 建立 `site-b/backend`，預設值改為 port `8082`、名稱 `Site B`，測試同步調整並通過
- [x] 4.2 依 site-a 建立 `site-b/frontend`，port 改為 `3002`、proxy 到 `http://localhost:8082`，主色改為 `emerald`
- [x] 4.3 執行 site-b 的後端測試與前端 build / lint 通過，並同時啟動前後端確認頁面顯示正確

## 5. 文件

- [x] 5.1 更新 `README.md`：說明 `site-a`、`site-b` 的目錄結構、port 配置、環境變數與啟動方式
- [x] 5.2 更新 `CLAUDE.md` 的 Architecture 段落，移除 `site-a/`、`site-b/` 的 `(planned)` 標記並補上前後端說明

## 6. 開發快捷指令

- [x] 6.1 在專案根目錄新增 `Makefile`：可一次或個別啟動各網站的前後端、啟停 authentik、安裝前端依賴、執行後端測試，並提供 `help`
- [x] 6.2 在 PowerShell 與 Git Bash 下實測各 target 可用
- [x] 6.3 更新 `README.md` 與 `CLAUDE.md` 的啟動說明，改用 make 指令
