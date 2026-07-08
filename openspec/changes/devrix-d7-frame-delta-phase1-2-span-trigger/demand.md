---
demand-id: DM-20260706-001
title: D7 MUPS Frame Delta Phase 1+2 spans 端到端触发 — testutil callback + seed helper 覆盖 e2e gap (scope 收窄)
priority: P1
status: S3_Rewriting
dsaft_domain: orchestration
created: 2026-07-06
parent_change: devrix-d7-mups-frame-delta-closure
parent_demand: DM-20260705-010
sibling_change: devrix-d7-frame-delta-phase2-production-wiring
sibling_demand: DM-20260706-004
---

# D7 MUPS Frame Delta Phase 1+2 spans 端到端触发 (S3-Rewrite)

> **S3-Rewrite 历史 (2026-07-08)**:S3-Gate codex CLI review 判定 **BLOCKED + 3 P0 issue**。
> 本 demand 在 S3-Rewrite 阶段承认 sibling hotfixes (DM-20260706-002 + 003) 已 wired production code,
> Phase 2 production wiring gap 拆分至独立 change DM-20260706-004,scope 收窄为 testutil_only e2e 覆盖提升。

## 1. 背景(S3-Rewrite 更新)

父 change `devrix-d7-mups-frame-delta-closure` (DM-20260705-010) Phase 4 端到端验证通过 PR #437 (`tests/integration/d7/d7_frame_delta_e2e_test.go`) 闭环,2 子测试 (`TestIntegration_D7FrameDelta_E2E_SpansAndMonotonicity` + `TestIntegration_D7FrameDelta_ConvergenceMonotonic`) 全 PASS,8 AC AC1-AC8 全 PASS。

**Phase 4 e2e 测试中存在一个 spec 实施 gap:**

`TestIntegration_D7FrameDelta_E2E_SpansAndMonotonicity` 测试场景下,Phase 1 (`InjectPlanFrameDelta`) + Phase 2 (`BuildObservePriorDelta`) 的 span **未在 e2e 场景触发**。验证时只能看到 Phase 3 (`ComputeConvergenceMetric`) span ≥ 1,前两阶段的 span 在 memory exporter 中记录数为 0。

### 1.1 S3-Rewrite 实测基线 (2026-07-08)

在 master 分支运行 `go test -tags 'integration d7' -v -run TestIntegration_D7FrameDelta_E2E_SpansAndMonotonicity ./tests/integration/d7/...` 后,**实测 baseline**:

