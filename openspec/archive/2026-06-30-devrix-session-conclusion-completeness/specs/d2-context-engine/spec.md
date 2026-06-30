# Spec Delta — D2-Context-Engine — Session Conclusion Completeness

**Change ID:** `devrix-session-conclusion-completeness`
**Target Spec:** `openspec/specs/d2-context-engine/spec.md`
**Target Version:** see `openspec/specs/d2-context-engine/CHANGELOG.md` (incremented on merge)
**Demand ID:** DM-20260630-011
**Created:** 2026-06-30
**Status:** S4_Implementation

---

## Why this delta

`D2_S16_Context_Materialize` span 在 `prepareContext` (D7 sessionorchestrator/workitem_executor.go:431) 以 start-time `message_count=0, token_est=0` 形式发布，随后被立即 `End()`。该模式有两个问题：

1. **Jaeger 上 message_count / token_est 永远是 0** — 无法在 Dashboards 中看到 WorkItem 实际物料规模。
2. **Materialize 0/0 yield 时无独立 signal** — partition 失败但返回成功的 silent failure 路径被掩盖。

本次 delta 治本：将 `EmitContextMaterialize` emit 时机从"start time"延后到"after Materialize returned"，并新增 `D2_Context_Materialize_EmptyYield` sibling span 让空 yield 条件可独立过滤。

---

## ADDED Requirements

### D2-S16-A73: ContextMaterialize Span Back-fill

The `D2_Context_Materialize` span SHALL be emitted **after** `Materializer.Materialize` returns, with `materialize.message_count` and `materialize.token_est` attributes set to the actual `mat.MessageCount` and `mat.TokenEst` values. Pre-call 0/0 placeholder values are explicitly disallowed (would mislead Jaeger filtering).

#### Scenario: regular materialize emits span with actual counts

- **Given** D7 `prepareContext` is invoked for WorkItem `wi_x` with policy `full`
- **When** `Materializer.Materialize` returns `Result.MessageCount=42, TokenEst=8500`
- **Then** a `D2_Context_Materialize` span SHALL be emitted with `materialize.message_count="42"`, `materialize.token_est="8500"`
- **And** span duration SHALL approximate the `Materialize` call latency (not 0)

### D2-S16-A74: EmptyYield Diagnostic Span

When `Materializer.Materialize` returns `MessageCount==0 && TokenEst==0`, the caller (D7 `prepareContext`) SHALL additionally emit `D2_Context_Materialize_EmptyYield` as a sibling span so dashboards can filter by empty-yield condition independently from regular materialize traces.

#### Scenario: empty yield emits both spans

- **Given** `Materialize` returns 0 messages and 0 token estimate
- **Then** a `D2_Context_Materialize` span SHALL fire with `materialize.message_count="0"`, `materialize.token_est="0"`
- **And** a `D2_Context_Materialize_EmptyYield` span SHALL additionally fire with `materialize.kind="empty_yield"`

#### Scenario: non-empty yield does NOT emit EmptyYield

- **Given** `Materialize` returns 1+ messages or 1+ tokens
- **Then** EmptyYield span SHALL NOT be emitted

---

## Test Point Mapping

| T Point             | Description                                    | File                                                                              |
| ------------------- | ---------------------------------------------- | --------------------------------------------------------------------------------- |
| D2-S16-T02 (AC3)    | Regular materialize emits actual counts       | sessionorchestrator/item_pipeline_materialize_test.go (extend existing coverage) |

---

## Out of Scope

- D2 Materialize Policy (=subagent/brief/full) 逻辑保持不变。
- ContextBudget (DM-20260620-001) 互动保持原状。

---

## Files Modified

- `internal/layers/orchestration/sessionorchestrator/workitem_executor.go` — `prepareContext` 调整 emit 时机 + EmptyYield 触发
- `internal/layers/orchestration/hardening/emitter.go` — 新增 `EmitMaterializeEmptyYield`
- `internal/layers/observability/instrument/telemetry/names.go` — 新增 `OpD2_S16_Context_Materialize_EmptyYield = "D2_Context_Materialize_EmptyYield"`
