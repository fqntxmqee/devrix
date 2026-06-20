# Design: Error Handling Design — Tier 1 + Tier 2

**Change ID:** devrix-error-handling-tier1-tier2
**Demand ID:** DM-20260620-003
**Status:** S7_Archived
**Date:** 2026-06-20

## 1. Root Cause Analysis

### 1.1 Sentinel 链跨域丢失的根因

D7 `DefaultOrchestrator.runLoop` 在 nested 分支（line 230+）和 main 分支（line 285+）都使用 `emitError(out, sessionID, fmt.Sprintf("...: %v", err))` 模式：

```go
// internal/layers/orchestration/turn/orchestrator.go:256 (and 292/371/428/568)
o.emitError(out, req.SessionID, fmt.Sprintf("prepare failed: %v", err))
//                                                    ^^^  %v 而非 %w
//                                                    进一步：err.Error() 字符串拼接
```

**根因**：
1. `emitError` 函数签名只接 `content string`（line 692），不接 `error` 类型 → 错误在 emit 时已变成 string
2. `%v` 而非 `%w` → 即使保留 err 也不参与 `errors.Is/As` 链路
3. D1 渲染时无法 `errors.Is` 区分错误类型 → 只能显示固定文案

### 1.2 IM 错误泄漏根因

D1 `feishu.go:237` 和 `:827` 直接拼接 `err.Error()`：

```go
// feishu.go:237
card := NewCard().Title("错误", "red").Markdown(err.Error()).Build()
// feishu.go:827
"⚠️ 消息处理失败："+err.Error()+"\n请重试，或发送 /new 开启新会话。"
```

**根因**：
- 没有 user-facing 错误信息规范化层
- `sharederrors.NewLLMUnavailableError` 已有 `truncateLLMUserMessage` 模式，但仅在 D2 prepare 路径使用
- D7 runLoop emit 后的 string 不再享受该规范化

### 1.3 类型双胞胎根因

`LLMError{Code, Message, Err}` (llm.go:37) 与 `SentinelError` (communication.go:38) 字段完全相同但**类型不同**。

**根因（历史）**：
- `SentinelError` 先存在（communication 域）
- `LLMError` 后加入（llm 域）—— 当时为快速独立设计，复用字段结构
- `errorclass/classifier.go:94-138` 必须双通道匹配：`errors.Is(sentinel)` + `LLMError.Code == X`
- `retry.go:86` `IsRetryable` 也必须双查

**根因（设计债）**：
- 错误处理要求"统一 sentinel 链"，但实际是双类型
- 任何新域加入都必须先决定"用哪个类型"，无统一入口

### 1.4 Silent fallback 根因

`task_manager.go:127` 返回签名 `*Task` 而非 `(*Task, error)`：

```go
// workmodel/task_manager.go:127 (示意)
if err != nil { return nil }  // ← 静默吞
```

**根因**：
- 当时为简化 API，但**静默吞错违反 CLAUDE.md "错误处理: ...禁止 panic 用于业务错误" + common/coding-style.md "Never silently swallow errors"**
- 调用方 `_, _ = o.taskManager.EnsureGoal(...)` 进一步"双下划线吞"形成叠加

### 1.5 nil-sentinel 包装根因

`retry.go:91-93` 在 retry 跑完所有 attempt 后：

```go
if lastErr == nil {
    lastErr = sharederrors.NewProviderUnavailableError(nil)  // ← wrap nil
}
return nil, lastErr
```

**根因**：
- 当时为"避免返回 nil err"，但用 sentinel wrap nil 是**反模式**：
  - `IsRetryable` 命中 `ErrProviderUnavailable` → 上游又进 retry loop
  - `errors.Is(err, ErrXxx)` 命中但 unwrap 是 nil → 实际无错误信息
  - 用户看到 "llm provider unavailable" 但无具体 reason

## 2. Solution Design

### 2.1 Tier 1 总体方案

```
                            ┌──────────────────────────────────┐
                            │ sharederrors.SanitizeForUser(err)│  ← 单一 SoT
                            │  - redact API key / token        │
                            │  - redact /Users/... /home/...   │
                            │  - truncate 240                  │
                            └──────────────────────────────────┘
                                          ▲
                                          │ 调用
                                          │
[3 处错误源]                  [3 处渲染点]
D7 emitError (×5)   ──┐      D1 feishu.go:237 Markdown
D7 subturn wrap (×2)──┼──►   D1 feishu.go:827 replyAck
D7 orchestrate_path  ─┘      D1 conclusion.EmitError (future)
```

