# Tasks: devrix-api-error-classification

**Change ID:** devrix-api-error-classification
**Demand ID:** DM-20260628-001
**Status:** S4_Implementation
**Created:** 2026-06-28

---

## 任务清单（按 F-T 映射）

### T1: APIErrorCode 闭集枚举 + HTTP status 映射 [D3-S1-A01-T04]

- **范围**: `internal/shared/errors/api_code.go` + `api_code_test.go`
- **F**: 新 F10 NewAPIErrorCodeFromStatus
- **预估**: 80 行实现 + 120 行单测
- **依赖**: 无

**步骤：**
- [x] 定义 `APIErrorCode int` 类型 + 7 const
- [x] 实现 `String() string` 方法（switch 返回 7 个字符串）
- [x] 实现 `NewAPIErrorCodeFromStatus(status int) APIErrorCode`（401/403/408/413/429/5xx/529/other 映射）
- [x] 实现 `ParseAPIErrorCode(s string) APIErrorCode`（反向解析 + Unknown 兜底）
- [x] 单元测试 12+ case（7 enum + 5 status 边界）

### T2: sharederrors 三 API（Code/IsCode/WithAPIErrorCode）[D3-S1-A01-T05]

- **范围**: `internal/shared/errors/api_code.go`（追加）
- **F**: 复用现有 F `WithCode`
- **预估**: 30 行
- **依赖**: T1

**步骤：**
- [x] 实现 `Code(err error) APIErrorCode`（遍历 errors.As 链，找 Code == APICode_*.String() 的 SentinelError）
- [x] 实现 `IsCode(err error, code APIErrorCode) bool`（遍历 errors.As + 比较）
- [x] 实现 `WithAPIErrorCode(code APIErrorCode, msg string, cause error) error`（薄包装 WithCode(code.String(), msg, cause)）
- [x] 单元测试 3 case（WithCode wrap + bare err + nil 路径）

### T3: llmgateway.APIError 结构 + NewAPIError 工厂 [D3-S3-A01-T17]

- **范围**: `internal/layers/llmgateway/api_error.go` + `api_error_test.go`
- **F**: 新 F11 NewAPIError
- **预估**: 50 行实现 + 60 行单测
- **依赖**: T1

**步骤：**
- [x] 定义 `APIError struct { Status int; Message string; Code sharederrors.APIErrorCode; Cause error }`
- [x] 实现 `Error() string`（优先 Message，否则 Cause.Error()，最后 Code.String()）
- [x] 实现 `Unwrap() error`（返回 Cause）
- [x] 实现 `NewAPIError(status int, message string) *APIError`（自动 NewAPIErrorCodeFromStatus）
- [x] 单元测试 4 case（4 status 映射 + Error 优先级 + Unwrap 链）

### T4: 4 Adapter HTTP 错误构造统一走 NewAPIError [D3-S3-A01-T17]

- **范围**: `internal/layers/llmgateway/stream/adapter/{openai,minimax,deepseek,anthropic}_*.go`
- **F**: 复用现有 adapter F
- **预估**: 4 adapter × ~5 行
- **依赖**: T3

**步骤：**
- [x] `openai_stream.go` L86-88 改造：`fmt.Errorf(...)` → `llmgateway.NewAPIError(status, msg)`
- [x] `minimax.go` 同模式
- [x] `deepseek.go` 同模式
- [x] `anthropic.go` 同模式
- [x] 4 adapter 单测覆盖 HTTP 5xx/4xx 错误构造路径

### T5: OrchestratorDeps.FallbackModel 字段 + emitError 改造 [D7-S2-A50-T05]

- **范围**: `internal/layers/orchestration/sessionorchestrator/turn_orchestrator.go` + `helpers.go`
- **F**: 新 F FallbackModel 字段 + emitError 改造
- **预估**: 25 行
- **依赖**: T1, T2

**步骤：**
- [x] `OrchestratorDeps` 加 `FallbackModel string` 字段
- [x] `DefaultOrchestrator` 加 `fallbackModel string` 字段
- [x] `NewOrchestrator` 复制字段
- [x] `helpers.go:emitError` 用 `sharederrors.Code(err)` 填 `Event.Metadata["error_code"]`
- [x] `turn_orchestrator.go` 在主模型错误处累计 `consecutiveServerErrors`，== 2 时 log `fallback_trigger_candidate`，如果 `fallbackModel == ""` 额外 log `fallback_model_set_but_not_yet_wired`
- [x] 单测覆盖 5 case（字段就位 / 未填日志 / 已填日志 / emitError error_code 字段 / 向后兼容）