| Span Op | 实测 baseline (post #443 + #444 hotfix) | 目标 (S3-Rewrite) |
|---------|----------------------------------------|-------------------|
| `D7_Execute_ConvergenceMetric_Emit` (Phase 3) | 2 | ≥ 5 |
| `D7_Execute_PlanFrameDelta_Inject` (Phase 1) | 2 | ≥ 5 |
| `D7_Observe_PriorDelta_Inject` (Phase 2) | 2 (e2e);0 (production,需 DM-20260706-004) | ≥ 5 (e2e);production 由 sibling change 修复 |

**Sibling hotfixes (DM-20260706-002 + 003) 已 wired production code (commit 8b720e39 + 9c5fc866):**

- `#443` (DM-20260706-002):`workitem_executor.go` +19 行 binder,显式读取 `ec.PlanFrameDelta` 并调用 `InjectPlanFrameDelta`
- `#444` (DM-20260706-003):Phase 1+2 emit sites 修 `end()` 调用,使 span 真正 reach Jaeger

Phase 1 production wiring 闭环 (DM-20260706-002)。Phase 2 production wiring 仍有 gap,详见 sibling DM-20260706-004 (observation_proposer.go:257 硬编码 nil → 需函数签名 +1 参数 + upstream caller 传参)。

### 1.2 Why (S3-Rewrite 更新)

**Scope 收窄原因 (S3-Gate codex BLOCKED 反馈):**

1. Phase 1 production wiring 已闭环 (#443) — 原 change "production code 0 修改" 承诺需更新为 "production Phase 1 已合规"
2. Phase 2 production wiring 缺失是独立 production-side gap — 需 sibling change DM-20260706-004 修复 (production code 必要修改)
3. 原 change scope 收窄为 testutil_only e2e 覆盖提升 (Phase 1+2 span count 从实测 baseline 2 提升到 ≥ 5)

**Spec/code 一致性的最后一道闸门仍成立:** spec 声称 FrameDelta 协议横跨 Observe→Plan→Execute 三节点,testutil_only e2e 场景需覆盖完整 5 节点调用链 (而非当前 baseline 2 round cycles)。

## 2. 问题陈述(S3-Rewrite 更新)

### 2.1 E2E Phase 1 span 未触发(实际触发 2,目标 ≥ 5)

| 测试场景 | Phase 1 span | 实测 baseline | 目标 |
|---------|-------------|---------------|------|
| `TestIntegration_D7FrameDelta_E2E_SpansAndMonotonicity` | `d7.s9.execute.plan_frame_delta.inject` | 2 (e2e scenario 仅 2 round cycles) | ≥ 5 (扩展 e2e scenario 到 5 round cycles) |

### 2.2 E2E Phase 2 span 未触发(实际触发 2,目标 ≥ 5)

| 测试场景 | Phase 2 span | 实测 baseline | 目标 |
|---------|-------------|---------------|------|
| `TestIntegration_D7FrameDelta_E2E_SpansAndMonotonicity` | `d7.s5.observe.prior_delta.span` | 2 (e2e scenario 仅 2 round cycles) | ≥ 5 (扩展 e2e scenario 到 5 round cycles) |

### 2.3 Production Phase 2 wiring 缺失(由 sibling DM-20260706-004 处理)

production 代码 `observation_proposer.go:257` 硬编码 `nil` prevExecCtx,导致生产环境 Phase 2 span count 永远 0。本 change 不处理 (testutil_only scope 守住),由 sibling DM-20260706-004 修复。

### 2.4 单元测试已覆盖但 e2e scenario 仅 2 cycles(S3-Rewrite focus)

| T-ID | 描述 | 状态 |
|------|------|------|
| D7-S9-A112-T01..T05 | InjectPlanFrameDelta 5 子测试 | 5/5 PASS (unit) |
| D7-S5-A111-T01..T06 | BuildObservePriorDelta 6 子测试 | 6/6 PASS (unit) |
| D7-S9-A113-T01..T05 | ComputeConvergenceMetric 5 子测试 | 5/5 PASS (unit) |
| L5-MUPS-FD-3 (T15) | Execute 5 sub-turn 全 convergence_metric span + 末轮 rate ≥ 0.5 | PASS (e2e Phase 3) |
| **L5-MUPS-FD-6 (新增)** | **e2e scenario 扩展到 5 round cycles,Phase 1+2 span count ≥ 5** | **本 change 目标** |
| **L5-MUPS-FD-7 (新增, sibling DM-20260706-004)** | **production wiring e2e span 触发链路** | **sibling change 目标** |

## 3. 验收标准(S3-Rewrite 更新)

| ID | 标准 | 优先级 | 验证方式 |
|----|------|--------|----------|
| AC1 | `TestIntegration_D7FrameDelta_E2E_SpansAndMonotonicity` Phase 1 span (`d7.s9.execute.plan_frame_delta.inject`) ≥ 5 (e2e scenario 扩展到 5 round cycles) | P0 | memory exporter inspection |
| AC2 | `TestIntegration_D7FrameDelta_E2E_SpansAndMonotonicity` Phase 2 span (`d7.s5.observe.prior_delta.span`) ≥ 5 (e2e scenario 扩展) | P0 | memory exporter inspection |
| AC3 | Phase 3 span (`d7.s9.execute.convergence_metric.emit`) ≥ 5 (无回归,e2e scenario 同步扩展) | P0 | memory exporter inspection |
| AC4 | SequenceLLMStub callback `FrameDeltaInject func(idx int) FrameDelta` + SeedPriorExecContext helper testutil 注释明确 "testutil only, NOT production" | P0 | `tests/testutil/d7_stack.go` + design.md §5 |
| AC5 | sibling DM-20260706-004 production wiring 后,本 change testutil `SeedPriorExecContext` helper 仍兼容 (testutil seed 注入 mock state,不破坏 production wiring) | P1 | sibling S5 联动验证 |
| AC6 | 现有 71 frame 测试 + 16 D7-FD unit 测试 0 行为变化 | P0 | `go test -race ./internal/layers/orchestration/sessionorchestrator/...` |
| AC7 | 跨链 prompt size 单调性测试在 e2e scenario 扩展到 5 round cycles 后仍 PASS (last ≤ first*3) | P1 | `TestIntegration_D7FrameDelta_ConvergenceMonotonic` |
| AC8 | L5-MUPS-FD-6 T-IDs 在 `openspec/specs/d7-orchestration/t-registry.md` D7-FD 段登记 | P1 | t-registry diff |

## 4. 依赖与约束

| 类型 | 内容 |
|------|------|
| 依赖 | 父 change DM-20260705-010 (Phase 1-4 已落地,PR #431-#439 全 MERGED) |
| 依赖 | **DM-20260706-002 PR #443** (Phase 1 production wiring — workitem_executor.go binder) |
| 依赖 | **DM-20260706-003 PR #444** (Phase 1+2 emit end() wiring) |
| 依赖 | sibling **DM-20260706-004** (Phase 2 production wiring — observation_proposer.go nil → prevExecCtx) |
| 依赖 | `tests/testutil/d7_stack.go` ObsConfig 字段已存在 (PR #437 引入) |
| 约束 | append-only 注入原则不变 (M1/M2 契约 0 修改) |
| 约束 | 0 LLM 调用承诺不变 (testutil callback 不调真 LLM) |
| 约束 | **testutil_only scope** (production code 0 修改;Phase 2 production wiring 由 sibling DM-20260706-004 处理) |
| 约束 | sequenceLLMStub 接口不变 (testutil 兼容性) |

## 5. 变更范围(S3-Rewrite 更新)

### 新增 / 修改 / 不变更

| 操作 | 路径 | 描述 |
|------|------|------|
| MODIFIED | `tests/testutil/d7_llm_stub.go` | SequenceLLMStub 增加 `FrameDeltaInject` callback + docstring 明确 testutil-only |
| MODIFIED | `tests/testutil/d7_stack.go` | NewD7TestStack 内部追加 WorkItemExecContext 注册 (供 SeedPriorExecContext 使用) |
| NEW | `tests/testutil/d7_frame_delta_helpers.go` | `SeedPriorExecContext(stack, workItemID, priorRound)` helper (seed `Item.LastRound.ArtifactSummary`) |
| MODIFIED | `tests/integration/d7/d7_frame_delta_e2e_test.go` | `TestIntegration_D7FrameDelta_E2E_SpansAndMonotonicity` 扩展到 5 round cycles + Phase 1+2 span count 断言 (AC1 + AC2);新增 sub-test `Phase1And2SpanTrigger` + `MonotonicWithSeed` |
| NEW | `openspec/specs/d7-orchestration/t-registry.md` (D7-FD 段) | L5-MUPS-FD-6 T-IDs 登记 |
| NEW | `openspec/specs/d7-orchestration/CHANGELOG.md` (顶部) | IMPLEMENTED 条目 |
| NEW | `openspec/specs/d7-orchestration/mups-frame-delta-spec.md` §3.4 | "Phase 1+2 span e2e 触发条件 (testutil)" 新章节 |

### 不变更(S3-Rewrite scope 收窄)

- `internal/layers/orchestration/sessionorchestrator/{convergence_metric,observe_frame_delta,execute_plan_frame_inject}.go` 0 修改 (Phase 1+2 production code 已合规 via #443 + #444)
- `internal/layers/orchestration/sessionorchestrator/observation_proposer.go` 0 修改 (**Phase 2 production wiring 由 sibling DM-20260706-004 处理**)
- `internal/layers/orchestration/sessionorchestrator/item_observe.go` 0 修改 (**同上,sibling change 处理**)
- `internal/layers/orchestration/hardening/emitter.go` 0 修改 (span emit 已合规)
- M1 ObservationFrame 9 字段 / M2 StrategicPlanFrame 16 字段契约 0 修改
- append-only 注入原则 + 0 LLM 承诺不变
- sibling DM-20260706-004 change 目录 0 修改 (本 change 与 sibling 互补)

## 6. 风险评估(S3-Rewrite 更新)

| 风险 | 影响 | 缓解 |
|------|------|------|
| SequenceLLMStub callback `FrameDeltaInject` 引入 stub vs production 行为差异 | High | design.md §5 文档化差异;在 stub 注释中明确 "testutil only, NOT production";callback 签名 `(idx int) FrameDelta`,FrameDelta 是 5 字段纯值对象,production FrameDelta 字段变化时 callback 自动适配 |
| `SeedPriorExecContext` 影响 WorkItemExecContext 单例状态污染其他 sub-test | Medium | setup/teardown 显式 reset: `obsConfig.MemoryExporter.Reset()` + `prevExecCtx` atomic.Pointer.Store(nil) |
| e2e scenario 扩展到 5 round cycles 破坏 prompt size monotonicity (AC7 last ≤ first*3) | Medium | 独立 sub-test `TestIntegration_D7FrameDelta_MonotonicWithSeed` 验证;不动原 `TestIntegration_D7FrameDelta_ConvergenceMonotonic` |
| testutil `d7_stack.go` 新增 `SeedPriorExecContext` 函数影响其他 D7 测试 | Low | 函数显式 only-for-seed 测试调用,其他 D7 测试不调即保持原行为 |
| sibling DM-20260706-004 production wiring 后,本 change testutil seed 与 production wiring 冲突 | Medium | AC5 显式验证;testutil mock state 与 production wiring 独立;sibling S5 验收时联动 |
| Phase 2 production wiring 缺失 (observation_proposer.go:257 hardcoded nil) | High (out of scope, 由 sibling 处理) | DM-20260706-004 独立 production-side 修复;本 change 不混入 |

## 7. 关联

### 父 Change

- `devrix-d7-mups-frame-delta-closure` (DM-20260705-010, S7_Archived 2026-07-05, PR #431-#439 全 MERGED) — Phase 4 §8.3 文档化此 gap

### Sibling Change

- `devrix-d7-frame-delta-phase2-production-wiring` (DM-20260706-004, status S1_Proposal 2026-07-08)
  - production-side 修复:observation_proposer.go:257 nil → prevExecCtx + item_observe.go upstream caller 传参
  - 互补:本 change testutil_only + e2e scenario 扩展;sibling change production wiring
  - S3-Gate codex review 2026-07-08 BLOCKED 拆分决定

### Sibling Hotfixes (parent change 同日合并)

- `DM-20260706-002 PR #443` (commit 8b720e39):Phase 1 production wiring — workitem_executor.go binder
- `DM-20260706-003 PR #444` (commit 9c5fc866):Phase 1+2 emit end() wiring

### 关联 Demand

- DM-20260705-010 (frame-delta-closure parent)
- DM-20260706-002 (Phase 1 production wiring hotfix)
- DM-20260706-003 (Phase 1+2 emit end() wiring hotfix)
- DM-20260706-004 (sibling Phase 2 production wiring,拆分自本 change)
- DM-20260629-007/008 (TaskContract 统一 — 提供 WorkItemExecContext atomic.Pointer 接口)
- DM-20260625-019 (MUPS 5-node Coverage — 提供 ObsConfig testutil 基础)
- DM-20260617-008 (D7 TestFramework — 提供 SequenceLLMStub + D7StackOptions 基础设施)

### 关联 Test (前置)

- `tests/integration/d7/d7_frame_delta_e2e_test.go` `TestIntegration_D7FrameDelta_E2E_SpansAndMonotonicity` (Phase 4 §8.3 gap 来源)
- `tests/integration/d7/d7_frame_delta_e2e_test.go` `TestIntegration_D7FrameDelta_ConvergenceMonotonic`
- `tests/testutil/d7_stack.go` ObsConfig 字段 (DM-20260705-010 PR #437 引入)
- `tests/testutil/d7_llm_stub.go` SequenceLLMStub (DM-20260617-008 引入)

### 关联 PR (前置)

- #437 (Phase 4 e2e trace replay — ObsConfig 字段基础)
- #438 (Phase 4 sync + span attr key alignment)
- #439 (demand-archive-index ACCEPTED sync)
- #443 (DM-20260706-002 Phase 1 production wiring — sibling hotfix)
- #444 (DM-20260706-003 Phase 1+2 emit end() wiring — sibling hotfix)
- #440 (S1 demand.md entry — 本 change S1 已落地)