# Proposal: D7 Phase 2 frame_delta production wiring

**Change ID:** `devrix-d7-frame-delta-phase2-production-wiring`
**Demand ID:** DM-20260706-004
**Created:** 2026-07-08
**Status:** S1_Proposal → S2_Proposal (待 S3-Gate 三方 review)
**Demand:** [`demand.md`](demand.md)
**OpenSpec YAML:** [`.openspec.yaml`](.openspec.yaml)
**Parent Change:** `devrix-d7-mups-frame-delta-closure` (DM-20260705-010, S7_Archived 2026-07-05)
**Sibling Change:** `devrix-d7-frame-delta-phase1-2-span-trigger` (DM-20260706-001, S3_Rewriting)

---

## 1. Background

父 change DM-20260705-010 已 S7_Archived。Phase 1+2+3 协议定义 + 单测覆盖 + e2e Phase 3 trace 重放全部闭环。但 sibling change DM-20260706-001 的 S3-Gate codex review (2026-07-08) 发现一个 **production-side 独立 gap**:

`internal/layers/orchestration/sessionorchestrator/observation_proposer.go:257` 硬编码 `nil` prevExecCtx,导致 `BuildObservePriorDelta` 在 production 永远走首轮零值分支,`d7.s5.observe.prior_delta.span` 在 Round 2-5 也不触发。

S3-Gate codex P0 issue 拆分决定:

