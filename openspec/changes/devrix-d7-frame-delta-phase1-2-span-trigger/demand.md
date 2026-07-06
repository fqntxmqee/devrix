---
demand-id: DM-20260706-001
title: D7 MUPS Frame Delta Phase 1+2 spans 端到端触发 — 真实路径或 mock 注入覆盖 e2e gap
priority: P1
status: S1_Proposal
dsaft_domain: orchestration
created: 2026-07-06
parent_change: devrix-d7-mups-frame-delta-closure
parent_demand: DM-20260705-010
---

# D7 MUPS Frame Delta Phase 1+2 spans 端到端触发

## 1. 背景

`devrix-d7-mups-frame-delta-closure` (DM-20260705-010) Phase 4 端到端验证通过 PR #437 (`tests/integration/d7/d7_frame_delta_e2e_test.go`) 闭环,2 子测试 (`TestIntegration_D7FrameDelta_E2E_SpansAndMonotonicity` + `TestIntegration_D7FrameDelta_ConvergenceMonotonic`) 全 PASS,8 AC AC1-AC8 全 PASS。

**但 Phase 4 e2e 测试中存在一个 spec 实施 gap:**

`TestIntegration_D7FrameDelta_E2E_SpansAndMonotonicity` 测试场景下,Phase 1 (`InjectPlanFrameDelta`) + Phase 2 (`BuildObservePriorDelta`) 的 span **未在 e2e 场景触发**。验证时只能看到 Phase 3 (`ComputeConvergenceMetric`) span ≥ 1,前两阶段的 span 在 memory exporter 中记录数为 0。

**根因(已确诊):**

1. Phase 1 `InjectPlanFrameDelta` 触发条件 = `plan.FrameDelta` 非零。e2e 测试用 `SequenceLLMStub` 模拟 Plan LLM 输出,但 stub 输出的 `StrategicPlanProposal` 在 FrameDelta 字段是零值(测试stub 未显式填充 ExecutionMode / ChildSpecs / DeliverableContract),导致 `injectPlanFrameDelta` 函数早返回不触发 span。
2. Phase 2 `BuildObservePriorDelta` 触发条件 = `prevExecCtx != nil` (即非首轮)。e2e 测试 `RouteInbound` 5 turns 中第 1 轮 `prevExecCtx` 为 nil,所以该轮 `BuildObservePriorDelta` 返回零值;后续 4 轮的 prevExecCtx 应该非零,但由于 Phase 1 未触发注入,Phase 3 的 ConvergenceMetric 也没有正常累积 prior-round 数据,导致 Phase 2 链路上的 FrameDelta 注入条件退化。

**Why:** 当前 e2e 测试覆盖到 Phase 3 span 即可证明 deterministic 0 LLM 计算路径正确,但无法验证 Phase 1 + Phase 2 的注入路径在真实运行链路中被触发。这是 spec/code 一致性的最后一道闸门:spec 声称 FrameDelta 协议横跨 Observe→Plan→Execute 三节点,但 e2e 测试只覆盖了 Execute→Observe 回写一半。

**How to apply:** 任何依赖 FrameDelta I/O 协议的 e2e 验收(包括未来 v1.1 TraceID 反向追溯、跨域 FrameDelta 抽象上提)都必须先通过这个 gap closure 才能算合规。

## 2. 问题陈述

具体要解决的问题:

### 2.1 E2E Phase 1 span 未触发

| 测试场景 | Phase 1 span | 期望 | 实际 |
|---------|-------------|------|------|
| `TestIntegration_D7FrameDelta_E2E_SpansAndMonotonicity` | `d7.s9.execute.plan_frame_delta.inject` | ≥ 5 (5 turns × 1 inject/turn) | **0** (synthetic stub 未触发 StrategicPlanProposal 非零) |

### 2.2 E2E Phase 2 span 未触发

