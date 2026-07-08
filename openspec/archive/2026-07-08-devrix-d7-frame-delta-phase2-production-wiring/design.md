# Design: D7 Phase 2 frame_delta production wiring

**Change ID:** `devrix-d7-frame-delta-phase2-production-wiring`
**Demand ID:** DM-20260706-004
**Status:** S7_Archived (2026-07-08)
**Parent Proposal:** `proposal.md`
**Parent Demand:** `demand.md`
**Template:** `docs/methodology/detail-design-framework.md`（六段式）
**Created:** 2026-07-08
**Parent Change:** `devrix-d7-mups-frame-delta-closure` (DM-20260705-010, S7_Archived 2026-07-05)

---

## ① 架构目标

### 1.1 业务目标

关闭 sibling `devrix-d7-frame-delta-phase1-2-span-trigger` (DM-20260706-001) S3-Gate codex review (2026-07-08 BLOCKED) 拆分的 Phase 2 production wiring gap:production 调用链路上 `prevExecCtx` 在 `mergeProposedObservations` 处中断 (硬编码 `nil`),导致 `BuildObservePriorDelta` 在 production 永远走首轮零值分支,`d7.s5.observe.prior_delta.span` Round 2-5 永远 0。

| 痛点（来自 codex P0-2 BLOCKED） | 本 Change 对应 AC |
|-------------------------------|-------------------|
| **`observation_proposer.go:257` 硬编码 nil**:`buildObserveSignalInput(sessionID, item, tm, nil)`,production `prevExecCtx` 中断 | AC1 + AC2 |
| **production span count 永远 0**:Round 2-5 真实 prior 已累积但 span 不触发,与 e2e baseline 2 spans 不对称 | AC3 |
| **函数签名 caller audit 缺失**:`itemObserve` 函数签名变化需 audit 现有 caller (ItemPipelineRunner.Run() 等) | AC1 |
| **sibling testutil 兼容性**:`SeedPriorExecContext` helper 在 production wiring 后仍需 work | AC5 |

### 1.2 技术目标（量化指标）

| 指标 | 目标值 | 触发 AC |
|------|-------|---------|
| **production code 增量** | ≤ 50 行 (函数签名变化 + caller audit + unit test) | scope |
| **testutil 增量** | 0 行 | scope |
| **新增 LLM 调用** | 0 | scope |
| **e2e baseline Phase 2 span** | ≥ 2 (Round 2-5 真实累积) | AC3 |
| **`BuildObservePriorDelta` 单测** | 6/6 PASS,0 行为变化 | AC4 |
| **`itemObserve` caller 审计** | 100% caller 适配,无 nil deref | AC1 |
| **sibling DM-20260706-001 testutil 兼容** | sibling S5 验收时联动验证 | AC5 |
| **L5-MUPS-FD-7 T-IDs 登记** | `openspec/specs/d7-orchestration/t-registry.md` D7-FD 段 | AC7 |
| **Jaeger 真实链路验证** | user action,follow_up_gaps 标注 | AC6 |

### 1.3 约束条件

- **production code 必要修改** — `internal/layers/orchestration/sessionorchestrator/{item_observe,observation_proposer}.go` 函数签名 +1 参数 (`prevExecCtx *WorkItemExecContext`)
- **函数签名 nil-safe** — 新增参数 `prevExecCtx *WorkItemExecContext` 必须 nil-safe,ItemPipelineRunner.Run() 在 Round 1 仍传 nil (符合 `BuildObservePriorDelta` 已有 nil 处理)
- **append-only 注入原则不变** — production FrameDelta 5 字段 0 修改
- **0 LLM 承诺不变** — production wiring 不调真 LLM,只 pass 参数
- **M1/M2 契约 0 修改** — sibling DM-20260706-001 testutil_only scope 不动
- **DM-20260706-002/003 已 wired** — Phase 1 production wiring + Phase 1+2 emit end() 已闭环,本 change 仅补 Phase 2 wiring
- **DM-20260705-008 Strategy 抽象** — 不冲突 (production wiring 不进决策表)
- **三层 fail-safe / Pessimistic Commit L3** — 不破坏 (nil-safe 设计)

