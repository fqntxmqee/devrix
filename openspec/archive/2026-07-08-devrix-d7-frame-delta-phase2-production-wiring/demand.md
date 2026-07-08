---
demand-id: DM-20260706-004
title: D7 Phase 2 frame_delta production wiring — observation_proposer.go:257 nil → prevExecCtx 上游传参
priority: P1
status: S7_Archived
dsaft_domain: orchestration
created: 2026-07-08
parent_change: devrix-d7-mups-frame-delta-closure
parent_demand: DM-20260705-010
sibling_change: devrix-d7-frame-delta-phase1-2-span-trigger
sibling_demand: DM-20260706-001
origin: |
  从 devrix-d7-frame-delta-phase1-2-span-trigger (DM-20260706-001) S3-Gate
  codex review (BLOCKED 2026-07-08) 拆分出的独立 Phase 2 production
  wiring gap。
---

# D7 Phase 2 frame_delta production wiring

## 1. 背景

父 change `devrix-d7-mups-frame-delta-closure` (DM-20260705-010) 已 S7_Archived,Phase 1+2+3 全部 wired production。但 **Phase 2 production wiring 缺失**:

`internal/layers/orchestration/sessionorchestrator/observation_proposer.go:257` 在调用 `buildObserveSignalInput(sessionID, item, tm, nil)` 时**硬编码 `nil`** 作为 `prevExecCtx` 参数:

```go
func mergeProposedObservations(
    ctx context.Context,
    proposer ObservationProposer,
    sessionID string,
    item *workmodel.WorkItem,
    tm *workmodel.TaskManager,
    prior *learn.AdaptivePrior,
) ([]orchtypes.Observation, string, error) {
    if proposer == nil || item == nil {
        return nil, "", nil
    }
    in := buildObserveSignalInput(sessionID, item, tm, nil)  // ← P0: 硬编码 nil
    // ...
}
```

下游 `buildObserveSignalInput` 调用 `BuildObservePriorDelta(ctx, sessionID, prevExecCtx)`:

```go
// observe_frame_delta.go:45
func BuildObservePriorDelta(ctx context.Context, sessionID string, prevExecCtx *WorkItemExecContext) interfaces.FrameDelta {
    if prevExecCtx == nil || prevExecCtx.ConvergenceMetric == nil {
        return FrameDelta{}  // ← 首轮零值,无 span emit
    }
    // ...
}
```

**影响:** 生产代码中 `d7.s5.observe.prior_delta.span` 永远 `span_tag_complete=false`,即使 Round 2-5 真实 prior 已累积,也不会触发 Phase 2 span emit。

## 2. 根因(已确诊)

production 调用链路上 `prevExecCtx` 在 2 处中断:

1. **ItemPipelineRunner.Run() → itemObserve()** — `prevExecCtx` 通过 `WorkItemExecContext` atomic.Pointer 累积,但当前未向下传
2. **itemObserve() → mergeProposedObservations() → buildObserveSignalInput()** — `prevExecCtx` 缺失,`buildObserveSignalInput` 接 nil

生产代码所需 wiring:

```go
// item_observe.go (MODIFIED, signature change)
func itemObserve(
    ctx context.Context,
    sessionID string,
    item *workmodel.WorkItem,
    prevExecCtx *WorkItemExecContext,  // ← NEW parameter
    learner learn.Learner,
    // ...existing params...
)

// item_observe.go (MODIFIED, line 91)
proposed, observeReject, _ := mergeProposedObservations(
    ctx, proposer, sessionID, item, tasks, prior,
    prevExecCtx,  // ← NEW arg
)

// observation_proposer.go (MODIFIED, signature change)
func mergeProposedObservations(
    ctx context.Context,
    proposer ObservationProposer,
    sessionID string,
    item *workmodel.WorkItem,
    tm *workmodel.TaskManager,
    prior *learn.AdaptivePrior,
    prevExecCtx *WorkItemExecContext,  // ← NEW parameter
) (...) {
    if proposer == nil || item == nil {
        return nil, "", nil
    }
    in := buildObserveSignalInput(sessionID, item, tm, prevExecCtx)  // ← 替换 nil
    // ...
}
```

## 3. 验收标准

| ID | 标准 | 优先级 | 验证方式 |
|----|------|--------|----------|
| AC1 | production code wiring:`itemObserve` 接 `prevExecCtx` 参数并向下传 | P0 | unit test |
| AC2 | `mergeProposedObservations` 接 `prevExecCtx` 并传给 `buildObserveSignalInput` | P0 | unit test |
| AC3 | e2e baseline:`D7_Observe_PriorDelta_Inject` span count ≥ 2 (Round 2+) | P0 | memory exporter inspection |
| AC4 | `BuildObservePriorDelta` 单元测试 (6 sub-test) 0 行为变化 | P0 | `go test -race ./internal/layers/orchestration/sessionorchestrator/` |
| AC5 | sibling DM-20260706-001 testutil seed 设计可对接 production wiring (testutil_only 不破坏) | P1 | sibling e2e PASS |
| AC6 | production trace 重放验证 (Jaeger): Round 2+ 真实触发 `d7.s5.observe.prior_delta.span` | P1 | out of scope, user action |
| AC7 | L5-MUPS-FD-7 T-IDs 在 `openspec/specs/d7-orchestration/t-registry.md` 登记 | P1 | t-registry diff |

