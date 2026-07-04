# Proposal: Runtime 反馈链路闭环

**Change ID:** `devrix-runtime-feedback-closure`  
**Demand ID:** DM-20260704-003  
**Created:** 2026-07-04  
**Status:** S2_Proposal (Draft)  
**Demand:** [`demand.md`](demand.md)

---

## 1. Problem Statement

DM-20260704-002（`mups-d2-context-tools-ownership`）刚把 D2 contextengine 的 MUPS 上下文出口统一到 `MaterializeForMUPS`，但运行时仍暴露三个反馈回路断点（详见 [`demand.md`](demand.md) §2）：

1. **i18n 断点** — `prompt_sections_zh.go` 缺中文硬规则，LLM 在中文 system prompt 下会因 tool 错误 / 英文 `<system-reminder>` 切到英文。
2. **Tracing 父链断点** — `tracingStepObserver.OnStep` 把 `startSpan` 返回的新 ctx 用 `_` 丢弃，compression pipeline 上 200+ orphan spans。
3. **Tool 超时断点** — `turn_adapter.executeOne` 无 tool-level timeout，tool 卡死 → Turn 卡死 → feishu 卡片 never complete。

三个问题的共同主题：**上游信号（locale / ctx / tool budget）到下游消费者（LLM / observability / orchestrator）之间的 transmission gap**。

## 2. Proposed Solution

3 个独立小修复，**不引入新架构**，纯 runtime hardening：

### 2.1 i18n 中文硬规则

在 `internal/layers/contextengine/i18n/prompt_sections_zh.go` 的 `intro` 段（**仅 ZH locale**）追加：

```text
请始终用中文回复用户，除非用户主动切到其他语言或明确要求。
```

并在 `tone_and_style` 段加：

```text
- 可见输出（除代码标识符、文件路径、技术名词外）必须用中文
- 错误消息、警告、日志说明用中文
```

EN locale `prompt_sections_en.go` **不加**反向指令（英文用户不被中文污染已隐式保证）。

### 2.2 Tracing ctx 透传

修改 `internal/layers/contextengine/prepare/compression/tracing_step_observer.go::OnStep`：

```go
// 之前
_, span := o.startSpan(ctx, ...)
if span != nil { span.End() }
if o.inner != nil { o.inner.OnStep(ctx, step, before, after) }

// 之后
ctx, span := o.startSpan(ctx, ...)
if span != nil { span.End() }
if o.inner != nil { o.inner.OnStep(ctx, step, before, after) }
```

并在 `internal/layers/observability/instrument/tracer/tracer.go` 的 `Start()` fallback 路径（ctx 缺 sc 时）加 orphan 标记：

```go
if sc, ok := SpanContextFromContext(ctx); !ok || !sc.IsValid() {
    slog.Warn("orphan span", "operation", operation)
    // 创建 fallback sc，但 span attribute 加 span.orphan=true
}
```

### 2.3 Tool-level Timeout

`internal/bootstrap/turn_adapter.go::executeOne` 加 `context.WithTimeout` 包裹：

```go
func executeOne(ctx context.Context, call ToolCall, cfg ExecuteConfig) (ToolResult, error) {
    timeout := cfg.ToolTimeoutSeconds
    if timeout <= 0 { timeout = 60 }
    
    callCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
    defer cancel()
    
    result := executeWithContext(callCtx, call)
    if errors.Is(callCtx.Err(), context.DeadlineExceeded) {
        // 走 fail-closed: emit ChannelRoute warn span + 返回 timeout error
        emitChannelRoute(ctx, "tool.timeout.exceeded", attrs(call, timeout))
        return ToolResult{Err: ErrToolTimeout}, nil
    }
    return result, nil
}
```

配置走 `internal/shared/config/user.go::ToolTimeoutSeconds` + env `DEVRIX_TOOL_TIMEOUT_SECONDS`，默认 60s。

## 3. Capabilities

| Capability ID | 描述 | 域 | 优先级 |
|---------------|------|-----|--------|
| **D2-S15-A82** | i18n locale-gated 中文硬规则（zh-CN/zh-Hans/zh） | D2 | P0 |
| **D5-S2-A01** | Tracing ctx 透传 + orphan span 标记 | D5 | P0 |
| **D7-S2-A50** | Tool-level timeout + fail-closed 兜底 | D7 | P0 |

