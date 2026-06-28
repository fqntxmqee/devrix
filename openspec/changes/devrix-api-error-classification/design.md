# Design: D3 LLM Gateway API 错误分类与可恢复语义

**Change ID:** devrix-api-error-classification
**Demand ID:** DM-20260628-001
**Status:** S3_Design
**Primary Domain:** D3 (llm-gateway, public)
**Secondary Domains:** D7 (orchestration, core), D1 (communication, core)
**Author:** clawcode v2.1.88 对比分析（2026-06-28）

---

## 1. Root Cause Analysis

| ID | 根因 | 触发条件 | 影响面 |
|----|------|----------|--------|
| **RC-1** | adapter 层 `*llmgateway.APIError` 只携带 `Status` / `Message`，无受控分类枚举 | 4 adapter（minimax/deepseek/anthropic/openai）在 HTTP 5xx/4xx 时构造 `*sharederrors.SentinelError` 但 Code 字段是 LLM_* 命名空间，与 HTTP status 语义无对应 | D7 orchestrator 拿不到分类信息，统一文案 |
| **RC-2** | `OrchestratorDeps` 没有 `FallbackModel` 字段 | v1.0/v1.1 编排层只接 `DefaultModel` 单模型；主模型 529 时无 fallback 路径 | 高优用户（minimax M2.7 偶发 529）必须手动重发 |
| **RC-3** | `prompt_too_long` / `media_size` 错误直接 emit error 终止 session | `emitError` 路径不区分错误可恢复性；`withheld` 状态字段从未存在 | 长 session 累计超 200K 时丢失全部已完成工作 |
| **RC-4** | D1 IM 适配器只能基于 `Event.Type == "error"` 走统一文案 | 2026-06-27 hotfix 虽暴露 `error_code` 字段，但 `sharederrors.ErrorCode(err)` 无受控枚举语义；feishu.go/cli.go 无 case 分支 | 用户看到"网络异常"实际是 key 过期 |
| **RC-5** | `sharederrors.Code()` / `IsCode()` API 缺失 | DM-20260620-003 落地了 `WithCode` 但未提供**枚举版本**的查询 API；现有 `ErrorCode(err) string` 返回任意字符串，IM 端无法 switch | 与 RC-1 互锁：枚举不暴露 → IM 端无法消费 |

## 2. Solution Design

### 2.1 三层契约（与 proposal §3.3 一致）

```
[闭集枚举层]                [传播层]                        [消费层]
sharederrors.APIErrorCode   llmgateway.APIError{Status,    D7 emitError →
  7 const                     Message, Code, Cause}          Event.Metadata["error_code"]
sharederrors.NewAPIErrorCode sharederrors.Code(err)         D1 feishu.go / cli.go
  CodeFromStatus(status)   sharederrors.IsCode(err, code)    switch code → 差异化文案
```

### 2.2 复用既有基础设施（DM-20260620-003）

- **SentinelError**：`type SentinelError struct { Code string; Message string; Err error }` 已就位
- **`LLMError = SentinelError`** 类型别名保留（向后兼容）
- **`WithCode(code, message string, err error) *SentinelError`**：API 已存在，本需求复用
- **SanitizeForUser**：不修改，仅作为错误信息的最后一层脱敏

**关键设计选择**：APIErrorCode 设计为 `int` 类型而非 string，闭集约束由 Go `const ( ... )` 强约束（编译期禁止动态新增）。

### 2.3 与 clawcode 对照

| clawcode 实现 | devrix 设计 | 差异 |
|---------------|-------------|------|
| `string` 类型 enum（TypeScript 风格） | `int` 常量（Go 强类型） | 闭集约束更强 |
| `categorizeRetryableAPIError` 函数返回 string | `NewAPIErrorCodeFromStatus(status int) APIErrorCode` 函数返回 int | 输入从 error 改为 status int，更纯粹 |
| `isWithheldMaxOutputTokens` 内联状态 | `TurnState.Withheld bool` 字段 | 状态归属 D7 orchestrator |
| `FallbackTriggeredError` 自动切换 fallback | 仅字段预留 `OrchestratorDeps.FallbackModel string` | P0-2 follow-up |

