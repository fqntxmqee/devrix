---
demand-id: DM-20260705-005
title: "MUPS Verify 节点决策表化 — 4 VerdictKind × N trigger 表驱动重构"
source: MUPS 5 节点重构路线图（M4 Verify decision-table）
priority: P1
status: S3_Design
l1-domain: orchestration
created: 2026-07-05
related:
  - openspec/specs/d7-orchestration/spec.md
  - openspec/specs/d7-orchestration/pipeline-architecture.md
  - internal/layers/orchestration/sessionorchestrator/item_verify.go
  - internal/layers/orchestration/sessionorchestrator/rollup_verify.go
  - internal/layers/orchestration/sessionorchestrator/item_pipeline.go
  - internal/layers/orchestration/workmodel/aggregate_verdicts.go
  - internal/shared/types/verdict.go
  - openspec/changes/mups-go-struct-driven/  # M1
  - openspec/changes/mups-plan-structbind/   # M2
parent_demands:
  - DM-20260705-003  # M1 Observe go-struct-driven
  - DM-20260705-004  # M2 Plan go-struct-driven
---

# MUPS Verify 节点决策表化 — M4

## 1. 原始描述

> MUPS Verify 节点当前由 `sessionorchestrator/{item_verify,rollup_verify}.go` 三个手工 `verify*` 函数实现：嵌套 if/switch 把 4 种 VerdictKind × 多类 trigger（nil artifact / execute error / max_iters+tool_calls / SideEffectStatus / user-gate phrase / scope-only / deliverable contract incomplete / rollup child stats）的决策逻辑散落在函数体中。新增 trigger（如未来 LLM Verifier 注入、特定 plan kind 升级、parser failure）必须修改函数体；trigger 顺序、置信度、Reason 文案缺乏显式声明。本 change 以 **trigger × verdict-template 决策表** 模式重构，把 trigger 抽取为命名函数，verify 函数变成"构建表 → 应用表"。

## 2. 问题陈述（现状诊断）

### 2.1 已有能力（不重复建设）

| 能力 | 状态 | 路径 |
|------|------|------|
| `workmodel.Verdict` 4 字段 + `WithKind/WithConfidence/WithReason/WithSourceID/WithIndeterminateReason` 不可变 builder | ✅ | `workmodel/aggregate_verdicts.go:24-95` |
| `types.VerdictKind` 4 态 typed enum + `String/Parse/Marshal/Unmarshal` | ✅ | `shared/types/verdict.go:24-50` |
| `verifyArtifact(art)` Phase B 决策 | ✅ | `sessionorchestrator/item_verify.go:30` |
| `verifyArtifactForWorkItem(art, item, pl)` 4 overlay layer | ✅ | `sessionorchestrator/item_verify.go:108` |
| `verifyArtifactForWorkItemWithSchema/WithContract` DM-20260630-012 升级 | ✅ | `sessionorchestrator/item_verify.go:99-105` |
| `verifyRollupArtifact(art, stats)` rollup-specific gates | ✅ | `sessionorchestrator/rollup_verify.go:13` |
| 现有 13 测试覆盖 (item_verify + deliverable_verify + item_pipeline_rollup) | ✅ | `sessionorchestrator/*_test.go` |

### 2.2 缺口（trigger 散落 + 顺序隐式）

