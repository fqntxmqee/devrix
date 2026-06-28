# D3 LLM Gateway Spec Delta — APIErrorCode V4 (DM-20260628-001)

**Change ID:** devrix-api-error-classification
**Demand ID:** DM-20260628-001
**Delta Type:** ADDED (V4 → §14 ADDED Requirements)
**SOT:** `internal/shared/errors/api_code.go` · `internal/layers/llmgateway/api_error.go`

---

## 1. 修改总览

| 内容 | 文件 | 类型 | 行为变化 |
|------|------|------|----------|
| 1. `APIErrorCode` 7 类闭集枚举 | `internal/shared/errors/api_code.go` | NEW | RateLimit / AuthenticationFailed / ServerError / MediaSize / PromptTooLong / ImageSize / Unknown |
| 2. `NewAPIErrorCodeFromStatus` HTTP status → APIErrorCode 映射 | `internal/shared/errors/api_code.go` | NEW | 401/403→AuthFailed, 408/413→PromptTooLong, 429→RateLimit, 5xx/529→ServerError |
| 3. `sharederrors.IsCode(err, code)` 包装链识别 | `internal/shared/errors/api_code.go` | NEW | 走 APICodeProvider → SentinelError 链 |
| 4. `APICodeProvider` 接口（跨包 duck-typing） | `internal/shared/errors/api_code.go` | NEW | sharederrors 定义，llmgateway.APIError 实现 |
| 5. `llmgateway.APIError` 结构 + `NewAPIError` / `NewAPIErrorWithCause` | `internal/layers/llmgateway/api_error.go` | NEW | Status + Message + Code + Cause |
| 6. 4 adapter HTTP 错误构造统一走 `NewAPIError` | `internal/layers/llmgateway/stream/adapter/{minimax,deepseek,anthropic,openai}_*.go` | MODIFIED | 替换 `fmt.Errorf` 字符串拼接 |

---

## 2. 跨包错误传播契约

```go
// adapter 层：构造带 APICode 的错误链
apiErr := llmgateway.NewAPIErrorWithCause(429, "rate limited", nil)
wrapped := sharederrors.NewProviderUnavailableError(apiErr)

// 任意层：提取受控枚举
sharederrors.Code(wrapped) == sharederrors.APICodeRateLimit // ✅
sharederrors.IsCode(wrapped, sharederrors.APICodeRateLimit) // ✅
```

---

## 3. T 点增量（v3.2.0 → v3.3.0 → v3.3.1）

| T ID | 描述 | Status |
|------|------|--------|
| D3-S1-A01-T04 | `NewAPIErrorCodeFromStatus` HTTP status → APIErrorCode 映射覆盖 8 类 (401/403/408/413/429/529/5xx/4xx-unknown) | IMPLEMENTED (DM-20260628-001 S5 PR #265) |
| D3-S1-A01-T05 | `sharederrors.IsCode(err, code)` 正确识别 WithCode→Unwrap→bare APIError 包装链 | IMPLEMENTED (DM-20260628-001 S5 PR #265) |
| D3-S3-A01-T17 | 4 adapter 错误构造统一走 `NewAPIError` 工厂 | IMPLEMENTED (DM-20260628-001 S5 PR #265) |

---

## 4. Out of Scope（本 Change 不做）

- 完整 streaming fallback 自动切换（放 P0-2 `devrix-streaming-fallback`）
- prompt_too_long fold→retry 闭环（放 P0-3 `devrix-withhold-then-recover`）
- per-tool maxResultSizeChars（放 P1-4）