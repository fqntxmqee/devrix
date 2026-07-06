# Proposal: D7 MUPS Frame Delta Phase 1+2 spans 端到端触发

**Change ID:** `devrix-d7-frame-delta-phase1-2-span-trigger`
**Demand ID:** DM-20260706-001
**Created:** 2026-07-06
**Status:** S2_Design
**Demand:** [`demand.md`](demand.md)
**OpenSpec YAML:** [`.openspec.yaml`](.openspec.yaml)
**Parent Change:** `devrix-d7-mups-frame-delta-closure` (DM-20260705-010, S7_Archived 2026-07-05)

---

## 1. Background

`devrix-d7-mups-frame-delta-closure` (DM-20260705-010) Phase 1-3 code + Phase 4 e2e trace replay 全部闭环,9 PR (#431-#439) 全部 squash merge,8 AC AC1-AC8 全 PASS via `TestIntegration_D7FrameDelta_E2E_SpansAndMonotonicity` + `TestIntegration_D7FrameDelta_ConvergenceMonotonic`。

**但 Phase 4 e2e 测试中存在一个 spec 实施 gap:**

`TestIntegration_D7FrameDelta_E2E_SpansAndMonotonicity` 测试场景下,3 个新 span op (Phase 1 `d7.s9.execute.plan_frame_delta.inject` + Phase 2 `d7.s5.observe.prior_delta.span` + Phase 3 `d7.s9.execute.convergence_metric.emit`) 中只有 Phase 3 在 memory exporter 中实际触发。Phase 1 + Phase 2 的 span 因 synthetic LLM stub 不触发 StrategicPlanProposal 非零 + prevExecCtx prior-round gate 而计 0。

父 Change 的 acceptance-report §8.3 已显式记录此 gap 为 ⚠️ FOLLOW-UP,父 mups-frame-delta-spec.md §3 标注 "production code 已合规,e2e scenario 未触发 gates"。本 Change 即关闭该 gap。

**Why (gap 的影响):**

1. **Spec/code 一致性的最后一道闸门:** spec 声称 FrameDelta 协议横跨 Observe→Plan→Execute 三节点,但 e2e 测试只覆盖了 Execute→Observe 回写一半。如果 Phase 1+2 注入链路在 e2e 场景下不触发,意味着 production code 的触发条件在真实运行链路中也没有被验证,只能依赖 unit 测试覆盖。
2. **后续 FrameDelta 演进(v1.1 TraceID 反向追溯 + v2.0 跨域 FrameDelta 抽象上提)的前置条件:** 任何依赖 FrameDelta I/O 协议的 e2e 验收都必须先通过这个 gap closure。
3. **测试金字塔失衡:** Phase 3 (deterministic 纯函数) 有 e2e 覆盖,Phase 1+2 (append-only 注入链路) 只有 unit 覆盖,e2e tier 缺一块。

**How to apply:** 任何依赖 FrameDelta I/O 协议的 production 代码改动,必须先通过此 gap closure 的 e2e 验证才能视为合规。

## 2. Problem Statement

### 2.1 根因(已确诊)

**Phase 1 `InjectPlanFrameDelta` 触发条件 = `plan.FrameDelta` 非零:**

```go
// internal/layers/orchestration/sessionorchestrator/execute_plan_frame_inject.go
func InjectPlanFrameDelta(ctx, plan.FrameDelta, baseline string) string {
    if plan.FrameDelta.IsZero() {
        return baseline  // 早返回,无 span emit
    }
    // ...emit d7.s9.execute.plan_frame_delta.inject span...
}
```

`SequenceLLMStub` 输出的 `StrategicPlanProposal` JSON 含 `execution_mode + child_specs + deliverable_contract`,但经过 Plan LLM 解码 + Frame Observe/Plan 字段填充后,在 Plan OutputProcessor 中 `plan.FrameDelta.IsZero()` 仍为 true (因为 production Frame Delta 5 字段 `PriorArtifactSummary / KnownGaps / ExecutionMode / ChildSpecs / DeliverableContract` 与 LLM 输出 schema 命名不同,需要显式映射)。

**Phase 2 `BuildObservePriorDelta` 触发条件 = `prevExecCtx != nil` (非首轮):**

```go
// internal/layers/orchestration/sessionorchestrator/observe_frame_delta.go
func BuildObservePriorDelta(prevExecCtx *WorkItemExecContext) FrameDelta {
    if prevExecCtx == nil || prevExecCtx.ConvergenceMetric == nil {
        return FrameDelta{}  // 首轮零值,无 span emit
    }
    // ...emit d7.s5.observe.prior_delta.span...
}
```

Round 1 (Observe 首轮) `prevExecCtx == nil` 返回零值,符合预期。Round 2-5 因 Phase 1 未触发注入,Phase 3 ConvergenceMetric 没有正常累积 prior-round 数据,导致 Phase 2 `BuildObservePriorDelta` 链路上的 FrameDelta 注入条件退化,即使 `prevExecCtx != nil` 也不发射 span。

### 2.2 e2e 覆盖 gap

| Span Op | 期望 (5 turns) | 实际 (Phase 4 PR #437 测试) | 状态 |
|---------|--------------|----------------------------|------|
| `d7.s9.execute.convergence_metric.emit` | ≥ 5 (Phase 3) | ≥ 5 | ✅ e2e PASS |
| `d7.s9.execute.plan_frame_delta.inject` | ≥ 5 (Phase 1) | **0** | ❌ e2e GAP (unit PASS) |
| `d7.s5.observe.prior_delta.span` | ≥ 4 (Phase 2 Round 2-5) | **0** | ❌ e2e GAP (unit PASS) |

### 2.3 unit 测试已覆盖

| T-ID | 描述 | 状态 |
|------|------|------|
| D7-S9-A112-T01..T05 | `InjectPlanFrameDelta` 5 子测试 (摘要 + schema hash + zero-value + budget) | 5/5 PASS |
| D7-S5-A111-T01..T06 | `BuildObservePriorDelta` 6 子测试 (首轮/非首轮 + known_gaps + JSON + append-only) | 6/6 PASS |
| D7-S9-A113-T01..T05 | `ComputeConvergenceMetric` 5 子测试 (zero + 工具 diff + claim + span + 0 LLM) | 5/5 PASS |

### 2.4 待解决问题列表

| ID | 问题 | 优先级 |
|----|------|--------|
| Q1 | 如何让 e2e 测试触发 `plan.FrameDelta` 非零条件? | P0 |
| Q2 | 如何让 e2e 测试 Round 1 `prevExecCtx` 非零(seed prior-round data)? | P0 |
| Q3 | SequenceLLMStub 真实路径 vs mock 注入的 trade-off | P0 |
| Q4 | Phase 1+2 真实触发后,Phase 3 convergence 计算是否会破 last ≤ first*3 单调性? | P1 |

## 3. Proposed Solution（候选方案）

### 方案 A — 最小修复:SequenceLLMStub 增加 `InjectFrameDelta` 钩子

在 `SequenceLLMStub` 增加可选字段 `FrameDeltaInject func(idx int) FrameDelta`,在每次 `Stream` 调用时,如果 callback 非 nil,返回 JSON 前注入 `plan.FrameDelta` 非零字段。

```go
type SequenceLLMStub struct {
    Responses        [][]llmgateway.Chunk
    FrameDeltaInject func(idx int) orchestration.FrameDelta  // NEW
    CallCount        atomic.Int64
}
```

- **优点:** testutil 1 文件 +30 行,0 production code 修改,0 LLM 调用增加
- **缺点:** stub vs production 行为差异需要文档化(stub 通过 callback 注入 FrameDelta,真实 LLM 通过 output processor 解析)
- **范围:** ~50 行 + 2 测试

### 方案 B — 打开 `ProductOwnerPlan` 真实路径

修改 e2e 测试场景,显式调用 `ProductOwnerPlan.Plan(workItem)` 触发真实路径,在 Plan 完成后用 `plan.FrameDelta` 喂给 `InjectPlanFrameDelta`。

- **优点:** 真实路径触发,无 stub vs production 差异
- **缺点:** `ProductOwnerPlan` 需要 LLM 真实调用,违反 0 LLM 承诺 + 需要 mock 整个 Plan 链
- **范围:** ~200 行 + 4 测试

### 方案 C — 方案 A + Round 1 prevExecCtx seed (推荐)

方案 A 基础上,在 e2e 测试场景中显式 seed Round 1 `prevExecCtx` 通过 `WorkItemExecContext` factory 函数,模拟"已运行 1 轮"的状态,这样 Round 1 Observe → Round 2 Plan/Execute 的链路中 Phase 2 触发条件 `prevExecCtx != nil` 满足。

```go
// tests/testutil/d7_stack.go (NEW function)
func SeedPriorExecContext(stack *D7TestStack, workItemID string, priorRound ConvergenceMetric) {
    // 显式注入 prevExecCtx 到 WorkItemExecContext
}
```

- **优点:** A 方案的钩子注入 + C 方案的 prevExecCtx seed,2 链路全覆盖,testutil 仅 +50 行,0 production code 修改
- **缺点:** stub vs production 行为差异需要文档化 + seed 状态需要 setup/teardown 清理避免污染其他 sub-test
- **范围:** ~80 行 + 3 测试

**推荐方案 C** — testutil_only scope,production code 0 修改,2 链路全覆盖,Phase 1+2 e2e span 触发链路完整闭环。

## 4. 候选方案对比矩阵

| 维度 | 方案 A | 方案 B | 方案 C (推荐) |
|------|--------|--------|----------------|
| 触发 Phase 1 span | ✅ (callback 注入) | ✅ (真实路径) | ✅ (callback 注入) |
| 触发 Phase 2 span (Round 2-5) | ✅ (Round 2-5 prevExecCtx 累积) | ✅ (真实 prior 累积) | ✅ (Round 2-5 prevExecCtx + Round 1 seed) |
| 触发 Phase 2 span (Round 1) | ❌ (Round 1 prevExecCtx=nil) | ❌ (Round 1 prevExecCtx=nil) | ✅ (显式 seed) |
| production code 修改 | 0 | 0 (但需 mock ProductOwnerPlan) | 0 |
| testutil 修改 (行) | ~50 | ~200 | ~80 |
| 测试增量 (sub-test) | 2 | 4 | 3 |
| LLM 调用增加 | 0 | 0 (但需 mock 真 LLM) | 0 |
| 与父 change 兼容性 (append-only + 0 LLM) | ✅ | ⚠️ (需真 LLM mock 链路) | ✅ |
| stub vs production 差异文档化负担 | 中 | 无 (真实路径) | 中 |

## 5. Implementation Plan

### 5.1 S3 design 阶段(下一步)

`design.md` 六段式按 `docs/methodology/detail-design-framework.md`:

1. **① 架构目标** — 关闭 Phase 4 §8.3 follow-up gap,testutil_only scope,production code 0 修改
2. **② 架构原则** — append-only + 0 LLM + stub vs production 差异文档化原则
3. **③ 业务流程** — SequenceLLMStub callback 注入 + WorkItemExecContext seed 时序图
4. **④ 领域模型** — D7 testutil 限界上下文(无新聚合根,只新增 helper 函数)
5. **⑤ 核心链路图** — 端到端 e2e 测试路径:setup → seed prior → stub inject → 5 turns → memory exporter inspect
6. **⑥ 接口/API 设计** — `SequenceLLMStub.FrameDeltaInject` 字段 + `SeedPriorExecContext` 函数,纯函数 + With* 不可变

### 5.2 S3-Gate 三方 review

- codex (workspace-codex): production code 路径 0 修改 + testutil 钩子语义
- cursor (workspace-cursor): stub vs production 行为差异 + e2e 覆盖充分性
- claude (本案主导): design.md §5 stub vs running system 差异分析

### 5.3 S4 实现 (3 阶段)

| Phase | 内容 | 范围 | 估计 PR |
|-------|------|------|--------|
| Phase 1 | `tests/testutil/d7_llm_stub.go` + `tests/testutil/d7_stack.go` 增 `FrameDeltaInject` + `SeedPriorExecContext` | ~80 行 | PR #441 |
| Phase 2 | `tests/integration/d7/d7_frame_delta_e2e_test.go` 增 3 sub-test (Phase1And2SpanTrigger + SeedPriorEffect + MonotonicWithSeed) | ~150 行 | PR #442 |
| Phase 3 | `openspec/specs/d7-orchestration/{t-registry,CHANGELOG,mups-frame-delta-spec.md}` 域文档同步 + L5-MUPS-FD-6 登记 | ~30 行 | PR #443 |

### 5.4 S5 验收 (5 标准)

1. AC1: Phase 1 span ≥ 5 via `TestIntegration_D7FrameDelta_Phase1And2SpanTrigger`
2. AC2: Phase 2 span ≥ 4 (Round 2-5 + Round 1 seed)
3. AC3: Phase 3 span ≥ 5 (无回归)
4. AC4: design.md §5 stub vs running system 差异分析
5. AC5: 71 frame 测试 + 16 D7-FD unit 测试 0 行为变化

### 5.5 S6 归档

- PR #441 + #442 + #443 全部 squash merge
- archive `openspec/changes/devrix-d7-frame-delta-phase1-2-span-trigger/` → `openspec/archive/2026-07-06-devrix-d7-frame-delta-phase1-2-span-trigger/`
- `demand-archive-index.md` 新增 DM-20260706-001 行
- `.openspec.yaml` status → `s7_archived`
- verify-archive.sh 12 PASS / 0 FAIL / 1 WARN (heuristic 误判允许)

## 6. Risks & Mitigations

| 风险 | 影响 | 缓解 |
|------|------|------|
| SequenceLLMStub callback 注入引入 stub vs production 行为差异 | High | design.md §5 文档化差异;在 stub 注释中明确 "testutil only, NOT production" |
| `SeedPriorExecContext` 影响 WorkItemExecContext 单例状态污染其他 sub-test | Medium | setup/teardown 显式 reset: `obsConfig.MemoryExporter.Reset()` + `prevExecCtx` atomic.Pointer.Store(nil) |
| Phase 1+2 真实触发后,Phase 3 convergence 因 prior 数据累积而变化,可能破 last ≤ first*3 单调性 | Medium | 独立 sub-test `TestIntegration_D7FrameDelta_MonotonicWithSeed` 验证;不动原 `TestIntegration_D7FrameDelta_ConvergenceMonotonic` |
| testutil `d7_stack.go` 新增 `SeedPriorExecContext` 函数影响其他 D7 测试 | Low | 函数显式 only-for-seed 测试调用,其他 D7 测试不调即保持原行为 |
| Phase 1+2 真实触发链路与 running system 行为不一致(stub ≠ running system) | Medium | design.md §5 stub vs running system 差异分析;running system 真实测试通过独立飞书 session 验证(out of scope) |
| S3-Gate 三方 review 因 cc-connect binding 缺失无法启动 | Medium | fallback: 用户在群聊 `/bind workspace-codex` + `/bind workspace-cursor`,或在 docs PR 中显式记录三方 review record(claude 主导) |
| `FrameDeltaInject` callback 增加 testutil 字段,未来 LLM output schema 演化时需同步更新 | Low | callback 签名 `(idx int) FrameDelta`,FrameDelta 是 5 字段纯值对象,production FrameDelta 字段变化时 callback 自动适配 |

## 7. Out of Scope

明确**不在本 Change 范围**的事项:

1. **production code 修改** — `internal/layers/orchestration/sessionorchestrator/{execute_plan_frame_inject,observe_frame_delta,convergence_metric}.go` 0 修改,hardening emitter.go 0 修改
2. **M1 ObservationFrame / M2 StrategicPlanFrame 契约** — 0 修改,append-only 注入原则不变
3. **新 LLM 调用** — 0 LLM 承诺不变,testutil callback 不调真 LLM
4. **PlanKind / VerdictKind 决策表** — 不破坏 DM-20260705-008 Strategy 抽象
5. **Pessimistic Commit L3** — 不破坏三层 fail-safe
6. **FrameDelta v1.1 TraceID 反向追溯** — 单独 OpenSpec change
7. **FrameDelta v2.0 跨域抽象上提** — 单独 OpenSpec change
8. **真实飞书 session Jaeger trace 重放** — running system 验证,需 user action
9. **T19 S3-Gate 三方 review 独立 track** — 父 change acceptance-report §8.5 已标 follow-up,本 change 不合并

## 8. 关联

### 父 Change

- `devrix-d7-mups-frame-delta-closure` (DM-20260705-010, S7_Archived 2026-07-05, PR #431-#439 全 MERGED)
  - acceptance-report §8.3 文档化此 gap 为 ⚠️ FOLLOW-UP
  - mups-frame-delta-spec.md §3 标注 "production code 已合规,e2e scenario 未触发 gates"

### 兄弟 Change

- `devrix-d7-mups-frame-delta-closure` (DM-20260705-010) — frame-delta 协议本身
- `devrix-d7-mups-v5-escape-engine-v5-6-review-fixes` (DM-20260625-004) — 类似 follow-up PR 联动模式参考

### 关联 Demand

- DM-20260705-010 (frame-delta-closure parent)
- DM-20260629-007/008 (TaskContract 统一 — 提供 WorkItemExecContext atomic.Pointer 接口)
- DM-20260625-019 (MUPS 5-node Coverage — 提供 ObsConfig testutil 基础)
- DM-20260617-008 (D7 TestFramework — 提供 SequenceLLMStub + D7StackOptions 基础设施)

### 关联 Test (前置)

- `tests/integration/d7/d7_frame_delta_e2e_test.go` `TestIntegration_D7FrameDelta_E2E_SpansAndMonotonicity` (Phase 4 §8.3 gap 来源)
- `tests/testutil/d7_stack.go` ObsConfig 字段 (DM-20260705-010 PR #437 引入)
- `tests/testutil/d7_llm_stub.go` SequenceLLMStub (DM-20260617-008 引入)

### 关联 PR (前置)

- #437 (Phase 4 e2e trace replay — ObsConfig 字段基础)
- #438 (Phase 4 sync + span attr key alignment)
- #439 (demand-archive-index ACCEPTED sync)
- #440 (S1 demand.md entry — 本 change S1 已落地)