## Why

本專案的目標是讓兩個不同網站透過 authentik 實現 SSO，但目前只有 authentik 本身，還沒有可以接入的網站。需要先建立兩個前後端分離的簡易網站作為基礎，後續再於其上整合 authentik 登入。

## What Changes

- 新增 `site-a/` 與 `site-b/` 兩個獨立網站，結構相同，各自包含：
  - `backend/`：Golang + Gin 的 HTTP API 服務（獨立 Go module）
  - `frontend/`：React + TypeScript + Vite + Tailwind CSS 的單頁應用
- 後端提供基本 API：
  - `GET /api/health`：健康檢查
  - `GET /api/info`：回傳網站識別資訊（例如網站名稱）
- 後端設定（監聽 port、網站名稱）透過環境變數提供，並有預設值
- 前端首頁顯示網站名稱，並呼叫後端 API 顯示回傳結果；開發時透過 Vite proxy 將 `/api` 轉發至對應後端
- 兩個網站使用不同 port，避免與 authentik（9000 / 9443）衝突：
  - site-a：backend 8081、frontend 3001
  - site-b：backend 8082、frontend 3002
- 後端單元測試遵循 `TESTING_GUIDELINES.md`
- 新增根目錄 `Makefile`，提供開發快捷指令：一次或個別啟動各網站前後端、啟停 authentik、安裝前端依賴、執行後端測試

### Non-goals

- 不包含 authentik / OIDC 登入整合（另開 change 處理）
- 不包含資料庫、使用者資料或其他業務功能
- 不包含正式環境部署與容器化

## Capabilities

### New Capabilities

- `demo-site-backend`: 網站後端 API 服務，包含健康檢查、網站資訊 API 與環境變數設定
- `demo-site-frontend`: 網站前端頁面，包含首頁顯示網站名稱、呼叫後端 API 並呈現結果

### Modified Capabilities

（無）

## Impact

- 新增目錄：`site-a/backend`、`site-a/frontend`、`site-b/backend`、`site-b/frontend`
- 新增依賴：
  - Go：`github.com/gin-gonic/gin`、`github.com/stretchr/testify`（測試用）
  - Node（以 pnpm 管理）：`react`、`react-dom`、`vite`、`typescript`、`tailwindcss`、`@tailwindcss/vite`、`oxlint`（lint，Vite 範本內建）
- 新增根目錄 `Makefile`，本機開發需安裝 GNU Make
- 本機開發需佔用 port 8081、8082、3001、3002
- `.gitignore` 需補上前端相關忽略項目（如 `node_modules/`、`dist/`）