### 2.2 Tier 2 总体方案

```
[旧: 双类型]
LLMError{Code, Message, Err}          SentinelError{Code, Message, Err}
   ↓ (调用方)                             ↓ (调用方)
   errors.Is(err, ErrLLMAuthFailed)  errors.Is(err, ErrSessionNotFound)
   errors.As(err, &LLMError{...})    errors.As(err, &SentinelError{...})
        ↓
        [Tier 2 后: 单一类型]
        Error{Code, Message, Err, IsRetryable func() bool}
            ↓ (调用方)
            errors.Is(err, ErrLLMAuthFailed) ← 平起平坐
            errors.As(err, &Error{...})      ← 单一类型
            sharederrors.IsRetryable(err)    ← 单一通道
        兼容: LLMError / SentinelError 保留为 type alias (1 minor release)
```

### 2.3 SanitizeForUser 设计

```go
// internal/shared/errors/redact.go
package errors

import (
    "regexp"
    "strings"
)

const sanitizeMaxLen = 240

// redaction patterns (顺序敏感：先长后短)
var sanitizePatterns = []*regexp.Regexp{
    // Bearer tokens
    regexp.MustCompile(`(?i)(bearer\s+)[A-Za-z0-9._\-]{20,}`),
    // sk-xxx, sk-ant-xxx style API keys
    regexp.MustCompile(`(?i)\bsk-[A-Za-z0-9\-_]{16,}\b`),
    // ghp_xxx, gho_xxx GitHub tokens
    regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{20,}\b`),
    // xoxb-xxx, xoxp-xxx Slack tokens
    regexp.MustCompile(`\bxox[baprs]-[A-Za-z0-9-]{10,}\b`),
    // AWS access keys
    regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`),
    // File paths (absolute)
    regexp.MustCompile(`/(?:Users|home)/[^\s:'"]+`),
    // Hex/base64 blobs > 64 chars
    regexp.MustCompile(`\b[A-Za-z0-9+/]{64,}={0,2}\b`),
}

const sanitizeRedacted = "[REDACTED]"

// SanitizeForUser returns a user-safe string from err.Error().
// Redacts known sensitive patterns (API keys, tokens, file paths) and
// truncates to sanitizeMaxLen (240) characters.
func SanitizeForUser(err error) string {
    if err == nil {
        return ""
    }
    s := err.Error()
    for _, re := range sanitizePatterns {
        s = re.ReplaceAllString(s, sanitizeRedacted)
    }
    s = strings.TrimSpace(s)
    if len(s) > sanitizeMaxLen {
        s = s[:sanitizeMaxLen] + "..."
    }
    return s
}
```

**关键不变量**：
- 已知敏感 pattern 才 redact（白名单 + 黑名单混合，避免误删正常信息）
- truncate 在 redact 之后，保证最后 240 字符含 redact 标记
- 永远返回非空字符串（除 err == nil），保证 IM 渲染有内容

### 2.4 Error 类型合并设计

```go
// internal/shared/errors/error.go
package errors

import "errors"

// Error is the unified sentinel error type. Replaces both the legacy
// LLMError and SentinelError (now type aliases for 1 minor release).
type Error struct {
    Code    string  // e.g. "LLM_AUTH_1004"
    Message string  // human-readable summary
    Err     error   // wrapped cause
}

func (e *Error) Error() string {
    if e.Message != "" {
        return e.Message
    }
    if e.Err != nil {
        return e.Err.Error()
    }
    return e.Code
}

func (e *Error) Unwrap() error {
    return e.Err
}

// IsCode reports whether err is an *Error with the given code.
// Use this instead of `errors.Is(err, sentinel)` for code-based dispatch.
func IsCode(err error, code string) bool {
    var e *Error
    if !errors.As(err, &e) {
        return false
    }
    return e.Code == code
}

// WithCode constructs an *Error with the given code, message and cause.
func WithCode(code, message string, err error) *Error {
    return &Error{Code: code, Message: message, Err: err}
}