---

## ② 架构原则

### 2.1 设计原则（5 条）

| # | 原则 | 落地方式 | 触发 AC |
|---|------|---------|---------|
| P1 | **函数签名变化最小化** | 仅 +1 参数 `prevExecCtx *WorkItemExecContext`,其余参数 0 变化 | AC1 |
| P2 | **nil-safe 设计** | `prevExecCtx == nil` 兼容 (BuildObservePriorDelta 已有 nil 处理 → 零值返回) | AC1 + AC4 |
| P3 | **caller audit 100%** | `git grep` 列出所有 caller 逐个 audit,ItemPipelineRunner.Run() 在 Round 1 传 nil | AC1 |
| P4 | **production 与 e2e 对称** | production wiring 后 Phase 2 span count 与 e2e baseline 对齐 (≥ 2) | AC3 |
| P5 | **sibling 兼容性** | sibling DM-20260706-001 testutil `SeedPriorExecContext` 在 production wiring 后仍 work (testutil mock state 与 production wiring 独立) | AC5 |

### 2.2 命名规范

| 元素 | 规范 | 示例 |
|------|------|------|
| **函数签名新增参数** | `prevExecCtx *WorkItemExecContext` (与 `BuildObservePriorDelta` 已有签名一致) | `func itemObserve(..., prevExecCtx *WorkItemExecContext, ...)` |
| **变量名** | `prevExecCtx` (统一,避免混淆 LastRound / ConvergenceMetric / Item.LastRound.ArtifactSummary 等多个候选字段) | `prevExecCtx` |
| **单元测试名** | `Test<func>_<scenario>` | `TestMergeProposedObservations_NonNilPrevExecCtx` |

### 2.3 代码风格

- 函数 < 50 行 (`itemObserve` 现状 ~80 行,+1 参数后仍 < 100 行)
- 文件 < 800 行 (`observation_proposer.go` 当前 ~280 行,+1 参数后仍 < 350 行)
- 异常不过模块边界 (新增参数 nil-safe,下游 `BuildObservePriorDelta` 已有 nil 处理)
- 不可变性:`WorkItemExecContext` 是 atomic.Pointer 持有,函数签名仅传引用
- 0 panic / 0 业务 log

---

## ③ 业务流程

### 3.1 核心用例 — Phase 2 production span 真实触发链路

```
ItemPipelineRunner.Run()                  ← Round 1..N
  │
  ├─ Round 1
  │   ├─ observeRound(item, prevExecCtx=nil)  ← ItemPipelineRunner WorkItemExecContext atomic.Pointer nil
  │   │   └─ itemObserve(item, prevExecCtx=nil, ...)
  │   │       └─ mergeProposedObservations(prevExecCtx=nil)
  │   │           └─ buildObserveSignalInput(item, tm, prevExecCtx=nil)
  │   │               └─ BuildObservePriorDelta(prevExecCtx=nil) → FrameDelta{} 零值 → 无 span emit ✓ (设计预期)
  │   └─ executeRound(item)
  │       └─ InjectPlanFrameDelta(ec.PlanFrameDelta)  ← Phase 1 已 wired (#443)
  │           └─ emit span D7_Execute_PlanFrameDelta_Inject ✓
  │
  ├─ Round 2 (WorkItemExecContext atomic.Pointer 已累积 Round 1 prior)
  │   ├─ observeRound(item, prevExecCtx=&WorkItemExecContext{...})
  │   │   └─ itemObserve(item, prevExecCtx=&WorkItemExecContext{...}, ...)
  │   │       └─ mergeProposedObservations(prevExecCtx=&...)
  │   │           └─ buildObserveSignalInput(item, tm, prevExecCtx=&...)
  │   │               └─ BuildObservePriorDelta(prevExecCtx=&...) → FrameDelta{Item.LastRound.ArtifactSummary, ...}
  │   │                   └─ emit span D7_Observe_PriorDelta_Inject ✓ [AC3]
  │   └─ executeRound(item)
  │       └─ InjectPlanFrameDelta(ec.PlanFrameDelta) → emit span D7_Execute_PlanFrameDelta_Inject ✓
  │
  ├─ Round 3..N (同上 Round 2)
  │
  └─ Round N ConvergenceMetric emit
      └─ ComputeConvergenceMetric → emit span D7_Execute_ConvergenceMetric_Emit ✓
```

