# Implementation Tasks: Runtime 反馈链路闭环

**Change ID:** `devrix-runtime-feedback-closure`  
**Demand ID:** DM-20260704-003  
**Status:** S4_Implementation (Draft)  
**Design:** [`design.md`](design.md)

---

## T 点注册表（8 个新 P0 T 点）

| T ID | L5 | Phase | 描述 | 优先级 |
|------|-----|-------|------|--------|
| **D2-S15-A82-T01** | L5-D2-RFC-01 | A | `prompt_sections_zh.go` `intro` 段含"请始终用中文回复用户"硬规则 | P0 |
| **D2-S15-A82-T02** | L5-D2-RFC-02 | A | `prompt_sections_en.go` 不含中文硬规则（防英文污染对称测试） | P0 |
| **D2-S15-A82-T03** | L5-D2-RFC-03 | A | i18n golden test：zh/en prompt bytes 稳定（hash 比对） | P0 |
| **D5-S2-A01-T01** | L5-D5-RFC-01 | B | `tracingStepObserver.OnStep` 透传 ctx（`_, span :=` → `ctx, span :=`） | P0 |
| **D5-S2-A01-T02** | L5-D5-RFC-02 | B | Worker fork 边界 parent_span_id 100% 命中（3 case: trace fork / scheduler dispatch / child_downlink） | P0 |
| **D5-S2-A01-T03** | L5-D5-RFC-03 | B | tracer.Start fallback 路径 emit `span.orphan=true` + slog.Warn | P1 |
| **D7-S2-A50-T09** | L5-D7-RFC-01 | C | `turn_adapter.executeOne` 加 WithTimeout（默认 60s，env 可调） | P0 |
| **D7-S2-A50-T10** | L5-D7-RFC-02 | C | tool timeout 后 emit `tool.timeout.exceeded` span + 返 ErrToolTimeout | P0 |

## Phase 划分

### Phase A: i18n 中文硬规则 + golden test — 3 P0 T

> **目标：** ZH locale prompt 必含硬规则，EN locale 必不含。

#### A.1 修改 `prompt_sections_zh.go` `intro` 段 — `@L5(L5-D2-RFC-01)` `@T(D2-S15-A82-T01)`

- 在 `intro` 段插入：
  ```text
  请始终用中文回复用户，除非用户主动切到其他语言或明确要求。
  ```
- 在 `tone_and_style` 段追加：
  ```text
  - 可见输出（除代码标识符、文件路径、技术名词外）必须用中文
  ```

**验证**：
- `go test -race -count=1 ./internal/layers/contextengine/i18n/...` PASS
- `go test -race -count=1 ./internal/layers/contextengine/prepare/...` PASS

#### A.2 golden test (zh) — `@L5(L5-D2-RFC-03)` `@T(D2-S15-A82-T03)`

新增 `prompt_sections_zh_test.go`：
- 加载 `promptSectionsZH` map
- 计算 SHA256 → 与 `prompt_sections_zh.golden` 文件比对
- 显式断言 `intro` 段含 "请始终用中文回复用户"

**验证**：
- `go test -run TestPromptSectionsZH_Golden ./internal/layers/contextengine/i18n/...` PASS

#### A.3 golden test (en) — `@L5(L5-D2-RFC-02)` `@T(D2-S15-A82-T02)`

新增 `prompt_sections_en_test.go`：
- 加载 `promptSectionsEN` map
- 计算 SHA256 → 与 `prompt_sections_en.golden` 文件比对
- 显式断言 `intro` 段**不**含 "请始终用中文回复"

**验证**：
- `go test -run TestPromptSectionsEN_Golden ./internal/layers/contextengine/i18n/...` PASS

### Phase B: Tracing ctx 透传 + orphan marker — 2 P0 + 1 P1 T

> **目标：** parent_span_id 100% 命中；orphan span 可筛。

#### B.1 修改 `tracing_step_observer.go` — `@L5(L5-D5-RFC-01)` `@T(D5-S2-A01-T01)`

```go
// 之前
func (o *tracingStepObserver) OnStep(ctx context.Context, step string, before, after int) {
    _, span := o.startSpan(ctx, telemetry.OpD2_S2_Context_Compression_Step+"."+step, ...)
    if span != nil { span.End() }
    if o.inner != nil { o.inner.OnStep(ctx, step, before, after) }
}

// 之后
func (o *tracingStepObserver) OnStep(ctx context.Context, step string, before, after int) {
    ctx, span := o.startSpan(ctx, telemetry.OpD2_S2_Context_Compression_Step+"."+step, ...)
    if span != nil { span.End() }
    if o.inner != nil { o.inner.OnStep(ctx, step, before, after) }
}
```

**验证**：
- `go test -race -count=1 ./internal/layers/contextengine/prepare/compression/...` PASS
- 0 race detector warnings

#### B.2 端到端 parent-span continuity test — `@L5(L5-D5-RFC-02)` `@T(D5-S2-A01-T02)`

新增 `internal/layers/orchestration/sessionorchestrator/turn_loop/parent_span_test.go`：

3 case 覆盖：
1. **Trace fork** — `tracer.Start("D2_S15.prepare")` 后子 trace 仍能继承 sc
2. **Scheduler dispatch** — Worker fork 边界 sc 传递（mock orchtypes.SpanContext）
3. **Child downlink** — `child_downlink.go` 跨 goroutine ctx 传递

**断言**：
- `parent_span_id` == 父 span id（`String()` 比对）
- `go test -race` 0 race

**验证**：
- `go test -race -run TestParentSpan_ ./internal/layers/orchestration/...` PASS (3 case)

#### B.3 Orphan marker (P1) — `@L5(L5-D5-RFC-03)` `@T(D5-S2-A01-T03)`