// IsRetryable reports whether the error should trigger a retry.
// Unified single-channel — works for LLM + Communication + Context + Agent
// domains via the *Error type.
func IsRetryable(err error) bool {
    if err == nil {
        return false
    }
    var e *Error
    if errors.As(err, &e) {
        switch e.Code {
        case CodeLLMTimeout, CodeLLMCircuitOpen, CodeLLMProviderUnavailable,
             CodeContextExceeded, CodeSnapshotCorrupt:
            return true
        }
    }
    return false
}
```

**Migrate design**:
```go
// internal/shared/errors/migrate.go
package errors

// LLMError is a deprecated alias for *Error. Kept for 1 minor release.
// Existing call sites can continue using errors.As(err, &LLMError{}) and
// type assertions until the next minor version removes these aliases.
type LLMError = Error

// SentinelError is a deprecated alias for *Error. Kept for 1 minor release.
type SentinelError = Error
```

**Migration plan**:
1. 添加 `*Error` 类型 + `IsCode` + `IsRetryable` 单一通道
2. `LLMError` / `SentinelError` 改为 `type X = Error` alias
3. 所有现有工厂函数（`NewLLMAuthFailed` / `NewSessionNotFound` 等）保持不变 —— 内部返回 `*Error`
4. 编译时所有 `errors.As(err, &LLMError{...})` 自动识别为 `*Error`
5. 下次 minor release 删除 alias

### 2.5 silent fallback 修复设计

**task_manager.go:127**:
```go
// 旧
func CreateTask(...) *Task {
    if err != nil { return nil }
    return task
}

// 新
func CreateTask(...) (*Task, error) {
    if err != nil {
        return nil, fmt.Errorf("taskmanager: create: %w", err)
    }
    return task, nil
}
```

**调用方适配**：所有 `task := o.taskManager.CreateTask(...)` 改为 `task, err := o.taskManager.CreateTask(...); if err != nil { ... }`

**EnsureGoal 修复**:
```go
// 旧
_, _ = o.taskManager.EnsureGoal(...)
// 新
if ok, err := o.taskManager.EnsureGoal(...); err != nil {
    slog.Warn("orchestrator: ensure goal failed", "session_id", sessionID, "error", err)
    // 不改变控制流，但落 metadata
}
```

**classifier_fallback 修复**:
```go
// 旧
if err != nil { return ruleResult, nil }
// 新
if err != nil {
    slog.Warn("decisionplanning: LLM classify failed, using rule fallback",
        "session_id", sessionID, "error", err)
    metadata["classify_fallback"] = "rule"
    return ruleResult, nil
}
```

### 2.6 nil-sentinel 修复设计

```go
// 旧
if lastErr == nil {
    lastErr = sharederrors.NewProviderUnavailableError(nil)  // wrap nil
}
return nil, lastErr

// 新
if lastErr == nil {
    return nil, fmt.Errorf("llm: all retry attempts exhausted without captured error (provider=%s)", e.provider)
}
return nil, lastErr  // lastErr 此时保证非 nil
```

**Test 覆盖**:
```go
func TestRetry_NilLastErr_ReturnsExhaustedError(t *testing.T) {
    // retry 路径全程不设 lastErr (mock 异常情况)
    _, err := retryExecutor.Run(ctx, llmRequest)
    require.Error(t, err)
    require.NotContains(t, err.Error(), "provider unavailable")
    require.Contains(t, err.Error(), "retry attempts exhausted")
}
```

### 2.7 emitError 签名扩展设计

```go
// 旧
func (o *DefaultOrchestrator) emitError(out chan<- *contracts.EngineEvent, sessionID, content string) {
    out <- &contracts.EngineEvent{
        Type:      "error",
        Content:   content,
        SessionID: sessionID,
    }
}

// 新（向后兼容：可选 code string）
func (o *DefaultOrchestrator) emitError(out chan<- *contracts.EngineEvent, sessionID, content string, code ...string) {
    metadata := map[string]string{}
    if len(code) > 0 && code[0] != "" {
        metadata["error_code"] = code[0]
    }
    out <- &contracts.EngineEvent{
        Type:      "error",
        Content:   content,
        SessionID: sessionID,
        Metadata:  metadata,
    }
}
```

**调用模式**:
```go
o.emitError(out, sessionID, sharederrors.SanitizeForUser(err), sharederrors.ErrorCode(err))
// output: event.Content = "（sanitized msg）", event.Metadata["error_code"] = "LLM_AUTH_1004"
```

## 3. Key Interfaces / Types

### 3.1 SanitizeForUser

```go
// internal/shared/errors/redact.go
func SanitizeForUser(err error) string
```

### 3.2 Error (unified)

```go
// internal/shared/errors/error.go
type Error struct {
    Code    string
    Message string
    Err     error
}