### 2.4 关键决策

#### Decision 1: APIErrorCode 是 int 还是 string

**选项:**
| 方案 | 优点 | 缺点 |
|------|------|------|
| A. `int`（Go 风格） | 闭集约束由编译器保证；switch 性能好；与 existing `CodeLLM*` string 不冲突 | JSON 序列化不直观（要写 MarshalJSON） |
| B. `string`（clawcode 风格） | JSON 友好；日志可读性好 | 无编译期闭集约束；容易拼错 |
| C. `int` + `String()` 方法 | A 的优点 + 日志可读 | JSON 仍需自定义序列化 |

**选择:** C
**理由:** 闭集是 AC1 硬要求（编译期禁止动态新增），`String()` 用于日志/IM 文案；JSON 序列化为次要关注点（Metadata 是 `map[string]string` 走 `.String()` 即可）。

#### Decision 2: `llmgateway.APIError.Code` 是新增字段还是替换 `Message`

**选项:**
| 方案 | 优点 | 缺点 |
|------|------|------|
| A. 保留 `Message`，**新增** `Code APIErrorCode` 字段 | 向后兼容；零行为变化 | struct 大小微增 |
| B. 用 Code 替换 Message | 字段精简 | 破坏现有调用方（adapter 内 log, SanitizeForUser 取 Message） |

**选择:** A
**理由:** `SanitizeForUser` 现有 30+ 调用点依赖 `Message` 字段；零行为变化是 AC6 硬要求。

#### Decision 3: emitError 的 error_code 字段值用什么类型

**选项:**
| 方案 | 优点 | 缺点 |
|------|------|------|
| A. `code.String()` 字符串（如 "rate_limit"） | IM 端 switch case 用字符串 | 受控性弱（容易拼错） |
| B. 整数（如 2） | 闭集 | 日志/调试不直观 |
| C. `code.String()` + D1 端用 `sharederrors.ParseAPIErrorCode(s)` 反解 | 序列化友好 + 编译期保护消费端 | 反解失败兜底 `Unknown` |

**选择:** C
**理由:** Metadata 是 `map[string]string` 已有约束；字符串是唯一选择；D1 端反解保证受控枚举语义（AC7 硬要求）。

#### Decision 4: withheld 状态归属

**选项:**
| 方案 | 优点 | 缺点 |
|------|------|------|
| A. `TurnState.Withheld bool`（per-session state） | 简单；与现有 TurnState 字段一致 | 仅限当前 session；重启丢失 |
| B. Per-message marker（写到 transcript） | 跨重启可见 | transcript 是只读审计，污染数据模型 |

**选择:** A
**理由:** 与 proposal §3.4 Decision "withheld in-memory only" 一致；session 重启后等价于重新进入 withhold 路径（FoldAssistantOutput 会重跑）。

## 3. Key Interfaces / Types

### 3.1 新增 `sharederrors.APIErrorCode`（闭集枚举）