### 3.2 异常补偿 — 3 类 Fallback 路径

| 异常场景 | 触发条件 | Fallback 路径 | 影响 |
|---------|---------|--------------|------|
| `prevExecCtx == nil` (Round 1) | ItemPipelineRunner.Run() Round 1 atomic.Pointer 初始 nil | `BuildObservePriorDelta(nil)` 返回 `FrameDelta{}` 零值,e2e Phase 2 Round 1 span count = 0 | 设计预期;Round 2-5 仍触发 span,AC3 阈值 ≥ 2 反映现实 |
| `prevExecCtx != nil` 但 `ConvergenceMetric == nil` | ItemPipelineRunner.Run() Round N (N>1) 但 ConvergenceMetric 未累积 | `BuildObservePriorDelta` line 47-49 零值返回,e2e Phase 2 Round N span count = 0 | 多见于 Round 2 边界 (ConvergenceMetric Round 1 末轮才累积);Round 3+ 触发 |
| 函数签名 caller 缺失 (caller audit 漏) | ItemPipelineRunner.Run() 某 caller 忘记传 prevExecCtx | caller audit 全量审计;unit test 覆盖每个 caller | P0 issue:生产事故 |

### 3.3 分支处理决策树

```
production Phase 2 span emit
├─ Round 1
│   └─ prevExecCtx == nil → FrameDelta{} → 无 span emit → 设计预期 ✓
├─ Round 2..N
│   ├─ prevExecCtx != nil && ConvergenceMetric != nil → FrameDelta{...} → emit span ✓
│   │   └─ AC3 PASS (e2e baseline ≥ 2)
│   └─ prevExecCtx != nil && ConvergenceMetric == nil → FrameDelta{} → 无 span emit
│       └─ AC3 Round N span count = 0 (边界,正常)
└─ 所有 caller 适配
    └─ ItemPipelineRunner.Run() 在 Round 1..N 均传 prevExecCtx → AC1 PASS
```

---

## ④ 领域模型

### 4.1 聚合根（本 Change 无新增）

本 Change **无新聚合根**,仅修改 production 函数签名 + 参数传递。父 change (DM-20260705-010) 已定义的核心聚合根保持不变:

- `FrameDelta` (5 字段纯值对象) — DM-20260705-010 §4.1
- `ConvergenceMetric` (3 字段纯值对象) — DM-20260705-010 §4.1
- `WorkItemExecContext` (atomic.Pointer,prior round state 承载) — DM-20260705-010 §4.1 + DM-20260629-007/008

### 4.2 限界上下文 — D7 Orchestration Session Orchestrator

```
internal/layers/orchestration/sessionorchestrator/
├── item_observe.go                         # itemObserve 函数签名 +1 (NEW prevExecCtx parameter)
├── observation_proposer.go                 # mergeProposedObservations 函数签名 +1 (NEW prevExecCtx parameter)
├── observation_proposer_test.go            # unit test +1 (Round 2 prevExecCtx 非空场景)
├── item_pipeline.go (ItemPipelineRunner.Run())
│   └─ caller audit:observeRound(item, prevExecCtx) 调用 itemObserve(item, prevExecCtx) ← AC1 必填
├── observe_frame_delta.go                  # 0 修改 (BuildObservePriorDelta 已有 nil 处理)
├── workitem_exec_context.go                # 0 修改 (atomic.Pointer 已合规)
└── ...
```