| 测试场景 | Phase 2 span | 期望 | 实际 |
|---------|-------------|------|------|
| `TestIntegration_D7FrameDelta_E2E_SpansAndMonotonicity` | `d7.s5.observe.prior_delta.span` | ≥ 4 (Round 2-5 各 1 次) | **0** (Round 1 prevExecCtx=nil + Round 2-5 因 Phase 1 未触发 prior 累积空) |

### 2.3 单元测试已覆盖但 e2e gap 仍在

| T-ID | 描述 | 状态 |
|------|------|------|
| D7-S9-A112-T01..T05 | InjectPlanFrameDelta 5 子测试 | 5/5 PASS (unit) |
| D7-S5-A111-T01..T06 | BuildObservePriorDelta 6 子测试 | 6/6 PASS (unit) |
| D7-S9-A113-T01..T05 | ComputeConvergenceMetric 5 子测试 | 5/5 PASS (unit) |
| L5-MUPS-FD-3 (T15) | Execute 5 sub-turn 全 convergence_metric span + 末轮 rate ≥ 0.5 | PASS (e2e 仅 Phase 3) |
| **L5-MUPS-FD-6 (新增)** | **Execute 5 turns 全 plan_frame_delta.inject span + Observe Round 2-5 全 prior_delta.span** | **GAP** |

## 3. 验收标准

| ID | 标准 | 优先级 | 验证方式 |
|----|------|--------|----------|
| AC1 | `TestIntegration_D7FrameDelta_E2E_SpansAndMonotonicity` Phase 1 span (`d7.s9.execute.plan_frame_delta.inject`) ≥ 5 | P0 | memory exporter inspection |
| AC2 | `TestIntegration_D7FrameDelta_E2E_SpansAndMonotonicity` Phase 2 span (`d7.s5.observe.prior_delta.span`) ≥ 4 | P0 | memory exporter inspection |
| AC3 | 修复方案不破坏 Phase 3 span (`d7.s9.execute.convergence_metric.emit`) ≥ 5 | P0 | memory exporter inspection |
| AC4 | SequenceLLMStub 真实路径 OR mock 注入触发有明确文档化依据 | P0 | `tests/testutil/d7_stack.go` ObsConfig 字段注释 + design.md §5 |
| AC5 | 现有 71 frame 测试 + 16 D7-FD unit 测试 0 行为变化 | P0 | `go test -race ./internal/layers/orchestration/sessionorchestrator/...` |
| AC6 | 跨链 prompt size 单调性测试在 Phase 1+2 真实触发后仍 PASS (last ≤ first*3) | P1 | `TestIntegration_D7FrameDelta_ConvergenceMonotonic` |
| AC7 | L5-MUPS-FD-6 新增 T 在 `openspec/specs/d7-orchestration/t-registry.md` D7-FD 段登记 | P1 | t-registry diff |

## 4. 依赖与约束