```go
// internal/shared/errors/api_code.go

// APIErrorCode is the closed-set enumeration of LLM provider API error categories.
// v1.2 (DM-20260628-001): 7 classes aligned with clawcode's
// categorizeRetryableAPIError + OpenAI/Anthropic HTTP status semantics.
type APIErrorCode int

const (
    APICodeUnknown APIErrorCode = iota // 0 — default; JSON-friendly zero value
    APICodeRateLimit                   // 1 — HTTP 429
    APICodeAuthenticationFailed        // 2 — HTTP 401/403
    APICodeServerError                 // 3 — HTTP 5xx, 529
    APICodeMediaSize                   // 4 — Anthropic media_too_large / image_too_large
    APICodePromptTooLong               // 5 — HTTP 408/413
    APICodeImageSize                   // 6 — Image-specific size limit
)

// String returns the canonical lowercase name (e.g., "rate_limit").
// Used for log, metadata, and IM adapter switch.
func (c APIErrorCode) String() string { ... }

// NewAPIErrorCodeFromStatus maps an HTTP status code to APIErrorCode.
//   401/403 → AuthenticationFailed
//   408/413 → PromptTooLong
//   429     → RateLimit
//   5xx/529 → ServerError
//   other   → Unknown
func NewAPIErrorCodeFromStatus(status int) APIErrorCode { ... }

// ParseAPIErrorCode is the inverse of String. Unknown values map to APICodeUnknown.
func ParseAPIErrorCode(s string) APIErrorCode { ... }
```

### 3.2 新增 `sharederrors` 三 API（包装链感知）

```go
// Code extracts the APIErrorCode from an error chain.
// Returns APICodeUnknown if no APIErrorCode-tagged error is found.
func Code(err error) APIErrorCode { ... }

// IsCode reports whether err or any error in its chain has the given APIErrorCode.
func IsCode(err error, code APIErrorCode) bool { ... }

// WithAPIErrorCode is a convenience wrapper for WithCode(code.String(), msg, err).
// Existing WithCode(code, msg, err) string API remains for legacy callers.
func WithAPIErrorCode(code APIErrorCode, msg string, cause error) error { ... }
```

### 3.3 新增 `llmgateway.APIError` 结构

```go
// internal/layers/llmgateway/api_error.go

// APIError represents a typed LLM provider HTTP error.
// Code is the closed-set classification (DM-20260628-001 AC1).
type APIError struct {
    Status  int
    Message string
    Code    sharederrors.APIErrorCode
    Cause   error
}

func (e *APIError) Error() string  { ... }
func (e *APIError) Unwrap() error  { ... }

// NewAPIError creates an APIError with auto-mapped Code from HTTP status.
func NewAPIError(status int, message string) *APIError {
    return &APIError{
        Status:  status,
        Message: message,
        Code:    sharederrors.NewAPIErrorCodeFromStatus(status),
    }
}
```

### 3.4 修改 `OrchestratorDeps`（字段预留）

```go
// internal/layers/orchestration/sessionorchestrator/turn_orchestrator.go

type OrchestratorDeps struct {
    // ... existing 14 fields ...
    // FallbackModel is the optional secondary model used when primary model
    // returns RateLimit/ServerError ≥ 2 consecutive times.
    // EMPTY = fallback disabled; FULL LOGIC implemented in P0-2 follow-up
    // (devrix-streaming-fallback). S4 only logs fallback_trigger_candidate
    // + fallback_model_set_but_not_yet_wired for observability.
    FallbackModel string
}
```

### 3.5 新增 `TurnState.Withheld`（per-session in-memory）

```go
// (in sessionorchestrator or turn_state.go; field on existing TurnState struct)
type TurnState struct {
    // ... existing fields ...
    Withheld bool // true while a prompt_too_long/media_size error is being recovered
}
```

### 3.6 emitError 路径用 `sharederrors.Code(err)`

```go
// internal/layers/orchestration/sessionorchestrator/helpers.go (modified)

func emitError(ctx context.Context, sink EventPublisher, out chan<- *contracts.EngineEvent, sessionID, label string, err error) {
    apiCode := sharederrors.Code(err) // DM-20260628-001 AC7 — controlled enum
    sanitized := sharederrors.SanitizeForUser(err)
    ev := &contracts.EngineEvent{
        Type:      "error",
        Content:   fmt.Sprintf("%s: %s", label, sanitized),
        SessionID: sessionID,
        Metadata: map[string]string{
            "error_code": apiCode.String(), // controlled string from closed-set
        },
    }
    emit(ctx, sink, out, ev)
}
```

