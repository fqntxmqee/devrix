# Proposal: QueryLoop Span 对齐 v1.1 — runViaQueryLoop 补 iteration / llm_call span

**Change ID:** devrix-queryloop-spans-v1.1
**Demand ID:** DM-20260612-014
**Status:** S7_Archived (2026-06-18; S1_Cancelled; not implemented)
**Author:** Devrix Team
**Date:** 2026-06-12 → Cancelled 2026-06-18
**Parent Change:** devrix-harness-unification

> **取消原因 (2026-06-18):** 创建 6 天未推进；未进入 S2 提案。QueryLoop 链路在 devrix-harness-unification v1.0 已对齐了顶层 span；iteration / llm_call span 的细化属于"锦上添花"，在 devrix-eval / devrix-tracing 等更高优先级变更前非阻塞。归档为 S7_Archived（S1_Cancelled → Archived）。

## 1. Background

`devrix-harness-unification` v1.0（D5-S1/D5-S2 范围）完成了 QueryLoop 主链路的 span 对齐（顶层 span、agent span、tool span）。但 `runViaQueryLoop` 内部仍存在**两层未追踪**：

1. **iteration span**：QueryLoop 内的迭代循环（while iteration < max_iterations）作为独立 span
2. **llm_call span**：每次 LLM 调用（ILLMGateway.Complete）作为独立 span

## 2. Problem Statement

| 问题 | 影响 |
|------|------|
| iteration 未独立 span | Jaeger trace 中看不出"哪一轮迭代"耗时 |
| llm_call 未独立 span | LLM 调用延迟与 ContextEngine 处理延迟混在一起 |
| 与 harness v1.0 不完全对齐 | 违反"全链路可观测"原则 |

## 3. 提案范围（未实施）

### 3.1 iteration span

```go
// internal/layers/contextengine/queryloop.go
for iter := 0; iter < maxIter; iter++ {
    span := tracer.StartSpan(fmt.Sprintf("queryloop.iteration.%d", iter))
    defer span.End()
    // ... iteration body
}
```

### 3.2 llm_call span

```go
// internal/layers/contextengine/queryloop.go
span := tracer.StartSpan("queryloop.llm_call")
resp, err := s.llm.Complete(ctx, req)
span.End()
```

## 4. Non-Goals

- 不修改 harness v1.0 已对齐的顶层 span 结构
- 不引入新的 trace 后端
- 不修改 span attributes schema（沿用 OpenTelemetry 约定）

## 5. 取消决策

**Decision (2026-06-18):**
1. 6 天（2026-06-12 → 2026-06-18）未推进
2. 实际使用中：iteration / llm_call 的延迟可从 LLM 日志推断，痛点不明确
3. 资源优先级 → 让位给 devrix-tracing / devrix-eval 等活跃变更

## 6. 后续路径

- 如 iteration / llm_call span 成为明确痛点 → 可基于本 proposal 重新激活
- 引用：demand-archive-index.md DM-20260612-014 行

## 7. 归档

**Status:** S7_Archived (2026-06-18)
**Verdict:** S1_Cancelled → Archived；不实施；未来按需重开。