**修改清单 (本 change):**
- `internal/layers/orchestration/sessionorchestrator/{item_observe,observation_proposer}.go` 函数签名变化
- `internal/layers/orchestration/sessionorchestrator/item_pipeline.go` ItemPipelineRunner.Run() caller audit + 1 处传参
- `internal/layers/orchestration/sessionorchestrator/observation_proposer_test.go` unit test +1

**不修改清单:**
- `internal/layers/orchestration/sessionorchestrator/observe_frame_delta.go` (BuildObservePriorDelta 已有 nil 处理)
- `internal/layers/orchestration/sessionorchestrator/{execute_plan_frame_inject,convergence_metric}.go` 0 修改
- `internal/layers/orchestration/hardening/emitter.go` 0 修改 (Phase 1+2 emit 已 wired via DM-20260706-003)

### 4.3 领域事件 — Span emit 不变

本 Change 不修改 span emit 代码,仅通过 caller audit + 参数传递让 span 实际触发。沿用父 change 已注册的 3 span op:

| Span Op | 触发函数 | 当前状态 |
|---------|---------|---------|
| `d7.s9.execute.plan_frame_delta.inject` | `InjectPlanFrameDelta` (production) | ✅ 已 wired via DM-20260706-002 PR #443 |
| `d7.s5.observe.prior_delta.span` | `BuildObservePriorDelta` (production) | ⚠️ emit end() wired via DM-20260706-003 PR #444,**但 prevExecCtx caller 缺失,本 change 修复** |
| `d7.s9.execute.convergence_metric.emit` | `ComputeConvergenceMetric` (production) | ✅ wired |

### 4.4 跨域消费模型 — D7 orchestration 内部 + D5 observability

| 跨域消费点 | 现状 | 本 Change 影响 |
|-----------|------|---------------|
| **D7 orchestration** (`internal/layers/orchestration/`) | `ItemPipelineRunner.Run()` 持有 `WorkItemExecContext` atomic.Pointer | caller audit 1 处 (observeRound 调用 itemObserve 传 prevExecCtx) |
| **D5 observability** (`internal/layers/observability/`) | `Jaeger` exporter + memory exporter 已 wired | 0 修改,本 change 仅通过参数传递让 Phase 2 span 触发 |

---

## ⑤ 核心链路图

### 5.1 production 调用链 (本 change 后)

```
[ItemPipelineRunner.Run()]
    │
    ├─ Round 1
    │   ├─ observeRound(item, prevExecCtx=nil)         ← Round 1 atomic.Pointer 初始 nil
    │   │   └─ itemObserve(item, prevExecCtx=nil, ...)
    │   │       └─ mergeProposedObservations(..., prevExecCtx=nil)
    │   │           └─ buildObserveSignalInput(item, tm, prevExecCtx=nil)
    │   │               └─ BuildObservePriorDelta(prevExecCtx=nil)
    │   │                   └─ return FrameDelta{} ← 零值,无 span emit (设计预期)
    │   └─ executeRound(item)
    │       └─ InjectPlanFrameDelta(ec.PlanFrameDelta) → emit span ✓
    │
    ├─ Round 2
    │   ├─ observeRound(item, prevExecCtx=&WorkItemExecContext{ConvergenceMetric: &conv, Item.LastRound.ArtifactSummary: "..."})
    │   │   └─ itemObserve(item, prevExecCtx=&..., ...)
    │   │       └─ mergeProposedObservations(..., prevExecCtx=&...)
    │   │           └─ buildObserveSignalInput(item, tm, prevExecCtx=&...)
    │   │               └─ BuildObservePriorDelta(prevExecCtx=&...)
    │   │                   └─ return FrameDelta{Item.LastRound.ArtifactSummary: "...", KnownGaps: [...], ...}
    │   │                       └─ emit span D7_Observe_PriorDelta_Inject ✓ [AC3 PASS]
    │   └─ executeRound(item)
    │       └─ InjectPlanFrameDelta(ec.PlanFrameDelta) → emit span ✓
    │
    ├─ Round 3..N (同上 Round 2)
    │
    └─ ConvergenceMetric emit (Round N)
        └─ ComputeConvergenceMetric → emit span D7_Execute_ConvergenceMetric_Emit ✓
```