## 4. 依赖与约束

| 类型 | 内容 |
|------|------|
| 依赖 | 父 change DM-20260705-010 (frame-delta-closure S7_Archived 2026-07-05) |
| 依赖 | DM-20260706-002 PR #443 (`workitem_executor.go` Phase 1 wiring) |
| 依赖 | DM-20260706-003 PR #444 (Phase 1+2 emit sites end() wiring) |
| 依赖 | `WorkItemExecContext` atomic.Pointer 已存在 (DM-20260629-007/008 TaskContract 统一) |
| 依赖 | ItemPipelineRunner.Run() 已持有 `WorkItemExecContext` 引用 |
| 约束 | append-only 注入原则不变 |
| 约束 | 0 LLM 调用承诺不变 |
| 约束 | M1/M2 契约 0 修改 |
| 约束 | sibling DM-20260706-001 testutil_only scope 0 修改 (本 change 仅 production wiring) |

## 5. 变更范围

### 修改 / 不变更

| 操作 | 路径 | 描述 |
|------|------|------|
| MODIFIED | `internal/layers/orchestration/sessionorchestrator/item_observe.go` | `itemObserve` 函数签名 +1 (`prevExecCtx *WorkItemExecContext`);line 91 `mergeProposedObservations` 调用 +1 参数 |
| MODIFIED | `internal/layers/orchestration/sessionorchestrator/observation_proposer.go` | `mergeProposedObservations` 函数签名 +1;line 257 `buildObserveSignalInput` 接 prevExecCtx 替换 nil |
| MODIFIED | `internal/layers/orchestration/sessionorchestrator/` 调用 `itemObserve` 的上游 | ItemPipelineRunner.Run() 持有 `WorkItemExecContext` 引用并向下传 (预计 1-2 处 caller) |
| MODIFIED | `internal/layers/orchestration/sessionorchestrator/observation_proposer_test.go` | 单元测试 +1 (Round 2 prevExecCtx 非空场景) |
| NEW | `openspec/specs/d7-orchestration/t-registry.md` (D7-FD 段) | L5-MUPS-FD-7 T-IDs 登记 |
| NEW | `openspec/specs/d7-orchestration/CHANGELOG.md` (顶部) | IMPLEMENTED 条目 |

### 不变更

- `internal/layers/orchestration/sessionorchestrator/{execute_plan_frame_inject,observe_frame_delta}.go` 0 修改 (Phase 1+2 单元测试已合规)
- `internal/layers/orchestration/hardening/emitter.go` 0 修改 (Phase 2 emit 路径已 wired via DM-20260706-003)
- M1/M2 契约 0 修改
- testutil 0 修改 (与 sibling DM-20260706-001 互补,不重叠)
- sibling DM-20260706-001 change 目录 0 修改 (本 change 互补)

## 6. 风险评估

| 风险 | 影响 | 缓解 |
|------|------|------|
| `itemObserve` 函数签名变化破坏现有 caller | High | `git grep` 列出所有 caller,逐个 audit;新增参数使用 atomic.Pointer 兼容 (nil-safe) |
| `prevExecCtx` 在多轮次之间未正确累积(Round 1 → Round 2 → Round 3...) | Medium | `WorkItemExecContext` atomic.Pointer 已在 ItemPipelineRunner 中,只需确保 set 路径一致;新增 unit test 验证多轮累积 |
| e2e baseline:`D7_Observe_PriorDelta_Inject` span count 提升后破坏 sibling DM-20260706-001 testutil seed 假设 | Medium | sibling change AC5 验证 (testutil seed 兼容 production wiring);PR review 时联动验证 |
| Production Round 1 `prevExecCtx == nil` 触发零值分支,Round 1 span count = 0 | Low | 设计预期 (Round 1 无 prior);Round 2-5 触发 span;e2e AC3 阈值 ≥ 2 (而非 ≥ 5) 反映现实 |
| Jaeger 真实链路验证需 user action (飞书 session + trace 重放) | Low | out of scope, follow_up_gaps 显式标注 |

## 7. 关联

### 父 Change

- `devrix-d7-mups-frame-delta-closure` (DM-20260705-010, S7_Archived 2026-07-05) — Phase 1+2+3 协议定义

### Sibling Change

- `devrix-d7-frame-delta-phase1-2-span-trigger` (DM-20260706-001, status S3_Rewriting)
  - testutil_only scope:SequenceLLMStub callback + SeedPriorExecContext helper
  - 互补:本 change 修 production wiring,sibling change 修 testutil coverage
  - S3-Gate codex review 2026-07-08 BLOCKED → 拆分本 change

### 关联 PR (前置)

- #443 (DM-20260706-002): Phase 1 production wiring (workitem_executor.go)
- #444 (DM-20260706-003): Phase 1+2 emit end() wiring
- #325 (DM-20260629-007): TaskContract TaskReport (WorkItemExecContext atomic.Pointer)
- #327 (DM-20260629-008): TaskContract PR-B (WorkItemExecContext L3 Pessimistic)