### 3.7 D1 IM 适配器差异化文案

```go
// internal/layers/communication/channel/adapters/feishu.go (modified, line ~149-162)

case "error":
    // /stop 触发的 context cancellation
    if strings.Contains(content, "context canceled") {
        a.clearSessionStream(msg.SessionID)
        return
    }
    // DM-20260628-001 AC5: error_code driven differentiation
    body := formatErrorByCode(msg.Metadata["error_code"], content)
    card := NewCard().Title("错误", "red").Markdown(body).Build()
    if err := a.sendCardToSession(ctx, msg.SessionID, msg.ChatID, card); err != nil {
        slog.Error("feishu: failed to send error card", "error", err)
        return
    }
    a.clearSessionStream(msg.SessionID)

// formatErrorByCode maps the closed-set APIErrorCode string to user-facing copy.
func formatErrorByCode(code, fallback string) string {
    switch sharederrors.ParseAPIErrorCode(code) {
    case sharederrors.APICodeRateLimit:
        return "⚠️ 模型繁忙，请稍候重试"
    case sharederrors.APICodeAuthenticationFailed:
        return "🔑 API key 失效，请检查 ~/.devrix/config.yaml"
    case sharederrors.APICodePromptTooLong:
        return "📦 会话过长，已尝试压缩"
    case sharederrors.APICodeMediaSize, sharederrors.APICodeImageSize:
        return "📎 文件/图片过大，请缩小后重试"
    case sharederrors.APICodeServerError:
        return "🔧 服务暂时不可用，请稍候重试"
    default:
        return fallback // existing unified copy
    }
}
```

cli.go / cli renderers 中加入对称的 `formatErrorByCode` 调用（同样的 switch 结构）。

## 4. Data Flow

### 4.1 错误构造链路（D3 adapter → D7 orchestrator）

```
[Provider HTTP Response 4xx/5xx]
    ↓ adapter HTTP error site (openai_stream.go:79/86)
[4 个 adapter] minimax / deepseek / anthropic / openai
    ↓ 统一走 NewAPIError(status, msg) — AC2
[*llmgateway.APIError{Status, Message, Code, Cause}]
    ↓ sharederrors.WithAPIErrorCode(code, msg, apiErr) — 包装便于 errors.As
[wrapped error chain]
    ↓ 跨层传递到 D7 orchestrator
[D7 emitError]
    ↓ sharederrors.Code(err) → APIErrorCode
    ↓ apiCode.String() 填到 Event.Metadata["error_code"]
[*contracts.EngineEvent{Type:"error", Metadata:{"error_code":"rate_limit"}}]
    ↓ 沿 EventPublisher 推给 IM 适配器
[D1 feishu.go / cli.go]
    ↓ sharederrors.ParseAPIErrorCode("rate_limit") → APICodeRateLimit
    ↓ switch case 走差异化文案
[用户飞书卡片 / 终端输出]
```

### 4.2 Fallback 触发链路（D7 orchestrator 内部）

```
[Turn N: 主模型 529 ServerError]
    ↓ emitError 已发，session 仍存活
[TurnState.ConsecutiveServerErrors++]
    ↓ 如果 == 2 且 FallbackModel != ""
[log "fallback_trigger_candidate"]
    ↓ 如果 FallbackModel == ""
[log "fallback_model_set_but_not_yet_wired"]
[session 继续，Turn N+1 仍用主模型]
```

**注：** 完整 fallback 切换循环（P0-2 follow-up）放 `devrix-streaming-fallback` change。

### 4.3 Withhold-then-Recover 链路

```
[Turn N: 主模型 prompt_too_long]
    ↓ adapter 返回 APIError{Code: APICodePromptTooLong}
[D7 emitError 调用前]
    ↓ if code == APICodePromptTooLong { state.Withheld = true; return }
[emitError 不被调用]
    ↓ Turn N+1 进入 prepareContext 阶段
[D2 FoldAssistantOutput] 折叠超长 assistant 消息
    ↓ if success → state.Withheld = false; continue turn
    ↓ if fail → emitError({error_code: "prompt_too_long"}, 已 withhold=true)
[用户看到 "会话过长，已尝试压缩"]
```

