## Context

管理員會先替用戶建立帳號，用戶可能過一段時間才開始使用。目前 README 的做法是在 authentik UI 手動建立測試使用者。authentik 版本為 2026.8.6，REST API 位於 `/api/v3`，用 API token（`Authorization: Bearer`）驗證。API 沒有一次建立多個用戶的端點，所以要逐筆呼叫。

site-a、site-b 的應用程式目前沒有綁定 policy，因此所有用戶都能登入兩個網站，建立帳號後不需要另外設定權限。

本 change 是事後補做：程式碼已在 `tools/create-users/` 完成，這份文件記錄當時的決策。

## Goals / Non-Goals

**Goals:**
- 從一份 CSV 批次建立用戶、設定密碼、加入群組
- 可以安全地重跑：已存在的用戶不受影響
- 在 Windows（PowerShell、cmd.exe）和 sh 都能執行

**Non-Goals:**
- 更新或刪除既有用戶、首次登入強制改密碼、dry-run、改用 blueprint 建立用戶（見 proposal）

## Decisions

### 1. 用 Go 寫，放在獨立 module `tools/create-users/`

- 和後端同語言，開發者已有 Go 環境；`go run` 在 Windows 和 sh 都能跑，可以直接接到 Makefile
- 獨立 module，不和 site-a、site-b 的 module 共用，維持「兩站各自獨立」的原則
- 替代方案：
  - shell 腳本：bash 和 PowerShell 要各寫一份，而且 Makefile 要同時相容 cmd.exe 和 sh
  - Python：專案環境沒有 Python（實際執行時 Windows 也沒有安裝）

### 2. HTTP client 自己寫，不用 authentik 官方 Go SDK

- 只需要 5 個端點（查用戶、建用戶、設密碼、查群組、建群組），用標準函式庫 `net/http` 就夠了
- 官方 SDK（`goauthentik.io/api/v3`）體積大，版本要跟著 authentik 走
- 依賴只有 `godotenv`（和後端相同，讀 `.env`）以及只在測試用的 `testify`

### 3. 已存在的用戶略過，不更新

- 重跑同一份 CSV 時，不會覆蓋用戶已經改過的密碼
- 替代方案「存在就更新」：可能意外重設密碼，而且要決定哪些欄位該覆蓋，超出可行性評估的範圍

### 4. 一筆失敗不中斷整批

- 逐筆處理，每筆的結果記錄在 `Result`，最後印成表格；有失敗時結束碼為 1
- 建立用戶和設定密碼是兩次 API 呼叫，不是原子操作。用戶建好但密碼沒設成功時，以 `ErrPasswordNotSet` 標記，提醒管理員手動補設密碼。這種帳號在補設之前無法登入
- CSV 格式錯誤（缺 `username` 欄位、重複的 username 等）則在呼叫 API 之前就整批停止，避免只建了一半

### 5. 不存在的群組自動建立，並快取查詢結果

- 讓 CSV 可以直接寫群組名稱，不用先到 UI 建群組或查 UUID
- 同一次執行內，群組名稱對應的 UUID 會快取起來，每個群組最多查詢一次、建立一次
- 風險：群組名稱打錯字時，會建出一個多餘的群組（見下方 Risks）

### 6. 密碼空白時用 `crypto/rand.Text()` 產生

- Go 1.24 起內建，產生 26 個 base32 字元，約 128 bits 的亂數，而且不會回傳錯誤
- 產生的密碼只顯示在結果表格中，不寫入檔案
- 產生函式透過 `NewCreator` 注入，測試時可以改用固定值

### 7. 設定方式和後端一致

- `AUTHENTIK_TOKEN` 和 `AUTHENTIK_URL` 從環境變數或 `.env` 讀取，提供 `env.example` 範本
- 沿用專案慣例：範本叫 `env.example` 而不是 `.env.example`
- token 不提供命令列參數，避免留在 shell history

### 8. CSV 相容 Excel

- 標頭不分大小寫，會去除前後空白；開頭的 UTF-8 BOM 會被忽略，因為 Excel 存成「CSV UTF-8」時會加上 BOM
- 除了 `password` 以外的欄位都會去除前後空白，`password` 保留原樣

## Risks / Trade-offs

- [CSV 和結果表格含明文密碼] → `users.csv` 加入 `.gitignore`；產生的密碼只印在 stdout，請管理員記下後清除終端機紀錄
- [API token 權限很大] → token 只放在 gitignored 的 `.env`；評估結束後可以到 authentik UI 刪除 token
- [群組名稱打錯會建出多餘的群組] → 結果表格看不出建了哪些群組，需要到 authentik UI 確認；可行性評估階段接受這個風險
- [建立用戶和設定密碼不是原子操作] → 以 `ErrPasswordNotSet` 明確回報，不自動刪除用戶
- [逐筆呼叫，大量用戶時較慢] → 可行性評估的用戶數量少，不做並行處理
- [`set_password` 是否套用 authentik 的密碼政策未經實測] → 如果被拒絕，該筆會回報 `ErrPasswordNotSet` 和 API 的錯誤訊息
