## 1. 建立 module

- [x] 1.1 建立 `tools/create-users/go.mod`（module `create-users`，Go 1.26.5，依賴 `godotenv`、`testify`）
- [x] 1.2 新增 `env.example`（`AUTHENTIK_TOKEN`、`AUTHENTIK_URL`，附建立 token 的步驟）
- [x] 1.3 新增 `users.example.csv`，內容為不含真實資料的範例：涵蓋產生密碼、CSV 密碼、多個群組、只有 username 等情況

## 2. CSV 解析（`csv.go`）

- [x] 2.1 `ParseUsers`：標頭不分大小寫、去除空白、忽略 UTF-8 BOM；只有 `username` 必填；`name` 預設為 username；`password` 不去空白；`groups` 以 `;` 分隔
- [x] 2.2 錯誤：`ErrEmptyCSV`、`ErrMissingColumn`、`ErrMissingUsername`（帶行號）、`ErrDuplicateUsername`（帶兩個行號）、CSV 格式錯誤
- [x] 2.3 `csv_test.go`：每個情境驗證精確的 `Row` 內容或錯誤身分與訊息

## 3. API client（`client.go`）

- [x] 3.1 `Client` 送出 `Authorization: Bearer`，base URL 結尾的 `/` 不會造成 `//api`
- [x] 3.2 `FindUserByUsername`、`FindGroupByName` 精確比對名稱；`CreateGroup`、`CreateUser`（沒有群組時送 `[]`，空 email 省略）、`SetPassword`
- [x] 3.3 非 2xx 回應回傳 `APIError`（method、path、status、最多 4 KiB 的 body）；JSON 解析錯誤與連線錯誤也會回傳
- [x] 3.4 `client_test.go`：驗證每個端點的 method、path、query、header、body；APIError 欄位；status 299 與 300 的邊界；解析錯誤、連線錯誤

## 4. 建立流程與輸出（`creator.go`）

- [x] 4.1 `Creator.CreateAll`：已存在的用戶略過且不送出其他請求；否則解析群組 → 建立用戶 → 設定密碼；空白密碼用注入的產生函式
- [x] 4.2 群組名稱與 UUID 的快取：每個群組在同一次執行中最多查詢一次、建立一次
- [x] 4.3 每筆失敗獨立回報、不中斷其他筆；設定密碼失敗時包裝 `ErrPasswordNotSet`；失敗的列不顯示產生的密碼
- [x] 4.4 `PrintResults`：輸出 `USERNAME STATUS PASSWORD NOTE` 表格，回傳失敗筆數
- [x] 4.5 `creator_test.go`：用假的 authentik server 驗證送出的請求、建立的資料、各種失敗情況及表格的精確輸出

## 5. 進入點（`main.go`）

- [x] 5.1 載入 `.env`；`-csv`（預設 `users.csv`）、`-url`（預設 `AUTHENTIK_URL` 或 `http://localhost:9000`）
- [x] 5.2 沒有 token、CSV 開檔失敗或解析失敗時結束碼為 2；有任何一筆失敗時為 1；全部成功時為 0
- [x] 5.3 `.env` 改在 `main()` 載入，讓測試不會讀到真的 token；`main_test.go` 驗證每個結束碼、stderr 訊息、`-csv` 預設值、`-url` 優先於 `AUTHENTIK_URL`，以及 `envOr`

## 6. 專案整合

- [x] 6.1 `.gitignore` 加入 `tools/create-users/users.csv`
- [x] 6.2 Makefile：新增 `create-users` target 與 help 說明；`make test` 加入此工具的測試（指令要同時相容 cmd.exe 和 sh）
- [x] 6.3 README：目錄結構、快捷指令表、「批次建立使用者」章節（token 建立步驟、CSV 欄位、行為說明）
- [x] 6.4 CLAUDE.md Architecture 加上 `tools/create-users/`

## 7. 驗證

- [x] 7.1 `gofmt`、`go vet ./...`、`make test` 全部通過
- [x] 7.2 對執行中的 authentik 實際跑 `make create-users`，確認用戶建立成功，並能用該帳號登入 site-a、site-b