| 函数 | 隐式 trigger | 显式度 |
|------|--------------|--------|
| `verifyArtifact` (49 行) | (1) nil artifact → Indeterminate; (2) Error/ExitCode + max_iters+tool_calls>0 → Partial(0.55); (3) Error/ExitCode else → Fail(0.9); (4) SideEffectRolledBack → Fail(0.85); (5) SideEffectUnknown/Inflight → Partial(0.6); (6) default → Pass(0.9) | trigger 隐含在 if/switch 链里 |
| `verifyArtifactForWorkItemWithContract` (54 行) | 在 verifyArtifact 之上叠加：(7) user-gate phrase/regex → Partial(0.85); (8) ExplorationPlan + scope-only + 无 file:line citation → Partial(0.8); (9) DeliverableStatusIncomplete + 原 Pass → downgrad 到 Partial(0.65) | trigger 7/8/9 写在 `v := verifyArtifact(art); if X { v = ... }` 链里 |
| `verifyRollupArtifact` (47 行) | (1) nil/Error/ExitCode → verifyArtifact 委托; (2) Failed==Total → Fail(0.95); (3) Failed>0 && Running>0 → Partial(0.8); (4) empty summary → Fail(0.9); (5) RollupDeliverableContract 不满足 → Fail(0.85); (6) default → Pass(0.9) | trigger 1-6 写在 if/switch 链里 |

**风险（trigger 散落导致的 4 类问题）**：
1. **顺序依赖**：trigger 7 (user-gate) 在 verifyArtifact 之后应用，但 trigger 9 (deliverable incomplete) 期望原 verdict 是 Pass 时才 downgrade。顺序错位 → 静默错误。
2. **置信度漂移**：每条 trigger 硬编码 0.55/0.6/0.65/.../0.95，10+ 魔数散落；改一个数字必须找全。
3. **Reason 文案不一致**：相同 trigger (e.g. Execute failed) 在不同函数里文案可能漂移（"execute failed: %s" vs "execute failed: %s with exit_code %d"）。
4. **新增 trigger 风险**：未来加 trigger (e.g. LLM Verifier 注入、plan kind 升级路径、parser failure) 必须修改 3 个函数；新加的 trigger 在哪个函数、放在哪个 if 分支，缺乏单一权威位置。

### 2.3 目标行为（trigger × verdict-template 决策表）

1. **Trigger 命名函数**：每个 trigger 一个 `detectXxx(art, ctx) (verdict *workmodel.Verdict, fired bool)` 函数，命名 + 签名 + 单测一目了然。
2. **决策表数据结构**：`VerifyDecisionTable` 是一组有序的 `(Trigger, VerdictTemplate)`；`applyDecisionTable(table, art, ctx) Verdict` 顺序遍历，第一个 fired trigger 的 verdict 返回。
3. **3 个 verify 函数变成"建表 + 应用表"**：`verifyArtifact` 构造 6-trigger 表；`verifyArtifactForWorkItemWithContract` 在 6-trigger 表后追加 3 overlay trigger；`verifyRollupArtifact` 构造 6-trigger 表（含 child stats gating）。
4. **0 行为变化（M4）**：13 个现有测试 + 4 字节级 golden snapshot（参见 §2.4）全 PASS。
5. **可读性 / 可维护性**：新增 trigger = 1 个新 `detectXxx` 函数 + 1 行表注册；trigger 顺序、置信度、Reason 文案集中在表里。

### 2.4 0 行为变化验证矩阵

| 验证维度 | 工具 | 期望 |
|----------|------|------|
| 现有 13 测试 PASS | `go test ./internal/layers/orchestration/sessionorchestrator/ -run "VerifyArtifact\|VerifyRollupArtifact\|VerifyArtifactForWorkItem" -race -count=1` | 13/13 PASS |
| 字节级 verdict 等价 | 新增 `TestVerifyArtifactRefactor_ByteEquivalent_OldVsNew` 比较新旧实现对 6 artifact 组合 (nil / max_iters+tool / max_iters+no_tool / exit=1+error / SideEffectRolledBack / SideEffectInflight / Pass-default) 输出 | 7/7 byte-equal |
| trigger 顺序锁定 | 新增 `TestVerifyArtifact_DetectorOrder` 验证 11 detector 按预期顺序被调用 | 11 detector 顺序断言 |

### 2.5 重构总图（5 节点，M4 落点）