### 5.2 production vs e2e 对称性分析

| 维度 | e2e test (memoryExporterObsConfig) | production (Jaeger, 本 change 前) | production (Jaeger, 本 change 后) |
|------|-----------------------------------|----------------------------------|----------------------------------|
| Phase 1 `D7_Execute_PlanFrameDelta_Inject` | 2 spans | ✅ wired via #443 | ✅ wired (无变化) |
| Phase 2 `D7_Observe_PriorDelta_Inject` | 2 spans | ⚠️ 0 spans (prevExecCtx=nil) | ✅ 2+ spans (Round 2+ 真实累积) |
| Phase 3 `D7_Execute_ConvergenceMetric_Emit` | 2 spans | ✅ wired | ✅ wired (无变化) |

**对称性结论:** 本 change 后 production 与 e2e baseline 对齐 (Phase 2 span count ≥ 2),FrameDelta 协议 production-side 全链路可观测。

### 5.3 caller audit 范围

```
git grep "itemObserve\|mergeProposedObservations" internal/layers/orchestration/sessionorchestrator/
  → item_observe.go:91 (line 91)              ← MODIFIED (传 prevExecCtx)
  → observation_proposer_test.go:129 (test)   ← 0 修改 (test scenario 仍可 nil)
  → observation_proposer_test.go:153 (test)   ← 0 修改 (test scenario 仍可 nil)
```

预计 caller audit 影响范围:仅 `item_observe.go:91` 1 处需传 `prevExecCtx` (新增参数);`ItemPipelineRunner.Run()` 上游已持有 `WorkItemExecContext` 引用,只需 1 处 caller 传参。

### 5.4 单点风险与缓解

| 单点风险 | 影响 | 缓解 | 触发 AC |
|---------|------|------|---------|
| `itemObserve` 函数签名变化破坏 caller audit 漏掉的 caller | 中 | git grep 全量审计 + caller 适配 unit test | AC1 |
| `prevExecCtx` 在 ItemPipelineRunner.Run() 多轮累积失效 | 中 | `WorkItemExecContext` atomic.Pointer 已在 DM-20260629-007/008 合规;新增 unit test 验证多轮累积 | AC4 |
| Round 1 `prevExecCtx=nil` 触发零值分支导致 Phase 2 Round 1 span count = 0 | 低 | 设计预期 (Round 1 无 prior);e2e AC3 阈值 ≥ 2 反映现实 | AC3 |
| Jaeger 真实链路验证需 user action | 低 | out of scope, follow_up_gaps 标注 | AC6 |
| sibling DM-20260706-001 testutil `SeedPriorExecContext` 与 production wiring 冲突 | 低 | AC5 显式验证;sibling S5 验收时联动 | AC5 |

---

## ⑥ 接口 / API 设计

### 6.1 风格 — 函数签名 +1 参数 + nil-safe 设计

```go
// internal/layers/orchestration/sessionorchestrator/item_observe.go (MODIFIED)
//
// itemObserve (DM-20260706-004, AC1) accepts prevExecCtx as a parameter so that
// BuildObservePriorDelta can fire d7.s5.observe.prior_delta.span in production
// Round 2+. nil-safe: Round 1 prevExecCtx=nil → BuildObservePriorDelta returns
// zero-value FrameDelta (no span emit), which is the design contract.
func itemObserve(
    ctx context.Context,
    sessionID string,
    item *workmodel.WorkItem,
    prevExecCtx *WorkItemExecContext,  // NEW (DM-20260706-004)
    classifier IntentClassifier,
    learner learn.Learner,
    trackMode string,
    tasks *workmodel.TaskManager,
    proposer ObservationProposer,
) (orchtypes.UncertaintyReport, []string, string, error) {
    // ...existing body unchanged...
    // line 91 (MODIFIED)
    proposed, observeReject, _ := mergeProposedObservations(
        ctx, proposer, sessionID, item, tasks, prior, prevExecCtx,  // NEW arg
    )
    // ...
}

// internal/layers/orchestration/sessionorchestrator/observation_proposer.go (MODIFIED)
//
// mergeProposedObservations (DM-20260706-004, AC2) accepts prevExecCtx and
// forwards to buildObserveSignalInput, replacing the hardcoded nil at line 257.
func mergeProposedObservations(
    ctx context.Context,
    proposer ObservationProposer,
    sessionID string,
    item *workmodel.WorkItem,
    tm *workmodel.TaskManager,
    prior *learn.AdaptivePrior,
    prevExecCtx *WorkItemExecContext,  // NEW (DM-20260706-004)
) ([]orchtypes.Observation, string, error) {
    if proposer == nil || item == nil {
        return nil, "", nil
    }
    in := buildObserveSignalInput(sessionID, item, tm, prevExecCtx)  // 替换 nil
    // ...rest unchanged...
}
```

