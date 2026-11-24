# Unit Test 撰寫規範

適用於 `site-a/backend`、`site-b/backend` 等 Go 後端的單元測試。

## 三大原則

### 1. 反向驗證 (Reverse Validation)

每個 production code 的 `if` 條件，至少要有一個測試會在該條件被移除時失敗。

- 邊界值測試：剛好在限制上（應成功）、超過限制（應失敗）
- 確保測試的是 `>` 而非 `>=`（或反之）

### 2. 精確斷言 (Precise Assertion)

不得只用 `NoError` / `NotEmpty` / `Contains` 作為唯一斷言。每個測試都要驗證具體的輸出值。

- 數值型結果：驗證預期的精確值
- 字串型結果：用精確字串比對，而非 `Contains`
- 集合型結果：驗證長度及每個元素的關鍵欄位

### 3. 錯誤身份驗證 (Error Identity Verification)

所有 error path 測試必須驗證具體錯誤類型/代碼：

- Sentinel error → `errors.Is(err, domain.ErrXxx)`
- 自訂錯誤型別 → `errors.AsType[*domain.AppError](err)` + 驗 `appErr.Code`
- 不得僅用 `s.Error(err)`

## 各層 Assertion 深度要求

| 層級 | 要求 |
|------|------|
| **Repository** | 寫入後讀回，逐欄位 assert；驗證 generated fields（ID、CreatedAt）非零值；side effect（cascade delete）顯式驗證 |
| **Service** | `DoAndReturn` 驗傳入 mock 的每個欄位；計算型結果逐欄位驗證預期值 |
| **Handler** | 驗 HTTP status code + unmarshal response body 驗 JSON 欄位；`DoAndReturn` 驗 service input struct 轉換正確；不得用 `gomock.Any()` 作為成功案例的唯一 matcher |
| **Config** | 每個環境變數驗證「有設定」與「未設定（使用預設值）」兩種情況的精確值 |

## CRUD 操作必要測試分類

| 分類 | 說明 |
|------|------|
| Happy path（最小） | 只帶必填欄位 |
| Happy path（完整） | 帶齊所有欄位，逐欄位驗證 |
| 邊界值 | 剛好在限制上 / 超過限制 |
| Error：domain 規則違反 | 業務邏輯錯誤 |
| Error：not found | entity 不存在 |
| Error：conflict | 唯一約束衝突 |
| Error：上游依賴失敗 | 每個可能失敗的 repo/service call |