| 类型 | 内容 |
|------|------|
| 依赖 | `devrix-d7-mups-frame-delta-closure` (DM-20260705-010) Phase 1-4 已落地 (PR #431-#439 全 MERGED) |
| 依赖 | `tests/testutil/d7_stack.go` ObsConfig 字段已存在 (PR #437 引入) |
| 依赖 | `internal/layers/orchestration/sessionorchestrator/strategic_plan.go` `StrategicPlanProposal` FrameDelta 字段已定义 |
| 依赖 | `internal/layers/orchestration/sessionorchestrator/workitem_exec_context.go` `prevExecCtx` prior-round gate 已实现 |
| 约束 | append-only 注入原则不变 (M1/M2 契约 0 修改) |
| 约束 | 0 LLM 调用承诺不变 (Phase 2/3 是 deterministic 纯函数) |
| 约束 | sequenceLLMStub 接口不变 (testutil 兼容性) |

## 5. 变更范围

### 新增 / 修改 / 不变更

| 操作 | 路径 | 描述 |
|------|------|------|
| MODIFIED | `tests/testutil/d7_stack.go` | SequenceLLMStub 增加 `InjectFrameDelta` 钩子 + Round 1 prevExecCtx seed |
| MODIFIED | `tests/integration/d7/d7_frame_delta_e2e_test.go` | `TestIntegration_D7FrameDelta_E2E_SpansAndMonotonicity` 增加 Phase 1 + Phase 2 span 计数断言 (AC1 + AC2) |
| NEW | `tests/integration/d7/d7_frame_delta_e2e_test.go` (新增 sub-test) | `TestIntegration_D7FrameDelta_Phase1And2SpanTrigger` 独立验证 Phase 1 + Phase 2 span 真实触发链路 |
| NEW | `openspec/specs/d7-orchestration/t-registry.md` (D7-FD 段) | L5-MUPS-FD-6 T-IDs 登记 |
| NEW | `openspec/specs/d7-orchestration/CHANGELOG.md` (顶部) | IMPLEMENTED 条目 |
| NEW | `openspec/specs/d7-orchestration/mups-frame-delta-spec.md` §3.4 | "Phase 1+2 span e2e 触发条件" 新章节 |

### 不变更

- `internal/layers/orchestration/sessionorchestrator/{convergence_metric,observe_frame_delta,execute_plan_frame_inject}.go` 0 修改 (production code 已合规)
- `internal/layers/orchestration/hardening/emitter.go` 0 修改 (span emit 已合规)
- M1 ObservationFrame 9 字段 / M2 StrategicPlanFrame 16 字段契约 0 修改
- append-only 注入原则 + 0 LLM 承诺不变

## 6. 风险评估

| 风险 | 影响 | 缓解 |
|------|------|------|
| SequenceLLMStub 真实路径触发会引入 LLM 真实调用,违反 0 LLM 承诺 | High | 用 mock 注入 `StrategicPlanProposal.FrameDelta` 非零值 + Round 1 显式 seed `prevExecCtx`;不调用真 LLM |
| `prevExecCtx` seed 会影响 `WorkItemExecContext` 单例状态 | Medium | 在 sub-test setup/teardown 中显式 reset `obsConfig.MemoryExporter.Reset()` + `prevExecCtx` atomic.Pointer.Store(nil) |
| Phase 1+2 span 触发后,Phase 3 convergence 计算因 prior 数据累积而变化,可能破坏 last ≤ first*3 单调性 | Medium | 在 `TestIntegration_D7FrameDelta_Phase1And2SpanTrigger` 独立 sub-test 中验证,不动原 `TestIntegration_D7FrameDelta_ConvergenceMonotonic` |
| testutil d7_stack.go 新增 `InjectFrameDelta` 字段会影响其他 D7 测试 | Low | 字段默认 zero value,其他 D7 测试不传即保持原行为 |
| Phase 1+2 真实触发链路与 running system 行为不一致 (stub 触发 ≠ running system) | Medium | 在 design.md §5 文档化 stub vs running system 差异;runningsystem 真实测试通过独立飞书 session 验证 (out of scope for this change) |

## 7. 关联

### 父 Change

- `devrix-d7-mups-frame-delta-closure` (DM-20260705-010, S7_Archived 2026-07-05, PR #431-#439 全 MERGED) — Phase 4 §8.3 文档化此 gap

### 关联 Demand

- DM-20260705-010 (frame-delta-closure parent)
- DM-20260629-007/008 (TaskContract 统一 — 提供 WorkItemExecContext atomic.Pointer 接口)
- DM-20260625-019 (MUPS 5-node Coverage — 提供 ObsConfig testutil 基础)

### 关联 Test

- `tests/integration/d7/d7_frame_delta_e2e_test.go` `TestIntegration_D7FrameDelta_E2E_SpansAndMonotonicity`
- `tests/integration/d7/d7_frame_delta_e2e_test.go` `TestIntegration_D7FrameDelta_ConvergenceMonotonic`
- `tests/testutil/d7_stack.go` `ObsConfig` 字段

### 关联 PR (前置)

- #437 (Phase 4 e2e trace replay — 提供 ObsConfig 字段基础)
- #438 (Phase 4 sync + span attr key alignment)
- #439 (demand-archive-index ACCEPTED sync)