### 6.2 契约 (Span + 函数签名 + 错误码)

| 元素 | 契约 | 验证 |
|------|------|------|
| **`itemObserve` 函数签名** | `func itemObserve(..., prevExecCtx *WorkItemExecContext, ...)` | static type check |
| **`mergeProposedObservations` 函数签名** | `func mergeProposedObservations(..., prevExecCtx *WorkItemExecContext)` | static type check |
| **`buildObserveSignalInput` 调用** | 第 4 参数 = `prevExecCtx` (非 nil) | unit test |
| **Span Op 触发** | 沿用父 change 已注册的 `d7.s5.observe.prior_delta.span` | memory exporter inspection |
| **nil-safe 行为** | `prevExecCtx == nil` → `BuildObservePriorDelta` 返回零值,无 panic | unit test |
| **错误码** | production silent 行为,0 panic;错误由 item_pipeline.go 上游报 (已合规) | item_pipeline unit test |

### 6.3 幂等保障表

| 操作 | 幂等保障 | 触发条件 |
|------|---------|---------|
| `itemObserve(prevExecCtx)` | 多次调用同一 Round + 同一 prevExecCtx 返回相同 result | Round 内幂等 |
| `mergeProposedObservations(prevExecCtx)` | 多次调用同一 prevExecCtx 返回相同 observation set | Round 内幂等 |
| `WorkItemExecContext atomic.Pointer` | Round N set 后,Round N+1 read 看到一致 state | Round 间一致 |

### 6.4 版本演进路径

```
v1.0 (本 Change, DM-20260706-004)
  - itemObserve + mergeProposedObservations 函数签名 +1 参数 (production wiring)
  - caller audit 100% 适配
  - e2e baseline Phase 2 span count 提升到 ≥ 2
  - 单元测试覆盖 Round 2 prevExecCtx 非空场景

v1.1 (后续 Phase 5 follow-up)
  - 真实飞书 session running system Jaeger trace 重放验证
  - out of scope for v1.0 (running system 验证需 user action)
```

---

## 附录

### 附录 A：File Manifest

| 操作 | 路径 | 描述 | LOC |
|------|------|------|-----|
| MODIFIED | `internal/layers/orchestration/sessionorchestrator/item_observe.go` | itemObserve 函数签名 +1 (prevExecCtx) + line 91 caller 适配 | +3 行 |
| MODIFIED | `internal/layers/orchestration/sessionorchestrator/observation_proposer.go` | mergeProposedObservations 函数签名 +1 + line 257 替换 nil | +3 行 |
| MODIFIED | `internal/layers/orchestration/sessionorchestrator/observation_proposer_test.go` | unit test +1 (Round 2 prevExecCtx 非空场景) | +30 行 |
| MODIFIED | `internal/layers/orchestration/sessionorchestrator/item_pipeline.go` | ItemPipelineRunner.Run() caller audit 1 处 (observeRound 调用 itemObserve 传 prevExecCtx) | +1 行 |
| NEW | `openspec/specs/d7-orchestration/t-registry.md` (D7-FD 段) | L5-MUPS-FD-7 T-IDs 登记 | +10 行 |
| NEW | `openspec/specs/d7-orchestration/CHANGELOG.md` (顶部) | IMPLEMENTED 条目 | +5 行 |
| NEW | `openspec/specs/d7-orchestration/mups-frame-delta-spec.md` §3.5 | "Phase 2 production wiring 触发条件" 新章节 | +15 行 |