## 5. File Manifest

### 5.1 新增 (4 files)

| 路径 | 行数估算 | 说明 |
|------|----------|------|
| `internal/shared/errors/api_code.go` | ~80 | APIErrorCode 闭集枚举 + String + NewAPIErrorCodeFromStatus + ParseAPIErrorCode + Code/IsCode/WithAPIErrorCode 三 API |
| `internal/shared/errors/api_code_test.go` | ~120 | 12+ case：7 enum 字符串 + 5 status 映射边界 + 3 IsCode 包装链 |
| `internal/layers/llmgateway/api_error.go` | ~50 | APIError struct + NewAPIError 工厂 + Error/Unwrap |
| `internal/layers/llmgateway/api_error_test.go` | ~60 | 4 status 映射 + 包装链测试 |
| `tests/integration/llm_fallback_e2e_test.go` | ~150 | AC8 E2E：mock 主模型 3 次 529 → fallback 切到次模型 → 完成 turn |

### 5.2 修改 (12 files)

| 路径 | 修改摘要 | 行数变化 |
|------|----------|----------|
| `internal/layers/llmgateway/stream/adapter/openai_stream.go` | L86-88 改用 NewAPIError + WithAPIErrorCode | +5/-2 |
| `internal/layers/llmgateway/stream/adapter/minimax.go` | 同上模式 | +5/-2 |
| `internal/layers/llmgateway/stream/adapter/deepseek.go` | 同上模式 | +5/-2 |
| `internal/layers/llmgateway/stream/adapter/anthropic.go` | 同上模式 | +5/-2 |
| `internal/layers/orchestration/sessionorchestrator/turn_orchestrator.go` | OrchestratorDeps +FallbackModel + DefaultOrchestrator 字段 + emitError 路径填 error_code | +20/-3 |
| `internal/layers/orchestration/sessionorchestrator/helpers.go` | emitError 用 sharederrors.Code | +8/-2 |
| `internal/layers/orchestration/sessionorchestrator/turn_state.go`（或等价） | TurnState +Withheld bool | +3/-0 |
| `internal/layers/communication/channel/adapters/feishu.go` | "error" case 用 formatErrorByCode | +30/-5 |
| `internal/layers/communication/channel/adapters/feishu_error_format.go`（或合并 feishu.go） | formatErrorByCode 函数 | +25/-0 |
| `internal/layers/communication/channel/adapters/cli.go` | OnError 路径同步差异化文案 | +15/-2 |
| `internal/layers/communication/channel/renderers/message.go` | RenderError 支持 error_code | +8/-2 |

**注**：adapter 文件可能在 S4 阶段确认 minimax.go / anthropic.go / deepseek.go 是否独立存在（S2 阶段从 openai_stream.go 类比推断）。

### 5.3 文档同步 (5 files)

| 路径 | 变更 |
|------|------|
| `openspec/specs/d3-llm-gateway/spec.md` | + `## 14. ADDED Requirements (V4 API 错误分类 — DM-20260628-001)`，8 FR |
| `openspec/specs/d7-orchestration/spec.md` | + `## (对应章节)`，2 FR（D7-S2-A50 FallbackModel + Withheld） |
| `openspec/specs/d1-communication/spec.md` | + `## (对应章节)`，1 FR（IM 差异化文案） |
| `openspec/changes/devrix-api-error-classification/.openspec.yaml` | status: s3_design |
| `openspec/changes/devrix-api-error-classification/tasks.md` | S4 阶段创建，T 编号映射 |

### 5.4 不变更（明确）