| P0 issue | 归属 |
|----------|------|
| 根因分析 stale — sibling hotfixes (#443/#444) 已 wired Phase 1 emission path | DM-20260706-001 重写方案承认 |
| Phase 2 production caller 硬编码 nil — testutil 修复无法触达 | **本 change DM-20260706-004 处理** |
| `SeedPriorExecContext` 设计字段错位 (`ConvergenceMetric` vs `Item.LastRound.ArtifactSummary`) | DM-20260706-001 重写方案承认 |

**Why:** Phase 2 production span 缺失意味着 FrameDelta 协议在 Observe→Execute 回写链路上 production-side 无法观测。spec/code 一致性的最后一道闸门:spec 声称 FrameDelta 协议横跨 Observe→Plan→Execute 三节点,但 production trace 重放只能看到 Plan→Execute 单向。

**How to apply:** 任何依赖 FrameDelta I/O 协议的 production-side 改动(包括未来 v1.1 TraceID 反向追溯、跨域 FrameDelta 抽象上提)都必须先通过本 change 的 production wiring 验证才能视为合规。

## 2. Problem Statement

### 2.1 根因(已确诊)

production 调用链路上 `prevExecCtx` 在 2 处中断:

```
ItemPipelineRunner.Run()
  └─ observeRound() / itemObserve()  ← prevExecCtx 通过 WorkItemExecContext atomic.Pointer 持有
       └─ mergeProposedObservations()  ← ⚠️ 缺失 prevExecCtx 参数
            └─ buildObserveSignalInput(sessionID, item, tm, nil)  ← ⚠️ 硬编码 nil
                 └─ BuildObservePriorDelta(ctx, sessionID, prevExecCtx=nil)
                      └─ return FrameDelta{}  // 零值,无 span emit
```

### 2.2 production vs e2e 不对称

| 维度 | e2e test (memoryExporterObsConfig) | production (Jaeger) |
|------|-----------------------------------|---------------------|
| Phase 1 `D7_Execute_PlanFrameDelta_Inject` | 2 spans (post-#443 wiring) | wired via DM-20260706-002 PR #443 |
| Phase 2 `D7_Observe_PriorDelta_Inject` | 2 spans (post-#444 emit end() wiring) | ⚠️ 永远 0 (prevExecCtx 硬编码 nil) |
| Phase 3 `D7_Execute_ConvergenceMetric_Emit` | 2 spans | wired |

### 2.3 单元测试已覆盖但 production wiring 缺失

| T-ID | 描述 | 状态 |
|------|------|------|
| D7-S5-A111-T01..T06 | `BuildObservePriorDelta` 6 子测试 (prevExecCtx 参数化) | 6/6 PASS (unit) |
| D7-S9-A112-T01..T05 | `InjectPlanFrameDelta` 5 子测试 | 5/5 PASS (unit) |
| **L5-MUPS-FD-7 (新增)** | **production wiring e2e span 触发链路** | **GAP** |

## 3. Proposed Solution

### 方案 A — 函数签名 +1 参数 + upstream 传参

修改 `itemObserve` 函数签名增加 `prevExecCtx *WorkItemExecContext` 参数,`mergeProposedObservations` 同步增加,`buildObserveSignalInput` 接收非空 prevExecCtx。

```go
// item_observe.go (MODIFIED, signature change)
func itemObserve(
    ctx context.Context,
    sessionID string,
    item *workmodel.WorkItem,
    prevExecCtx *WorkItemExecContext,  // NEW
    learner learn.Learner,
    // ...existing params...
) (...) {
    // line 91 (MODIFIED)
    proposed, observeReject, _ := mergeProposedObservations(
        ctx, proposer, sessionID, item, tasks, prior, prevExecCtx,
    )
}

// observation_proposer.go (MODIFIED)
func mergeProposedObservations(
    ctx context.Context,
    proposer ObservationProposer,
    sessionID string,
    item *workmodel.WorkItem,
    tm *workmodel.TaskManager,
    prior *learn.AdaptivePrior,
    prevExecCtx *WorkItemExecContext,  // NEW
) (...) {
    if proposer == nil || item == nil {
        return nil, "", nil
    }
    in := buildObserveSignalInput(sessionID, item, tm, prevExecCtx)  // 替换 nil
    // ...
}
```

- **优点:** 最小改动,直接打通 production 调用链
- **缺点:** 函数签名变化需 audit 所有 caller
- **范围:** ~30 行 production code + ~50 行 unit test + 域文档

### 方案 B — 在 `buildObserveSignalInput` 内部用 closure/atomic 读 prevExecCtx

在 `buildObserveSignalInput` 内部通过全局 registry 或 atomic.Pointer 读取 prevExecCtx,不修改函数签名。

- **优点:** 函数签名不变
- **缺点:** 引入全局状态,违反 "function purity" 原则;Round 2-5 prior 数据无法准确回放
- **范围:** ~50 行 (global registry + closure capture)

### 方案 C — 方案 A + sibling testutil 兼容验证 (推荐)

方案 A 基础上,在 sibling DM-20260706-001 S5 验收时联动验证:本 change production wiring 后,sibling testutil `SeedPriorExecContext` helper 仍能 work (testutil seed 注入的是 testutil-only mock state,不影响 production wiring)。

- **优点:** A 方案打通 production + sibling testutil 兼容,production 与 e2e 同步推进
- **缺点:** 跨 change 协调 (sibling review 时需本 change PR landed)
- **范围:** A 方案 + sibling 协调 1 PR review 联动

**推荐方案 C** — production wiring 闭环 + sibling 协调 + AC5 显式验证。

## 4. 候选方案对比矩阵

| 维度 | 方案 A | 方案 B | 方案 C (推荐) |
|------|--------|--------|----------------|
| 函数签名变化 | +1 参数 | 0 | +1 参数 |
| 全局状态污染 | 无 | 有 (registry/atomic) | 无 |
| production 调用链打通 | ✅ | ✅ | ✅ |
| Round 2-5 prior 累积 | ✅ | ⚠️ (global state 不准) | ✅ |
| sibling testutil 兼容 | 需验证 | ⚠️ | ✅ (AC5 显式) |
| production code 增量 (行) | ~30 | ~50 | ~30 + 协调 |
| 测试增量 (sub-test) | 2 | 3 | 2 + 1 联动 |
| LLM 调用增加 | 0 | 0 | 0 |
| 与父 change 兼容性 (append-only + 0 LLM) | ✅ | ✅ | ✅ |

## 5. Implementation Plan

### 5.1 S3 design 阶段(下一步)

`design.md` 六段式按 `docs/methodology/detail-design-framework.md`:

1. **① 架构目标** — 关闭 Phase 2 production wiring gap,~30 行 production code 增量
2. **② 架构原则** — 函数签名变化 + nil-safe + atomic.Pointer 兼容 + 不引入全局状态
3. **③ 业务流程** — ItemPipelineRunner.Run() → itemObserve() → mergeProposedObservations() 调用链 wiring
4. **④ 领域模型** — WorkItemExecContext atomic.Pointer 不变,仅增加 parameter passing
5. **⑤ 核心链路图** — production 调用链 + e2e baseline 对比 + Round 2-5 prior 累积
6. **⑥ 接口/API 设计** — `itemObserve` / `mergeProposedObservations` 函数签名变化 + 单元测试覆盖

### 5.2 S3-Gate 三方 review

- codex (workspace-codex): production code 路径修改 + 函数签名变化 caller audit + atomic.Pointer 累积一致性
- cursor (workspace-cursor): sibling DM-20260706-001 testutil 兼容性 + production 与 e2e span count 对齐
- claude (本案主导): design.md §5 production vs e2e 对称性分析

### 5.3 S4 实现 (2 阶段)

| Phase | 内容 | 范围 | 估计 PR |
|-------|------|------|--------|
| Phase 1 | production wiring:`itemObserve` / `mergeProposedObservations` 函数签名变化 + `buildObserveSignalInput` 接 prevExecCtx + ItemPipelineRunner.Run() 上游传参 | ~30 行 | PR #1 |
| Phase 2 | unit test + 域文档同步 (t-registry + CHANGELOG + spec §3.5) | ~50 行 | PR #2 |

### 5.4 S5 验收 (5 标准)

1. AC1: production code wiring (itemObserve + mergeProposedObservations)
2. AC2: buildObserveSignalInput 接非空 prevExecCtx
3. AC3: e2e baseline D7_Observe_PriorDelta_Inject ≥ 2 spans
4. AC4: BuildObservePriorDelta 单元测试 0 行为变化
5. AC5: sibling DM-20260706-001 testutil seed 设计兼容 (sibling S5 验收联动)

### 5.5 S6 归档

- PR #1 + #2 全部 squash merge
- archive `openspec/changes/devrix-d7-frame-delta-phase2-production-wiring/` → `openspec/archive/2026-07-08-devrix-d7-frame-delta-phase2-production-wiring/`
- `demand-archive-index.md` 新增 DM-20260706-004 行
- `.openspec.yaml` status → `s7_archived`
- verify-archive.sh 12 PASS / 0 FAIL / 1 WARN (heuristic 误判允许)

## 6. Risks & Mitigations

| 风险 | 影响 | 缓解 |
|------|------|------|
| `itemObserve` 函数签名变化破坏现有 caller | High | `git grep` 列出所有 caller,逐个 audit;新增参数使用 atomic.Pointer 兼容 (nil-safe) |
| `prevExecCtx` 在多轮次之间未正确累积 (Round 1 → Round 2 → Round 3...) | Medium | `WorkItemExecContext` atomic.Pointer 已在 ItemPipelineRunner 中,只需确保 set 路径一致;新增 unit test 验证多轮累积 |
| e2e baseline:Phase 2 span count 提升后破坏 sibling DM-20260706-001 testutil seed 假设 | Medium | sibling change AC5 验证 (testutil seed 兼容 production wiring);PR review 时联动验证 |
| Production Round 1 `prevExecCtx == nil` 触发零值分支,Round 1 span count = 0 | Low | 设计预期 (Round 1 无 prior);Round 2-5 触发 span;e2e AC3 阈值 ≥ 2 (而非 ≥ 5) 反映现实 |
| Jaeger 真实链路验证需 user action (飞书 session + trace 重放) | Low | out of scope, follow_up_gaps 显式标注 |

## 7. Out of Scope

明确**不在本 Change 范围**的事项:

1. **testutil 修改** — 与 sibling DM-20260706-001 互补,本 change 0 testutil 修改
2. **M1/M2 契约修改** — 0 修改,append-only 注入原则不变
3. **新 LLM 调用** — 0 LLM 承诺不变
4. **Phase 1 production wiring** — 已在 DM-20260706-002 PR #443 闭环
5. **Phase 1+2 emit end() wiring** — 已在 DM-20260706-003 PR #444 闭环
6. **PlanKind / VerdictKind 决策表** — 不破坏 DM-20260705-008 Strategy 抽象
7. **Pessimistic Commit L3** — 不破坏三层 fail-safe
8. **FrameDelta v1.1 TraceID 反向追溯** — 单独 OpenSpec change
9. **FrameDelta v2.0 跨域抽象上提** — 单独 OpenSpec change
10. **真实飞书 session Jaeger trace 重放** — running system 验证,需 user action

## 8. 关联

### 父 Change

- `devrix-d7-mups-frame-delta-closure` (DM-20260705-010, S7_Archived 2026-07-05, PR #431-#439 全 MERGED)
  - Phase 1+2+3 协议定义 + 单测覆盖 + e2e Phase 3 trace 重放全部闭环

### Sibling Change

- `devrix-d7-frame-delta-phase1-2-span-trigger` (DM-20260706-001, status S3_Rewriting)
  - testutil_only scope:SequenceLLMStub callback + SeedPriorExecContext helper
  - 互补:本 change 修 production wiring,sibling change 修 testutil coverage
  - S3-Gate codex review 2026-07-08 BLOCKED → 拆分本 change

### 关联 Demand

- DM-20260705-010 (frame-delta-closure parent)
- DM-20260706-001 (sibling testutil_only change)
- DM-20260706-002 (PR #443 Phase 1 production wiring — sibling hotfix 同日合并)
- DM-20260706-003 (PR #444 Phase 1+2 emit end() wiring — sibling hotfix 同日合并)
- DM-20260629-007/008 (TaskContract 统一 — WorkItemExecContext atomic.Pointer 接口)

### 关联 PR (前置)

- #443 (Phase 1 production wiring — workitem_executor.go +19 binder)
- #444 (Phase 1+2 emit end() wiring)
- #325 (TaskContract TaskReport — WorkItemExecContext atomic.Pointer)
- #327 (TaskContract PR-B — WorkItemExecContext L3 Pessimistic)

### 关联 Test (前置)

- `internal/layers/orchestration/sessionorchestrator/observe_frame_delta_test.go` `TestBuildObservePriorDelta_*` (6 sub-test,parameterized by prevExecCtx)
- `internal/layers/orchestration/sessionorchestrator/observation_proposer_test.go` 单元测试 (Round 1 prevExecCtx=nil 场景)