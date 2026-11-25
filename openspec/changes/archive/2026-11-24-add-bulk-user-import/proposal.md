## Why

管理員會先替用戶建立帳號，用戶再用這些帳號登入 site-a、site-b。在 authentik UI 逐一建立帳號、設密碼、加群組很耗時，可行性評估時也需要能快速建立一批測試帳號。authentik 提供 REST API，但沒有一次建立多個用戶的端點，所以需要一個讀 CSV、逐筆呼叫 API 的工具。

## What Changes

- 新增 `tools/create-users/`：獨立的 Go module（命令列工具），讀取 CSV，透過 authentik REST API（`/api/v3`）批次建立用戶
  - CSV 只有 `username` 必填，另有選填的 `name`、`email`、`password`、`groups` 欄位；標頭不分大小寫，可讀取 Excel 存的 UTF-8 BOM
  - 已存在的 username 會略過，不會修改；不存在的群組會自動建立
  - `password` 空白時產生隨機密碼，顯示在結果表格中
  - 任一筆失敗時其他筆繼續處理，最後以表格列出每筆結果；有失敗時結束碼為 1
  - `AUTHENTIK_TOKEN`（必填）與 `AUTHENTIK_URL` 從環境變數或工具目錄的 `.env` 讀取
- 新增範本 `env.example` 與 `users.example.csv`；`users.csv` 可能含密碼，加入 `.gitignore`
- Makefile 新增 `make create-users`，`make test` 加入此工具的測試
- README、CLAUDE.md 補上使用說明

### Non-goals

- 不更新或刪除已存在的用戶
- 不做「首次登入強制改密碼」：已確認 authentik 可以透過自訂 flow 做到，但可行性評估階段先不實作
- 不提供 dry-run 模式
- 不改用 authentik blueprint 建立用戶

## Capabilities

### New Capabilities

- `bulk-user-import`：從 CSV 批次建立 authentik 用戶與群組的命令列工具，包含 CSV 格式、略過與錯誤處理規則、結果輸出及設定方式

### Modified Capabilities

（無）

## Impact

- 新增 `tools/create-users/`（`main.go`、`csv.go`、`client.go`、`creator.go` 及測試、`go.mod`、`env.example`、`users.example.csv`）
- 依賴：`github.com/joho/godotenv`、`github.com/stretchr/testify`（僅測試用），與後端使用相同版本
- `Makefile`、`.gitignore`、`README.md`、`CLAUDE.md`
- authentik：需要管理員帳號的 API token（手動在 UI 建立）
- site-a、site-b 不受影響
