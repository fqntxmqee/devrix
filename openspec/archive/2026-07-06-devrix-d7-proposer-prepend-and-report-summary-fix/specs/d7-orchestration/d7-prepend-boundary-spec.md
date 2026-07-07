# Spec Delta: D7 UserContextPrepend Boundary Contract (DM-20260706-008)

**Change**: `devrix-d7-proposer-prepend-and-report-summary-fix`
**Status**: S7_Archived (retroactive after 3-PR ship-then-archive cycle)
**Domain**: D7 Orchestration
**Spec section**: §7.x D2→D3 Boundary Contract (NEW)
**Version scope**: d7-orchestration v4.27.0 → v4.28.0

## 1. Boundary Function Definition

`messagesForLLMInvoke(msgs []types.Message, prepend map[string]string) []types.Message`
是 D2→D3 边界的**唯一允许路径**。所有 D7 proposer / executor / turn-adapter 的
LLM 调用必须经过此函数包装。

### 1.1 Input Contract

| Param | Type | Required | Description |
|-------|------|----------|-------------|
| `msgs` | `[]types.Message` | yes | 业务 messages (system + user + tool) |
| `prepend` | `map[string]string` | no (default nil) | AGENTS.md / D{N}→path 映射等 D2-prepared context |

### 1.2 Output Contract

| Return | Type | Description |
|--------|------|-------------|
| wrapped msgs | `[]types.Message` | prepend 块以 `<system-reminder>` 包裹插入第一条 user message 前 |

### 1.3 Nil / Zero-Value Behavior

`prepend == nil` 或 len 0 → `messagesForLLMInvoke` 跳过 prepend 包装,返回原 `msgs`。
**纯 passthrough**,legacy caller 完全不受影响(forward-compatible 零值)。

## 2. Call Site Enforcement

### 2.1 Allowed Call Patterns

```go
// ✅ CORRECT: boundary wrap
msgs := messagesForLLMInvoke([]types.Message{...}, prepared.UserContextPrepend)
ch, err := invoker.InvokeStream(ctx, orchtypes.LLMInvokeRequest{Messages: msgs, ...})

// ✅ CORRECT: nil prepend (legacy / no-op)
msgs := messagesForLLMInvoke([]types.Message{...}, nil)
ch, err := invoker.InvokeStream(ctx, orchtypes.LLMInvokeRequest{Messages: msgs, ...})
```

### 2.2 Forbidden Call Patterns

```go
// ❌ FORBIDDEN: bypass boundary
ch, err := invoker.InvokeStream(ctx, orchtypes.LLMInvokeRequest{Messages: msgs, ...})
// → LLM 收不到 AGENTS.md prepend,D2 context 丢失
```

### 2.3 Allow-list

| File | Reason |
|------|--------|
| `semantic_verifier_default.go` | Verify 节点 template-mimicry 检测,设计上不需要 AGENTS.md 注入 |

允许-list 由 CI guard 静态维护。**任何新增 allow-list 必须有 PR review + 注释**。

## 3. CI Guard Specification

`scripts/check-d7-d3-prepend-boundary.sh` 实现 §2 enforcement:

| Check | Description | Failure mode |
|-------|-------------|--------------|
| 1 | 扫描 `internal/layers/orchestration/sessionorchestrator/` 内所有 `LLMInvoker.InvokeStream` / `InvokeNonStream` 调用点 | — |
| 2 | 每个 call site 所在函数**前 30 行**必须已调过 `messagesForLLMInvoke` | `FAIL: <file>:<line> - no messagesForLLMInvoke in preceding 30 lines` |
| 3 | `messagesForLLMInvoke` 至少被 2 个文件引用(防止误删共享 boundary 函数) | `FAIL: messagesForLLMInvoke only referenced in <N> files` |
| 4 | Allow-list 文件跳过 check 2/3 | — |

**Exit codes**:
- 0 = PASS
- 1 = FAIL(任一 check 不通过)

**CI 接入**:本地 + GitHub Actions heavy steps(planned follow-up)。

## 4. UncertaintyReport Serialization Contract

`uncertaintyReportSummary(report UncertaintyReport) string` 序列化规约:

### 4.1 Format (newline-free single line)

```
intent=<kind>; anomalies=<count>[; q=<question>][; dev=<statement>][; obs<n>=<statement>]...
```

### 4.2 Field Rules