**总计:** ~67 行 (production code 7 + unit test 30 + 域文档 30)

### 附录 B：Rollback Plan

| 层级 | 回滚机制 | 触发条件 |
|------|---------|---------|
| **PR revert** | `git revert <commit>` (squash merge) | 任何 unit test 或 e2e 失败 |
| **函数签名回滚** | 移除 `prevExecCtx` 参数,`buildObserveSignalInput` 重新硬编码 nil → 旧行为 (Phase 2 production span count = 0) | production wiring 引发的 caller nil deref |
| **域文档回滚** | `git revert` CHANGELOG.md + t-registry.md + mups-frame-delta-spec.md | 文档同步问题 |

### 附录 C：回归风险评估

| 风险 | 影响 | 测试策略 |
|------|------|---------|
| `itemObserve` 函数签名变化影响其他 caller | 中 | git grep 全量审计 + unit test 覆盖每个 caller |
| `mergeProposedObservations` 函数签名变化影响其他 caller | 低 | 当前仅 `item_observe.go:91` 1 处 caller |
| production wiring 引入 nil deref | 高 | nil-safe 设计 + unit test 覆盖 Round 1 nil + Round 2+ non-nil 场景 |
| Round 2+ prior 累积未生效 | 中 | unit test 验证多轮累积 + e2e baseline 提升到 ≥ 2 |
| testutil 修改导致 layer-lint 报警 | 低 | 本 change 0 testutil 修改,sibling DM-20260706-001 已有 layer-lint PASS 基线 |
| span attr key 与父 change spec 不一致 | 低 | 本 change 不修改 span emit 代码,沿用父 change 已对齐的 attr |

### 附录 D：S3 检查清单自检

**S3 Checklist:**
- [x] ① 架构目标 — 业务目标 + 技术目标 + 约束条件 三段齐全
- [x] ② 架构原则 — 5 条原则 + 命名规范 + 代码风格 三段齐全
- [x] ③ 业务流程 — 核心用例时序图 + 3 类 Fallback + 决策树 三段齐全
- [x] ④ 领域模型 — 无新聚合根 + 限界上下文 + 领域事件 (沿用) + 跨域消费 0 影响 (除 caller audit)
- [x] ⑤ 核心链路图 — production 调用链 + production vs e2e 对称性分析 + caller audit 范围 + 5 单点风险
- [x] ⑥ 接口/API 设计 — 风格 + 契约 + 幂等 + 版本演进 四段齐全
- [x] 附录 A — File Manifest (7 文件 +67 行)
- [x] 附录 B — Rollback Plan (3 层)
- [x] 附录 C — 回归风险评估 (6 风险)
- [x] 附录 D — S3 Checklist 自检

**S3-Gate Review 结论:** 待启动 — codex CLI 已就位,cursor quota 待恢复,claude 主导 design.md §5 production vs e2e 对称性分析。

**S3-Gate 三方 review 重点:**
- codex:production code 函数签名变化 caller audit 完整性 + atomic.Pointer 多轮累积一致性
- cursor:sibling DM-20260706-001 testutil 兼容性 + production 与 e2e span count 对齐
- claude 主导:design.md §5.2 production vs e2e 对称性分析 + Round 2-5 prior 累积链路

### 附录 E：下一步

- S3-Gate 三方 review (待启动)
- S4 implementation:itemObserve + mergeProposedObservations 函数签名变化 + caller audit + unit test
- S5 acceptance:`D7_Observe_PriorDelta_Inject` e2e baseline ≥ 2 spans + sibling DM-20260706-001 testutil 兼容 (联动)
- S6 archive:`openspec/archive/2026-07-08-devrix-d7-frame-delta-phase2-production-wiring/` + `demand-archive-index.md` 登记 + verify-archive.sh 12/12 PASS