func (e *Error) Error() string
func (e *Error) Unwrap() error

func WithCode(code, message string, err error) *Error
func IsCode(err error, code string) bool
func IsRetryable(err error) bool
func ErrorCode(err error) string  // 新增：与 IsCode 配合使用
```

### 3.3 emitError 扩展

```go
// internal/layers/orchestration/turn/orchestrator.go
func (o *DefaultOrchestrator) emitError(
    out chan<- *contracts.EngineEvent,
    sessionID, content string,
    code ...string,
)
```

## 4. Data Flow

### 4.1 Tier 1 IM 渲染流程

```
[3rd-party LLM Provider]
    ↓ raw error (可能含 API key + 路径)
[D3 llmgateway.Gateway.Stream]
    ↓ classifyAndWrap → wrap with %w + classification
[*sharederrors.Error (Code=LLM_AUTH_1004, Message="...")]
    ↓ IsRetryable checks
[D7 runLoop]
    ↓ emitError(..., sharederrors.SanitizeForUser(err), sharederrors.ErrorCode(err))
    ↓
[EngineEvent{Type: error, Content: "[redacted]", Metadata: {error_code: "LLM_AUTH_1004"}}]
    ↓
[D1 conclusion.EmitError → OutboundMessage{Content, Metadata}]
    ↓
[D1 feishu.OnMessage → sendCard]
    ↓ Markdown(content)  ← 此时 content 已 sanitized
[飞书卡片]  ← 用户看到 [REDACTED] 替代敏感信息 + 含 error_code 可后续做差异化提示
```

### 4.2 Tier 2 类型合并后错误流

```
[任意域错误源]
    ↓
[sharederrors.WithCode(code, msg, cause) → *Error]
    ↓
[跨域传播: %w 包装]
    ↓
[调用方]
    errors.Is(err, ErrLLMAuthFailed)  ← 命中
    errors.As(err, &e)                ← e 是 *Error 类型
    sharederrors.IsCode(err, CodeLLMAuthFailed) ← true
    sharederrors.IsRetryable(err)     ← 走单一通道
[Logger / Metric / Tracing]
    e.Code  → structured logging
    e.Message → user-facing
    e.Err → cause for tracing