| 阶段 | 范围 | 行为变化 | 本 change 落点 |
|------|------|----------|-----------------|
| M1 | Observe go-struct 化 | 0 行为变化 | ✅ S7_archived (DM-20260705-003) |
| M2 | Plan go-struct 化（kernel 复用） | 0 行为变化 | ✅ S7_archived (DM-20260705-004) |
| **M4** | **Verify 决策表化** | **0 行为变化** | **本 change** |
| M5 | SpawnDecision 3 子决策代数化（R0-R8） | 0 行为变化 | follow-on (`d7-spawn-decision-algebra`) |
| M3 | Strategy 抽象注入 WorkItemExecContext | 行为增量（PlanKind 路由恢复） | 最后做 (`d7-mups-strategy-injection`) |

**为什么 M4 现在做**：M1+M2 kernel 已稳定；M3 行为增量最大风险，最后做；M4 + M5 是 0 行为变化局部表驱动化，并行可行。

## 3. 非目标

- 修改 `workmodel.Verdict` 4 字段（保持不变）
- 修改 `types.VerdictKind` 4 态枚举
- 修改 `exitReasonForVerdict` / `VerdictToExitReason` 映射
- LLM Verifier 真实注入（仅留 `ItemPipelineDeps.Verifier` 现有接口；本 change 不改 verifier 调用链）
- 触发 PlanKind 路由（属 M3）
- 触发 `SpawnPolicy` 升级路径（属 M5）
- 任何 Execute / Observe / Plan 节点改造
- 跨域 LLM 节点（D3 LLMGateway）改造

## 4. 澄清记录

### Q1: 决策表 vs switch 抽象的边界？
**A**: 决策表是 `[]VerifyTrigger` 有序 slice；每个 trigger 是 `(name string, fire func(*wavescheduler.Artifact, *verifyContext) bool, template VerdictTemplate)` 三元组。`applyDecisionTable` 从头遍历，第一个 `fire==true` 的 trigger 应用其 template 返回；都不 fire 则返回 default verdict (Pass 0.9)。**trigger 是 Go 函数，不是字符串/反射**，避免字符串配置导致的二义性。 — 2026-07-05

### Q2: 上下文 ctx 是什么？
**A**: `verifyContext` 是不可变结构体，含 `art *wavescheduler.Artifact` / `item *workmodel.WorkItem` / `pl *plan.Plan` / `contract workmodel.DeliverableContract` / `stats workmodel.ChildOutcomeStats` / `id string`（art.TaskID 兜底 "artifact_unknown"）。每个 verify 函数构造自己的 ctx，然后传 trigger 函数。 — 2026-07-05

### Q3: 是否需要 shared kernel？
**A**: **否**。M1/M2 shared structbind kernel 是 LLM I/O 形状反射注册；M4 决策表是 D7 内部行为规则，不需要跨域共享。决策表 + detector 全部在 `sessionorchestrator/` 包内。 — 2026-07-05

### Q4: 决策表是否动态构造？
**A**: **否**。决策表是包级 `var` 常量，在 `init()` 之前已就绪；`verifyArtifact` 不修改表，只读表。表的内容 = 硬编码 11 trigger 列表（与现状 if/switch 链 1:1 对应）。 — 2026-07-05

### Q5: 0 行为变化如何保证？
**A**: 3 重保险：(1) 现有 13 测试 0 修改必须 PASS；(2) 新增 7 byte-equivalent 测试用 `verifyArtifact` 旧版（保存为 `verifyArtifactLegacy` 仅供测试）+ 新版对比 6 组合；(3) 新增 detector 顺序测试验证 11 detector 调用次序。 — 2026-07-05

### Q6: `verifyArtifactLegacy` 是不是死代码？
**A**: **是**，但仅作为测试用 reference 保留在 `_legacy_test.go`（带 build tag `legacy_verify` 仅 test 编译）；生产代码 `verifyArtifact` 100% 走决策表。S5 验收通过后，下个 change (`mups-cleanup-legacy`) 删除 `_legacy_test.go`。 — 2026-07-05

## 5. L1-L5 映射

