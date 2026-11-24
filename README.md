# authentik-sso-demo
讓兩個不同網站透過 authentik 實現單點登入

## 目錄結構

```
.
├── authentik/        # authentik（server、worker、PostgreSQL），以 Docker Compose 執行
├── site-a/
│   ├── backend/      # Go + Gin API
│   └── frontend/     # React + TypeScript + Vite + Tailwind CSS
├── site-b/           # 與 site-a 結構相同的另一個網站
└── openspec/         # OpenSpec 規格與變更提案
```

## Port 配置

| 服務 | Port |
|------|------|
| authentik | 9000（HTTP）/ 9443（HTTPS） |
| site-a backend | 8081 |
| site-a frontend | 3001 |
| site-b backend | 8082 |
| site-b frontend | 3002 |

## 環境需求

- Go 1.26+
- Node.js 24+、pnpm 11+
- Docker（執行 authentik 用）
- GNU Make（PowerShell 與 Git Bash 皆可執行）

## 快捷指令

在專案根目錄執行，`make help` 可列出所有指令。

| 指令 | 說明 |
|------|------|
| `make install` | 安裝兩個網站的前端依賴 |
| `make dev` | 同時啟動 site-a、site-b 的前後端 |
| `make site-a` / `make site-b` | 只啟動單一網站的前後端 |
| `make site-a-backend` 等 | 只啟動單一服務（`site-{a,b}-{backend,frontend}`） |
| `make authentik-up` / `make authentik-down` | 背景啟動 / 停止 authentik |
| `make authentik-logs` | 追蹤 authentik log |
| `make test` | 執行兩個網站的後端單元測試 |

按 `Ctrl+C` 停止前景執行的服務。

## 啟動網站

```bash
make install   # 第一次執行前
make dev
```

開啟 http://localhost:3001（site-a）與 http://localhost:3002（site-b）。前端的 `/api` 請求會由 Vite dev server 轉發到對應的後端。後端首次 `go run` 需要編譯，期間前端會短暫出現 proxy error，屬正常現象。

不使用 make 時，可在各網站的 `backend/` 執行 `go run .`、`frontend/` 執行 `pnpm dev`。

### 後端環境變數

| 變數 | 說明 | site-a 預設 | site-b 預設 |
|------|------|-------------|-------------|
| `PORT` | 監聽 port，須為 1–65535 的整數 | `8081` | `8082` |
| `SITE_NAME` | 網站名稱，由 `GET /api/info` 回傳 | `Site A` | `Site B` |

### 後端 API

| Method | Path | 回應 |
|--------|------|------|
| GET | `/api/health` | `{"status":"ok"}` |
| GET | `/api/info` | `{"name":"<SITE_NAME>"}` |

## 測試

```bash
make test
```

單元測試撰寫規範見 [TESTING_GUIDELINES.md](TESTING_GUIDELINES.md)。

## 啟動 authentik

```bash
make authentik-up
```
