# D7 Orchestration Spec Delta — emitError error_code 注入 + FallbackModel 预留 (DM-20260628-001)

**Change ID:** devrix-api-error-classification
**Demand ID:** DM-20260628-001
**Delta Type:** ADDED (v4.8.0 → v4.9.0 → v4.9.1)
**SOT:** `internal/layers/orchestration/sessionorchestrator/helpers.go` · `turn_orchestrator.go`

---

## 1. 修改总览

| 内容 | 文件 | 类型 | 行为变化 |
|------|------|------|----------|
| 1. `OrchestratorDeps.FallbackModel string` 字段 | `turn_orchestrator.go` | NEW | 字段就位（P0-2 streaming-fallback 接线） |
| 2. `TurnState.Withheld bool` 字段 | `turn_orchestrator.go` | NEW | 标 prompt_too_long 错误不 surface error 事件 |
| 3. `emitError` 路径填 `Event.Metadata["error_code"]` | `helpers.go` | MODIFIED | 用 `sharederrors.Code(err)` 提取受控枚举 |
| 4. `emitErrorWithErr` (V4 变体) | `turn_orchestrator.go` | NEW | 携带原始 error，用于 sharederrors.Code() 提取 |
| 5. `observeFallbackTrigger` 日志 | `turn_orchestrator.go` | NEW | 2 次连续 RateLimit/ServerError 触发 `fallback_trigger_candidate` 日志 |
| 6. FallbackModel 未 wire 场景日志 | `turn_orchestrator.go` | NEW | 显式标注 `fallback_model_set_but_not_yet_wired` |

---

## 2. error_code 受控枚举契约（D7 → D1）

```go
// D7 emitErrorWithErr 填 metadata
apiErr := sharederrors.NewLLMAuthFailedError(
    llmgateway.NewAPIErrorWithCause(401, "auth failed", sharederrors.ErrLLMAuthFailed))
o.emitErrorWithErr(out, "sess_1", "boom", apiErr)
// Event.Metadata["error_code"] == "authentication_failed" ✅
```

---

## 3. T 点增量（v4.8.0 → v4.9.0 → v4.9.1）

| T ID | 描述 | Status |
|------|------|--------|
| D7-S2-A50-T05 | `OrchestratorDeps.FallbackModel` + `TurnState.Withheld` + `emitError` 路径用 `sharederrors.Code(err)` 填 `Event.Metadata["error_code"]` | IMPLEMENTED (DM-20260628-001 S5 PR #265, 2 case PASS) |
| D7-S2-A50-T06 | 主模型 2 次连续 RateLimit/ServerError 触发 `fallback_trigger_candidate` 日志 + withheld + SanitizeForUser 回归 | IMPLEMENTED (DM-20260628-001 S5 PR #265, 3 case PASS) |

---

## 4. 行为不变保证

- 现有 30+ `SanitizeForUser` 调用点零行为变化（sharederrors.WithCode string API 保留）
- error event 行为：除 metadata 新增 `error_code` 字段外，Type/Content/SessionID 不变
- 新 metadata 字段为空时 fallback `"unknown"`（不破坏下游消费者）

---

## 5. Out of Scope

- 完整 streaming fallback 自动切换循环（放 P0-2）
- prompt_too_long fold→retry 完整闭环（放 P0-3）