- `internal/shared/errors/llm.go` — 不修改；`LLMError = SentinelError` 类型别名保留
- `internal/shared/errors/communication.go` — `WithCode`/`SentinelError` 不变；新增包装在 api_code.go
- `internal/shared/errors/redact.go` — `SanitizeForUser` 不变（AC6 兼容性）
- 所有现有 30+ `SanitizeForUser` 调用点

## 6. Regression Risk Assessment

| 风险 | 影响 | 概率 | 缓解 | T 验证 |
|------|------|------|------|--------|
| **R-1** 新 enum 与现有 `CodeLLM*` 字符串冲突 | 中 | 低 | APIErrorCode 是 `int` 类型，独立命名空间 `APICode*`；`WithCode` 仍接受字符串 | D3-S1-A01-T04/T05 单测覆盖 |
| **R-2** adapter 改造 diff 大 → 合并冲突 | 低 | 中 | 4 adapter 模式高度相似，统一走 NewAPIError 后冲突面收敛 | D3-S3-A01-T17 单测覆盖 4 adapter |
| **R-3** `OrchestratorDeps.FallbackModel` 预留但未启用 → 用户误解 | 低 | 高 | 日志打 `fallback_model_set_but_not_yet_wired` 显式标注 | D7-S2-A50-T05/T06 单测验证日志存在 |
| **R-4** `withheld=true` 未持久化 → session 重启后丢失 | 中 | 中 | 设计已声明 in-memory only（proposal §3.4 Decision 2）；session 重启等价于重新进入 withhold | D7-S2-A50-T06 单测不验证持久化（明确范围外） |
| **R-5** `Event.Metadata["error_code"]` 现有调用方未读取 | 低 | 低 | Metadata 是 `map[string]string` map，缺失字段无副作用；D1 IM 端新增 reader | D1-S3-A08-T01 单测覆盖 5 类 code |
| **R-6** emitError 增加 error_code 后 IM 端未及时解析 → 显示空文案 | 中 | 中 | `formatErrorByCode` 默认 case 返回原 `content`（向后兼容） | AC5 7 个 case 全覆盖 + 兜底 case |
| **R-7** APIError struct 字段顺序变化破坏 ABI（cgo） | 极低 | 极低 | devrix 不导出 cgo；struct 不跨进程边界 | 无需验证 |

## 7. Rollback Plan

### 7.1 触发条件

- AC1-AC8 任一核心功能未通过 P0 T 验证
- SanitizeForUser 30+ 调用点回归测试失败（AC6）
- feishu/cli IM 文案出现 KeyError/空字符串/崩溃

### 7.2 回滚步骤（按影响面排序）

| 步骤 | 操作 | 影响范围 | 可逆性 |
|------|------|----------|--------|
| 1 | revert 主分支 PR（squash merge → revert commit） | 全部代码 + 测试 | 高（git revert） |
| 2 | 重新激活旧 SanitizeForUser 路径 | emitError helper | 高 |
| 3 | 清掉 orchestrator_deps.FallbackModel 字段引用 | turn_orchestrator.go | 高 |
| 4 | D1 IM 适配器回退到无 error_code 分支 | feishu.go / cli.go / renderers/message.go | 高 |

### 7.3 部分回滚（feature flag）

若 FallbackModel 字段引起问题，可在 S4 阶段加 `d3_fallback_model_wired` feature flag（默认 OFF）。S4 评审时确认是否纳入。

### 7.4 数据兼容性

- `Event.Metadata["error_code"]` 是新增字段，旧调用方读取未知 key 无副作用
- `llmgateway.APIError.Code` 是新增字段，旧读取 `Status`/`Message` 的调用方零行为变化
- `OrchestratorDeps.FallbackModel` 是新增字段，旧调用方构造时不填等价于空字符串（fallback disabled）

---

**S3 完成。下一步：S3-Gate（按 `review-design.md` §2 四维度自检：问题根因 / 方案设计 / 文件清单 / 回归风险）。**