| 层级 | 资产 ID | 名称 | 状态 |
|------|---------|------|------|
| L1 | d7-orchestration | Verify 节点 | 已有 |
| L2 | L2-ORCH-MUPS-VERIFY | 决策表化 | 改造 |
| L3-BE | D7-S10 | Verify Node (MUPS v4.3 Phase 4) | 改造 |
| L4-BE | **D7-S10-A101** | **verify_decision_table kernel（11 trigger + 决策表 + applyDecisionTable）** | **新增** |
| L4-BE | **D7-S10-A101** | **verifyArtifact / verifyArtifactForWorkItemWithContract / verifyRollupArtifact 走表驱动** | **新增** |
| L5 | **L5-MUPS-VTD-01** | **11 detector 命名函数 + 单测** | **草拟 P0** |
| L5 | **L5-MUPS-VTD-02** | **`applyDecisionTable` 顺序遍历 + 第一个 fired trigger 返回 + default verdict 兜底** | **草拟 P0** |
| L5 | **L5-MUPS-VTD-03** | **`verifyArtifact` 重构后 6 artifact 组合字节等价旧实现** | **草拟 P0** |
| L5 | **L5-MUPS-VTD-04** | **`verifyArtifactForWorkItemWithContract` 重构后 4 overlay layer 字节等价旧实现** | **草拟 P0** |
| L5 | **L5-MUPS-VTD-05** | **`verifyRollupArtifact` 重构后 6 rollup 组合字节等价旧实现** | **草拟 P0** |
| L5 | **L5-MUPS-VTD-06** | **现有 13 测试 0 修改 PASS + L5-VTD-03/04/05 byte-equivalent** | **草拟 P0** |
| L5 | **L5-MUPS-VTD-07** | **detector 顺序锁定测试 11 detector 顺序断言** | **草拟 P1** |

## 6. 验收标准

- **P0**：`go vet ./...` + `go test ./internal/layers/orchestration/sessionorchestrator/... -race -count=1` 全 PASS（13 现有测试 + 7 新 byte-equivalent 测试）。
- **P0**：11 detector 命名清晰（`detectNilArtifact` / `detectExecuteError` / `detectMaxItersPartial` / `detectSideEffectRolledBack` / `detectSideEffectUncertain` / `detectUserGate` / `detectScopeOnlyDeliverable` / `detectDeliverableIncomplete` / `detectRollupAllFailed` / `detectRollupMixedFailedRunning` / `detectRollupEmptySummary` / `detectRollupDeliverableMissing` = 12 detector，列表按文件分两组）。
- **P0**：`verifyArtifact` / `verifyArtifactForWorkItemWithContract` / `verifyRollupArtifact` 函数体 ≤ 30 行（≤ 现状 50%）。
- **P0**：决策表 `VerifyDecisionTable` 是包级 `var` 常量，11 trigger 注册一次。
- **P0**：trigger 顺序、置信度（0.55/0.6/0.65/0.8/0.85/0.9/0.95）、Reason 文案、SourceID 全部 1:1 保留。
- **P1**：`t-registry.md` D7-S10-A101 注册 7 T 点（IMPLEMENTED 297→304，P0 254→260）。
- **P1**：`specs/d7-orchestration/spec.md` §D7-S10 delta 新增 verify_decision_table Requirement。
- **P1**：`CHANGELOG.md` d7-orchestration 追加 M4 行。

## 7. 规划状态

- [x] S1 `demand.md`（本文）
- [x] S2 `proposal.md`
- [x] S3 `design.md` + `specs/d7-orchestration/spec.md` delta
- [x] S4 `tasks.md`（P0/P1 拆解）
- [ ] S4 实现（verify_decision_table.go + 3 verify 函数改造 + 7 测试）
- [ ] S5 验收（7 L5 + 13 现有 + 0 行为变化）
- [ ] S6-交付（PR squash → master）
- [ ] S6-归档（`archive/2026-07-05-mups-verify-table-driven/`）