## 4. Scope

### In Scope
- 3 个文件源码修改（`prompt_sections_zh.go` / `tracing_step_observer.go` / `turn_adapter.go`）
- 1 个 tracer fallback 路径增强（`tracer.go`）
- 1 个 config 字段新增（`ToolTimeoutSeconds`）
- 8 个新 P0 T 点（详见 [`tasks.md`](tasks.md)）
- i18n golden test (zh/en prompt bytes 稳定)
- tracing parent-span continuity test (3 case: trace fork / scheduler dispatch / child_downlink)
- tool timeout test (正常路径 + 超时路径)
- spec 增量（3 个 spec 文档）
- 域文档 CHANGELOG 一行（vX.Y.Z 增量）

### Out of Scope
1. 完整 tool #54 卡住 root cause 复现（沙箱限制，需 runtime）
2. per-tool timeout override（v1.1 follow-up）
3. D2↔D3 import lint 增强（DM-020 单独 change）
4. D7 verify-promotion PLANNED 收口（D7-S4-A50 T01-T03，独立 follow-up）
5. D7 S15 PARTIAL rollup E2E / trace replay stub（独立 follow-up）
6. LLM 输出 token-level i18n 检测（探索性）

## 5. PR 拆分

| PR | scope | files | risk | branch |
|----|-------|-------|------|--------|
| PR-1 | i18n 中文硬规则 + golden test | 4 files (~80 LOC) | Low | `feat/devrix-runtime-feedback-closure` |
| PR-2 | Tracing ctx 透传 + orphan marker | 5 files (~150 LOC) | Low | 同上（可拆 PR 但同 branch） |
| PR-3 | Tool-level timeout + fail-closed | 4 files (~120 LOC) | Low | 同上 |

**建议合并为单 PR（single change = single PR per `git-workflow.md`）**，因为三处修复同主题（runtime feedback closure），便于 S4-Gate 统一 review。

## 6. Verification

### S4-Gate
- `go vet ./...` 0 issue
- `go test -race -count=1 ./internal/layers/contextengine/...` PASS
- `go test -race -count=1 ./internal/layers/observability/...` PASS
- `go test -race -count=1 ./internal/layers/orchestration/...` PASS
- `go test -race -count=1 ./internal/bootstrap/...` PASS
- i18n golden test PASS
- tracing parent-span continuity test PASS
- tool timeout test PASS

### S5-Gate
- 全量 `go test -race -count=1 ./internal/...` PASS，0 race detector warnings
- 22/22 orchestration + 22/22 contextengine + 7 d5 packages `go test -race` PASS
- 8/8 新 P0 T 点 IMPLEMENTED
- 覆盖率（基线 72.2%）：持平或上升
- 0 现有 174 个归档的 acceptance criteria 破坏

### S6-Gate
- PR squash merged to `master`
- `./scripts/verify-archive.sh devrix-runtime-feedback-closure`

## 7. Risk

| 风险 | 影响 | 缓解 |
|------|------|------|
| 中文硬规则在英文 locale 误启用 | 英文用户被中文污染 | 严格 locale gating；golden test 覆盖 EN 不含中文 |
| tracing ctx 透传引入 race | 子 span 数据竞争 | `go test -race` 全量；tracingStepObserver 保持 value receiver |
| 60s timeout 太短打断正常 tool | 用户体验差 | env 可调；v1.1 per-tool override |
| orphan marker 增加 span 体积 | OTLP 出口负载 | 仅 fallback 路径标记 |
| 沙箱无法复现 tool #54 | 修复未验证 root cause | runtime 验证交用户；fail-closed 防御性兜底 |

## 8. Related

- DM-20260704-002 (`mups-d2-context-tools-ownership`) — D2 上下文出口统一
- DM-20260630-013 (`d2-d7-review-hardening`) — D2/D7 hardening 收口
- DM-20260703-001 (`d7-convergence-contract`) — CC-1~CC-5 收敛契约
- DM-20260610-005 (`devrix-observability-baggage`) — D5 W3C Baggage 业务上下文传播