```

## 5. File Manifest

### 5.1 新增

| 文件 | 用途 | LoC |
|------|------|-----|
| `internal/shared/errors/redact.go` | SanitizeForUser helper | +50 |
| `internal/shared/errors/redact_test.go` | 8+ redact test cases | +100 |
| `internal/shared/errors/error.go` | Unified *Error type | +80 |
| `internal/shared/errors/migrate.go` | LLMError/SentinelError alias | +40 |
| `internal/shared/errors/error_test.go` | Unified type behavior tests | +120 |
| `docs/error-handling.md` | 设计综述 | +200 |
| `openspec/specs/shared-errors/t-registry.md` | shared-errors 域 T 注册表 | +30 |

### 5.2 修改

| 文件 | 改动 |
|------|------|
| `internal/layers/orchestration/turn/orchestrator.go:256/292/371/428/568` | emitError 改 sanitize + 加 code 参数 |
| `internal/layers/orchestration/turn/orchestrator.go:692` | emitError 签名扩展 |
| `internal/layers/orchestration/turn/subturn.go:323/354` | 改 sharederrors.Wrap |
| `internal/layers/orchestration/sessionorchestrator/orchestrate_path.go:118` | `%v` → `%w` |
| `internal/layers/communication/channel/adapters/feishu.go:237/827` | 改 SanitizeForUser |
| `internal/layers/llmgateway/protect/retry.go:91-93` | nil-sentinel 修复 |
| `internal/layers/orchestration/workmodel/task_manager.go:127` | 返回 `(*Task, error)` |
| `internal/layers/orchestration/decisionplanning/classifier_fallback.go:73` | 加 slog.Warn + metadata |
| `internal/layers/orchestration/sessionorchestrator/orchestrator.go:270` | EnsureGoal 错误传播 |
| `internal/layers/orchestration/turn_adapter/ltl_hook.go:28` | 移到 shared/errors |
| `internal/layers/llmgateway/stream/gateway.go:110` | classifyAndWrap 前 inject ctx |
| `internal/layers/observability/observability.go:164` | fmt.Errorf → Wrap |
| `internal/shared/errors/llm.go` | 改 type alias |
| `internal/shared/errors/communication.go` | 改 type alias |
| 跨域调用方 (orchestrator, llmgateway, communicate) | 适配方类型断言 |

### 5.3 删除

- 无（保留 alias 用于 deprecation period）

## 6. Regression Risk Assessment

| 风险 | 概率 | 影响 | 缓解 |
|------|------|------|------|
| emitError 新签名打破调用方（variadic 实际兼容） | Low | Low | variadic 不破坏；现有 `emitError(a, b, c)` 调用照常工作 |
| 类型合并后 `errors.As` 行为微妙变化 | Low | Med | migrate.go 保 1 minor release；`IsCode` 替代 `Code == X` 字面比较；全量 unit test 跑 |
| `SanitizeForUser` 误删合法错误信息 | Low | Med | 仅 redact 明确敏感 pattern；测试覆盖 happy path（含 "Bearer" 等正常英文） |
| `task_manager` 返回签名变更触发级联 | Med | Med | 一次性全仓 sed 改调用方；CI 全绿即安全 |
| 4 PR 顺序合并 → 中间状态 master 不绿 | Low | Low | 每个 PR 独立可测试；PR-A 与 PR-B/C 之间无依赖，PR-C 在 PR-B 后 |
| `EnsureGoal` 错误传播让 orchestrator 路径炸 | Low | High | 仅加 slog.Warn + metadata，不改控制流；CI 全绿即安全 |

## 7. Rollback Plan

- **Tier 1 (PR-A)**: revert commit `feat(redact+sentinel-recover)` 即可
- **Tier 2 PR-B (类型合并)**: revert commit `refactor(shared-errors-unify)`；alias 删除会导致编译错误 → 需保留 alias 一段时间
- **Tier 2 PR-C (silent fallback)**: revert commit `fix(silent-fallback)`；新签名 `(*Task, error)` 调用方回到旧签名需逐个改回
- **PR-D (docs + 归档)**: revert 即可

每个 PR 单独 squash + revert 独立。

## 8. Verification

```bash
# PR-A
go test -race ./internal/shared/errors/...  # 8+ redact cases
go test -race ./internal/layers/orchestration/turn/...  # emitError 现有测试
go test -race ./internal/layers/llmgateway/protect/...   # retry nil-sentinel
go test -race ./internal/layers/communication/channel/adapters/  # feishu sanitize

# PR-B
go test -race ./internal/shared/errors/...  # 全部重写后的测试
go build ./...                              # 跨域类型断言适配

# PR-C
go test -race ./internal/layers/orchestration/workmodel/...  # task_manager 新签名
go test -race ./internal/layers/orchestration/decisionplanning/  # classifier fallback
go test -race ./internal/layers/observability/...  # observability Wrap

# PR-D (S6)
./scripts/verify-archive.sh devrix-error-handling-tier1-tier2
```

**手工验证（用户视角）**:
1. 启动 devrix，发送一条无效 prompt 让 provider 返回 401
2. 检查飞书卡片：error 文案含 "[REDACTED]" 替代 API key
3. grep IM card: `Markdown(err.Error())` 数量从 2 → 0
4. 全量回归: `./scripts/test-all.sh`

## 9. S3-Gate Self-Check

按 `review-design.md` §2 四个维度：

- [x] 层归属正确：sharederrors 公共域，跨 D1/D2/D3/D7 调用 — 通过 `contracts/` 接口
- [x] 接口方向正确：低层 sharederrors 不依赖高层 domain logic；domain 调用 sharederrors
- [x] 不重复造轮子：复用 Phase A `truncateLLMUserMessage` 模式扩展
- [x] 跨层依赖最小：SanitizeForUser 是 sharederrors 公共函数，零跨层依赖
- [x] 设计决策有记录：Decision 1/2/3 已记录理由
- [x] demand → proposal → design → specs 链路完整
- [x] 验收标准覆盖：14 AC → 4 PR，每 PR 至少 3 P0 AC
- [x] Out of Scope 明确
- [x] DM ID 无冲突：DM-20260620-003 当日序号递增
- [x] Gherkin 格式正确（见 specs/*/spec.md）
- [x] Happy + sad path 均有 Scenario
- [x] T 层映射完整：17 P0 T 点 + 注释
- [x] 回归风险已评估
- [x] 回滚方案可行

**S3-Gate 结论**: Approved