修改 `tracer.go::Start` fallback 路径：
```go
if sc, ok := SpanContextFromContext(ctx); !ok || !sc.IsValid() {
    slog.Warn("orphan span", "operation", operation)
    // 仍创建 span，但附加 attribute
    attrs = append(attrs, Attribute{Key: "span.orphan", Value: "true"})
}
```

新增 `orphan_marker_test.go`：
- mock ctx 无 sc → Start 应返回 span + `span.orphan=true` attribute
- slog 输出验证

**验证**：
- `go test -run TestOrphanMarker ./internal/layers/observability/...` PASS

### Phase C: Tool-level timeout + fail-closed — 2 P0 T

> **目标：** tool 卡死 → 1s 内 fail-closed，Turn 不阻塞。

#### C.1 新增 `turn_adapter_timeout.go` — `@L5(L5-D7-RFC-01)` `@T(D7-S2-A50-T09)`

```go
// internal/bootstrap/turn_adapter_timeout.go
package bootstrap

import (
    "context"
    "errors"
    "time"
    
    "github.com/devrix/devrix/internal/shared/errors"
)

var ErrToolTimeout = errors.New("tool execution timeout")

func executeWithTimeout(parent context.Context, timeoutSeconds int, fn func(ctx context.Context) error) error {
    if timeoutSeconds <= 0 { timeoutSeconds = 60 }
    ctx, cancel := context.WithTimeout(parent, time.Duration(timeoutSeconds)*time.Second)
    defer cancel()
    
    err := fn(ctx)
    if errors.Is(ctx.Err(), context.DeadlineExceeded) {
        return ErrToolTimeout
    }
    return err
}
```

#### C.2 修改 `turn_adapter.go::executeOne` — `@L5(L5-D7-RFC-01)` `@T(D7-S2-A50-T09)`

```go
func executeOne(ctx context.Context, call ToolCall, cfg ExecuteConfig) (ToolResult, error) {
    err := executeWithTimeout(ctx, cfg.ToolTimeoutSeconds, func(callCtx context.Context) error {
        // 现有 tool call 逻辑
        return tool.Call(callCtx, call)
    })
    
    if errors.Is(err, ErrToolTimeout) {
        // emit ChannelRoute warn span
        emitChannelRoute(ctx, "tool.timeout.exceeded", map[string]any{
            "tool": call.Name,
            "timeout_seconds": cfg.effectiveTimeout(),
        })
        return ToolResult{Err: ErrToolTimeout}, nil
    }
    return ToolResult{}, err
}
```

#### C.3 timeout test — `@L5(L5-D7-RFC-02)` `@T(D7-S2-A50-T10)`

新增 `turn_adapter_timeout_test.go`：
- **TestExecuteOne_NormalTimeout** — 1s timeout，tool 0.5s 完成 → 正常返回
- **TestExecuteOne_TimeoutTriggered** — 1s timeout，tool 3s 阻塞 → 返 ErrToolTimeout + 1s ± 0.1s
- **TestExecuteOne_ConfigOverride** — cfg.ToolTimeoutSeconds = 2，验证生效
- **TestExecuteOne_DefaultTimeout** — cfg.ToolTimeoutSeconds = 0，验证 default 60s
- **TestExecuteOne_EmitSpan** — timeout 触发时 verify `tool.timeout.exceeded` span emit

**验证**：
- `go test -race -count=1 -run TestExecuteOne ./internal/bootstrap/...` PASS (5 case)
- 0 race detector warnings

### Phase D: Spec 增量 + 文档同步

#### D.1 spec 增量
- 新增 `openspec/specs/d2-context-engine/runtime-feedback-closure.md`
- 新增 `openspec/specs/d5-observability/parent-span-continuity.md`
- 新增 `openspec/specs/d7-orchestration/tool-call-timeout.md`

#### D.2 域文档 CHANGELOG
- 3 个域 `CHANGELOG.md` 各追加一行（vX.Y.Z 增量）
- 根 `openspec/t-registry.md` 追加 8 T 点

#### D.3 `demand-archive-index.md` 增量
- S6 归档时追加 DM-20260704-003

## PR 拆分

**建议合并为单 PR**（single change = single PR per `git-workflow.md`）：
- branch: `feat/devrix-runtime-feedback-closure`
- PR title: `devrix-runtime-feedback-closure: i18n ZH 硬规则 + tracing parent-span 连续性 + tool-level timeout`
- body: 引用 `demand.md` + `proposal.md` + `design.md` + `tasks.md`

## S4-Gate 验证

```bash
go vet ./...
go test -race -count=1 ./internal/layers/contextengine/...
go test -race -count=1 ./internal/layers/observability/...
go test -race -count=1 ./internal/layers/orchestration/...
go test -race -count=1 ./internal/bootstrap/...
```

## S5 验收

```bash
CI_COVERAGE=1 go test -race -timeout 300s ./internal/... 
# 期望: 全量 PASS, 0 race, 覆盖率 ≥ 72.2% (baseline)
```

8 个新 P0 T 点全部 IMPLEMENTED（除 T03 P1）。

## 风险与回滚

| 风险 | 缓解 |
|------|------|
| 中文硬规则在 zh-TW 误触发 | 接受（暂用简化字 ZH 段） |
| tracing ctx 透传破坏 0 命中"feature" | 0 命中是 bug 而非 feature |
| 60s 误伤长时间 build | env 可调（DEVRIX_TOOL_TIMEOUT_SECONDS） |
| orphan marker 增加 span 体积 | 仅 fallback 触发，正常路径 0 开销 |

回滚：3 个核心修改 < 30 LOC，trivial git revert。