| Field | Required | Source | Threshold | Notes |
|-------|----------|--------|-----------|-------|
| `intent=<kind>` | yes | `report.QuantizedIntent.Kind` | — | 永远保留作为第一段(向后兼容) |
| `anomalies=<count>` | yes | `len(report.Anomalies)` | — | Numeric anomaly count |
| `q=<question>` | conditional | `report.BusinessObservations[ObsUncertainty].Question` | strength ≥ 0.7 | 第一个匹配的 ObsUncertainty 的 Question |
| `dev=<statement>` | conditional | `report.Anomalies[ObsDeviation].Statement` | strength ≥ 0.7 | 第一个匹配的 ObsDeviation 的 Statement |

### 4.3 Partition Discipline

| Partition | Includes | Why |
|-----------|----------|-----|
| `BusinessObservations` | CatBusiness ObsUncertainty + CatBusiness ObsFact | LLM-driven 用户级问题 |
| `Anomalies` | CatSystem ObsUncertainty + CatSystem ObsFact + ObsDeviation + 高强度 ObsUncertainty | 机械信号 + 系统级 |

**禁止**:把 `BusinessObservations` 折入 `Anomalies`(设计意图),也禁止
`Anomalies = len(report.Observations)`(语义坍塌)。

## 5. Trace Evidence (D7 Observability)

| Span | Attribute | Expected |
|------|-----------|----------|
| `llm.invoke` | `messages_count` | ≥ 2 (AGENTS.md + user) |
| `llm.invoke` | `prepend_size_bytes` | > 0 when prepend present |
| `plan.observation_summary` | `value` | 含 `intent=<kind>; ...; q=<question>; dev=<statement>` |
| `execute.plan_frame_delta` | `injection_status` | "ok" (Execute trace verified self-healing) |

## 6. Migration Path

### 6.1 Existing Proposer Audit (2026-07-06)

| # | Caller | File:Line | Status |
|---|--------|-----------|--------|
| 1 | WorkitemExecutor | `workitem_executor.go:485` | ✅ Wired (pre-PR-449) |
| 2 | TurnInvoke | `turn_invoke.go:240` | ✅ Wired (pre-PR-449) |
| 3 | LLMObservationProposer | `llm_observation_proposer.go:55` | ✅ Wired (PR #449) |
| 4 | LLMStrategicPlanProposer | `strategic_plan_proposer.go:403` | ✅ Wired (PR #449) |
| 5 | LLMIntentSegmenter | `intent_segmenter.go:293` | ✅ Wired (PR #460 跨域 debt) |
| 6 | SemanticVerifier (allow-list) | `semantic_verifier_default.go:157` | ✅ Allow-list |

### 6.2 New Proposer Pattern (Future)

任何新加 LLM proposer / executor call site 必须:
1. 在所在文件 import `messagesForLLMInvoke`(通常已通过 D7 sessionorchestrator import)
2. msgs 走 `messagesForLLMInvoke(msgs, prepared.UserContextPrepend)` 包装
3. CI guard 跑 PASS 才能 merge
4. 若必须 allow-list,需 PR review + 注释 + 增量 §2.3 表

## 7. Failure Modes & Recovery

| Failure | Behaviour | Recovery |
|---------|-----------|----------|
| `prepend == nil` | passthrough msgs unchanged | legacy caller 工作如常 |
| `messagesForLLMInvoke` 函数被删 | CI guard FAIL exit 1 | revert 删除 commit |
| 新加 call site 忘记 wrap | CI guard FAIL 报告 `file:line` | 编辑所在函数前 30 行加 wrap |
| Verify 节点误"修复"被 wrap | CI guard allow-list 兜底 | 重新 allow-list + 注释 |
| Cross-change sibling PR 未读本 contract | CI guard 立刻 FAIL 触发 hotfix | PR #460 模式:forward-compatible field + wrap |

## 8. References

- 触发事件 trace: `0369df062c125dfe2aad9de21363730e` (sess_1783333760211_6000)
- 关联 PR: #449 + #450 + #460
- 关联 Sibling Change: `devrix-d7-observational-fastpath` (DM-20260706-011, v4.28.0)
- 跨域 contract: D2 `prepared.UserContextPrepend` (D2 prepared context 输出)
- 跨域 contract: D3 `messagesForLLMInvoke` (D3 LLM gateway wrapper)
- CI guard: `scripts/check-d7-d3-prepend-boundary.sh`