### T6: TurnState.Withheld 字段 + prompt_too_long withhold 路径 [D7-S2-A50-T06]

- **范围**: `internal/layers/orchestration/sessionorchestrator/turn_state.go`（或等价）
- **F**: 新 F Withheld 状态
- **预估**: 15 行
- **依赖**: T1, T2

**步骤：**
- [x] `TurnState` 加 `Withheld bool` 字段
- [x] `turn_orchestrator.go` 在 adapter 返回 `APICodePromptTooLong` / `APICodeMediaSize` 时，`state.Withheld = true` 且**不调用** `emitError`
- [x] `FoldAssistantOutput`（D2 已有）成功后 `state.Withheld = false`
- [x] fold 失败后 `emitError({error_code: "prompt_too_long"})`
- [x] 单测 3 case（withhold 触发 / fold 成功清掉 / fold 失败 surface）

### T7: D1 feishu IM 适配器差异化文案 [D1-S3-A08-T01]

- **范围**: `internal/layers/communication/channel/adapters/feishu.go` + 新增 `feishu_error_format.go`
- **F**: 新 F IM 差异化文案
- **预估**: 50 行
- **依赖**: T1

**步骤：**
- [x] 新增 `formatErrorByCode(code, fallback string) string` 函数（switch 5 类 code + 兜底）
- [x] `feishu.go` "error" case (L149-162) 用 `formatErrorByCode(msg.Metadata["error_code"], content)`
- [x] 5 case 单测（RateLimit/Auth/PromptTooLong/MediaSize/ServerError + Unknown 兜底）

### T8: D1 cli 适配器差异化文案 [D1-S3-A08-T01]

- **范围**: `internal/layers/communication/channel/adapters/cli.go` + `renderers/message.go`
- **F**: 同 T7
- **预估**: 25 行
- **依赖**: T7

**步骤：**
- [x] `cli.go` 在 RenderError 前 format（如果是 emit error 路径）
- [x] 共享 `formatErrorByCode` 函数（移到 shared 或 cli_error_format.go）
- [x] 单测覆盖与 feishu 一致

### T9: 端到端集成测试 [D3-S3-A01-T17 E2E variant / AC8]

- **范围**: `tests/integration/llm_fallback_e2e_test.go`
- **F**: E2E test
- **预估**: 150 行
- **依赖**: T4, T5, T6

**步骤：**
- [x] mock 主模型连续 3 次 529
- [x] 启动 session + 用户发消息
- [x] 验证 3 次 emit 错误路径 + 日志含 `fallback_trigger_candidate`
- [x] 验证 mock feishu 适配器收到 `error_code: "server_error"` + "🔧 服务暂时不可用"

---

## 总 T 层映射表

| T ID | 任务 | 文件 | 状态 |
|------|------|------|------|
| D3-S1-A01-T04 | T1 APIErrorCode 枚举 + 映射 | api_code.go | ✅ |
| D3-S1-A01-T05 | T2 Code/IsCode/WithAPIErrorCode | api_code.go | ✅ |
| D3-S3-A01-T17 | T3+T4+T9 APIError + adapters + E2E | api_error.go + 4 adapter + e2e_test | ✅ |
| D7-S2-A50-T05 | T5 FallbackModel + emitError error_code | turn_orchestrator.go + helpers.go | ✅ |
| D7-S2-A50-T06 | T6 Withheld 状态 | turn_state.go + turn_orchestrator.go | ✅ |
| D1-S3-A08-T01 | T7+T8 IM 差异化文案 | feishu.go + cli.go + renderers/message.go | ✅ |

---

## 验证清单

- [ ] `go vet ./...` 0 errors
- [ ] `./scripts/test-unit.sh` 100% PASS
- [ ] `./scripts/test-domain.sh d3` -race 100% PASS
- [ ] `./scripts/test-domain.sh d7` -race 100% PASS
- [ ] `./scripts/test-domain.sh d1` -race 100% PASS
- [ ] coverage ≥ 80% on api_code.go / api_error.go / helpers.go / feishu_error_format.go

---

**S4 完成。下一步：S4-Gate（按 `review-code.md` §2 四维度自检）。**
