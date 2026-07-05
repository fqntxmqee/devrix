---
demand-id: DM-20260705-006
title: "MUPS Spawn 决策代数化 — R0-R8 嵌套 if 拆为 3 个命名子决策 (checkBudget / checkRollupGuard / checkVerdictDirection)"
source: MUPS 5 节点重构路线图（M5 SpawnDecision decision-algebra）
priority: P1
status: S3_Design
l1-domain: orchestration
created: 2026-07-05
related:
  - openspec/specs/d7-orchestration/spec.md
  - openspec/specs/d7-orchestration/pipeline-architecture.md
  - openspec/specs/d7-orchestration/mups-spawn-data-objects.md
  - openspec/specs/d7-orchestration/uncertainty-spawn-contract.md
  - internal/layers/orchestration/workmodel/spawn_policy.go
  - internal/layers/orchestration/workmodel/spawn_policy_test.go
  - internal/layers/orchestration/workmodel/spawn_policy_inline_test.go
  - internal/layers/orchestration/workmodel/evidence_progress.go
  - internal/layers/orchestration/workmodel/pipeline_round.go
parent_demands:
  - DM-20260705-003  # M1 Observe go-struct-driven
  - DM-20260705-004  # M2 Plan go-struct-driven
  - DM-20260705-005  # M4 Verify decision-table-driven
---

# MUPS Spawn 决策代数化 — M5

## 1. 原始描述

> MUPS Decide 节点的 `SpawnPolicyEvaluator(round, ctx) SpawnPolicy`（`workmodel/spawn_policy.go:19`）当前用 50+ 行嵌套 `if` + `switch round.VerdictKind` 把 R0-R8 共 9 条规则（含 rollup retry exhausted guard RH-MUPS-03 3 处重复）散落在一个函数体里。R0/R0.5/R1/R2 是 budget/gating 早退决策，R5/R6/R7 verdict 块内嵌套 rollup retry guard，R3/R4/R8 是 verdict-kind 路由。规则之间无显式边界，新加规则（未来 PlanKind 升级、Human Review 改造、budget 新维度）必须修改主函数体；rollup guard 在 3 个 verdict 块里重复 3 次，顺序错位（如 R1 之后才查 rollup）→ 静默错误。本 change 以 **3 个命名子决策**模式重构：把 R0/R0.5/R1/R2 抽为 `checkBudget(round, ctx) (SpawnPolicy, bool)` 早退门；把跨 verdict 共享的 rollup retry exhausted guard 抽为 `checkRollupGuard(round, ctx) (SpawnPolicy, bool)` 跨 verdict 守卫；R3-R8 抽为 `checkVerdictDirection(round, ctx) SpawnPolicy` 按 VerdictKind 路由。`SpawnPolicyEvaluator` 主函数体 50+→7-8 行"checkBudget → checkRollupGuard → checkVerdictDirection" 3 步显式声明。

## 2. 问题陈述（现状诊断）

### 2.1 已有能力（不重复建设）

| 能力 | 状态 | 路径 |
|------|------|------|
| `SpawnPolicy` 6 态 typed enum（None/Await/Inline/EscalateHuman/Decompose/ParallelExplore） | ✅ | `workmodel/spawn_policy.go` |
| `SpawnPolicyEvaluator(round, ctx) SpawnPolicy` 主函数 | ✅ | `workmodel/spawn_policy.go:19-90` (50+ 行) |
| `EvaluateSpawnPolicy(round, ctx)` 填 `round.SpawnPolicy` + `SpawnRationale` + `RollupSynthRequested` | ✅ | `workmodel/spawn_policy.go:145-152` |
| `spawnRationale(policy, round, ctx) string` 6 case 文案 | ✅ | `workmodel/spawn_policy.go:155-220` |
| `applicableDeliverableSchema` / `deliverableContinuationRequired` / `IsDeliverableInlineBudgetExhaustedFromCtx` / `deliverableInlineWouldExhaust` 4 个 helper | ✅ | `workmodel/spawn_policy.go:225-265` |
| `spawnForDeliverableContinuation(round, ctx)` 决策 | ✅ | `workmodel/evidence_progress.go:93-105` |
| `RollupSynthEligible(round, ctx)` + `IsExploratoryPlanKind` + `CanDecompose` 3 个依赖 | ✅ | `workmodel/evidence_progress.go:58` / `workmodel/pipeline_round.go:175` / `workmodel/decompose.go:102` |
| `WorkItemPipelineRound` + `TreeEvalContext` 2 不可变 struct | ✅ | `workmodel/pipeline_round.go:68 / 118` |
| 现有 22 测试 0 修改覆盖 R0-R8 全部 case | ✅ | `workmodel/spawn_policy_test.go` 21 + `spawn_policy_inline_test.go` 1 |
| RH-MUPS-03 (DM-20260701-001) rollup retry exhausted → EscalateHuman | ✅ | `spawn_policy_test.go:177-237` 4 case |

### 2.2 缺口（R0-R8 散落 + rollup guard 重复 + 边界隐式）

| 关注点 | 现状 | 风险 |
|--------|------|------|
| **R0-R8 边界** | 50+ 行函数体内 R0/R0.5/R1/R2 (4 budget gates) → switch (VerdictKind) → R5/R6/R7 各 1 block；无显式分组 | 难以一眼看出哪些是 budget / 哪些是 direction / rollup guard 嵌哪 |
| **rollup guard 重复** | R5/R6/R7 三个 verdict block 顶部都有相同 `if ctx.RollupRound { if ctx.RollupRetries >= ctx.MaxRollupRetries { return EscalateHuman } return Inline }` | 3 处重复 5 行；新加 VerdictKind 漏写 rollup guard → 静默无限循环；改逻辑（如 +1 retry 阈值）要改 3 处 |
| **normalizeCtx 5 行默认** | `MaxDepth/Threshold/MaxIndeterminateRetries/MaxRollupRetries/MaxInlineRetriesAtMaxDepth` 5 个 if `<=0` 兜底写在主函数体顶部 | 与"决策逻辑"混在一起；reader 第一眼分不清"配置兜底"和"决策" |
| **0 行为变化验证基线** | 22 现有测试 (21 + 1 inline) 覆盖 R0/R0.5/R1/R2/R3/R4/R5/R6/R7/R8 + 4 rollup guard case (Partial/Fail/Indeterminate/Pass)，但无 byte-equivalent 显式对比 | 重构引入顺序错位时无 sub-verdict 字节级 diff 工具 |
| **新增规则成本** | 加 1 条 spawn rule（如 R9 "用户暂停" / R10 "AutoClose 异步触发"）必须修改主函数体；插入位置（budget 之前？verdict 路由内？）无单一权威位置 | 散落 → 重复链路 → 二义性 |

**风险（散落 + rollup guard 重复导致的 3 类问题）**：
1. **顺序依赖**：budget gates (R0/R0.5/R1/R2) 顺序错位（如 R0.5 在 R0 之前）→ 终端后还在 await；rollup guard 在 verdict block 顶部顺序错位（如放在 R5.5 之后）→ 漏 escalate。
2. **rollup guard 重复**：3 处相同 5 行代码，未来加 1 类 rollup 规则（如 R5.5 "RollupSynthEligible at retry limit"）必须改 3 处 + 加 3 行；漏一处 → 静默死循环。
3. **normalize 与决策混排**：5 行 default 兜底写在函数体顶部；reader 第一眼要"跳过 5 行才能看到 R0"，理解成本高。

### 2.3 目标行为（3 个命名子决策代数化）

1. **3 个命名子决策**（`checkBudget` / `checkRollupGuard` / `checkVerdictDirection`）— 各自单一职责、命名清晰、单元测试覆盖 fire/fall-through 行为。
2. **`SpawnPolicyEvaluator` 主函数体 50+→7-8 行**：3 步显式调用 `checkBudget` → `checkRollupGuard` → `checkVerdictDirection`；`normalizeCtx` 抽到独立函数（5 行 default 兜底）。
3. **跨 verdict rollup guard 单一权威位置**：`checkRollupGuard(round, ctx)` 是唯一写 rollup retry exhausted 逻辑的地方；新加 VerdictKind 自动共享同一 guard。
4. **0 行为变化（M5）**：22 现有测试 + byte-equivalent 测试 (build tag `legacy_spawn`) 覆盖 R0-R8 全部 case + 4 rollup guard case + unknown verdict default 字节级 PASS。
5. **可读性 / 可维护性**：新增 spawn rule = 在对应子决策函数加 case；rollup guard 行为变更 = 改 1 处。

### 2.4 0 行为变化验证矩阵

| 验证维度 | 工具 | 期望 |
|----------|------|------|
| 现有 22 测试 PASS | `go test ./internal/layers/orchestration/workmodel/ -run "SpawnPolicyEvaluator\|EvaluateSpawnPolicy" -race -count=1` | 22/22 PASS |
| 字节级 spawn policy 等价 | 新增 `TestSpawnPolicyEvaluatorRefactor_ByteEquivalent_OldVsNew` 比较新旧实现对 22 组合 (R0/R0.5/R1 subcases/R2/R3/R4/R5 subcases/R6 subcases/R7 subcases/R8 + 4 rollup guard + nil round + default verdict) 输出 | 22/22 byte-equal |
| 3 子决策单元测试 | `TestCheckBudget_*` (6 case) + `TestCheckRollupGuard_*` (4 case) + `TestCheckVerdictDirection_*` (5 case) | 15/15 PASS |
| 子决策顺序锁定 | 新增 `TestSpawnPolicyEvaluator_SubDecisionOrder` 验证 3 子决策按预期顺序被调 | 1/1 PASS |

### 2.5 重构总图（5 节点，M5 落点）

| 阶段 | 范围 | 行为变化 | 本 change 落点 |
|------|------|----------|-----------------|
| M1 | Observe go-struct 化 | 0 行为变化 | ✅ S7_archived (DM-20260705-003) |
| M2 | Plan go-struct 化（kernel 复用） | 0 行为变化 | ✅ S7_archived (DM-20260705-004) |
| M4 | Verify 决策表化 | 0 行为变化 | ✅ S7_archived (DM-20260705-005) |
| **M5** | **SpawnDecision 3 子决策代数化（R0-R8）** | **0 行为变化** | **本 change** |
| M3 | Strategy 抽象注入 WorkItemExecContext | 行为增量（PlanKind 路由恢复） | 最后做 (`d7-mups-strategy-injection`) |

**为什么 M5 现在做**：M1+M2+M4 kernel 已稳定；M3 行为增量最大风险，最后做；M5 是 0 行为变化局部子决策代数化，并行可行。**M5 不复活 ChannelRouter 4 文件**（v1 死代码，DM-20260626-009 已 decommissioned）。

## 3. 非目标

- 修改 `SpawnPolicy` 6 态枚举（保持不变）
- 修改 `WorkItemPipelineRound` / `TreeEvalContext` 2 struct 字段
- 修改 `EvaluateSpawnPolicy` 写 `round.SpawnPolicy/SpawnRationale/RollupSynthRequested` 行为
- 修改 `spawnRationale` 6 case 文案
- 修改 `applicableDeliverableSchema` / `deliverableContinuationRequired` / `deliverableInlineWouldExhaust` 4 个 helper 行为
- 真实 PlanKind 路由（属 M3）
- 修改 `VerifyDeliverableContract` / `RollupSynthEligible` / `IsExploratoryPlanKind` / `CanDecompose` 4 个依赖
- 任何 Execute / Observe / Plan / Verify 节点改造
- 跨域 LLM 节点（D3 LLMGateway）改造
- 复活 ChannelRouter 4 文件

## 4. 澄清记录

### Q1: 3 个子决策边界如何划分？
**A**: (1) `checkBudget(round, ctx) (SpawnPolicy, bool)` — R0 (RunningChildren) + R0.5 (deliverable complete) + R1 (max depth) + R2 (daily limit) — 4 个早退 budget gates；与 VerdictKind 无关；`fired=true` 时立即返回；(2) `checkRollupGuard(round, ctx) (SpawnPolicy, bool)` — 跨 verdict 共享的 rollup retry exhausted guard（`ctx.RollupRound && ctx.RollupRetries >= ctx.MaxRollupRetries` → `SpawnEscalateHuman` else `SpawnInline`）— 3 处重复 5 行代码统一到 1 处；(3) `checkVerdictDirection(round, ctx) SpawnPolicy` — R3/R4 (Pass) + R5 (Partial) + R6 (Fail) + R7 (Indeterminate) + R8 (default) — 5 case switch on VerdictKind；总是返回 final decision。 — 2026-07-05

### Q2: `normalizeCtx` 5 行 default 兜底要抽吗？
**A**: **是**。`SpawnPolicyEvaluator` 顶部 5 行 `if ctx.MaxDepth <= 0 { ctx.MaxDepth = DefaultMaxDecomposeDepth }` 等等是"配置兜底"，不是"决策"；抽到 `normalizeCtx(ctx TreeEvalContext) TreeEvalContext` 函数（返回 new ctx，因为 ctx 是 value 不可变）。`SpawnPolicyEvaluator` 主函数体调用 `ctx = normalizeCtx(ctx)`，reader 一眼看清"先 normalize 再 3 步决策"。 — 2026-07-05

### Q3: 子决策返回签名为何 `(policy, fired bool)` 而非 (policy, ok bool)？
**A**: 沿用 M4 verify decision-table 命名约定（`Fire` function 返回 bool），命名上 `fired` 表示"该子决策命中并产出决策"，`fired=false` 表示"该子决策未命中 / 应继续下个子决策"。`fired` 强调"决策被触发"语义，比 `ok` 更准确（`ok` 是 Go 惯用的 err check 语义）。 — 2026-07-05

### Q4: `checkVerdictDirection` switch on VerdictKind 不再嵌入 rollup guard 吗？
**A**: **是**。rollup guard 提到 `checkRollupGuard` 子决策（独立于 VerdictKind），主函数先 `checkRollupGuard` 再 `checkVerdictDirection`。这样：(1) 新加 VerdictKind 自动共享 rollup guard，无重复；(2) `checkVerdictDirection` switch 块内不嵌 if-else，逻辑更清晰；(3) 顺序锁定：rollup guard 一定在 verdict direction 之前（防止"verdict direction 决策后被 rollup guard 覆盖"）。 — 2026-07-05

### Q5: `checkVerdictDirection` 内部 5 case 不再拆子函数吗？
**A**: **不**。Pass/Partial/Fail/Indeterminate/default 5 case 直接写在 switch 内，每个 case 1-2 段决策（如 Partial 包含 "rollup guard 已抽走" → 只剩 "exploratory partial / uncertainty high / deliverable continuation" 3 个 if）。如果未来某 case 长到 50+ 行（不可能），再拆子函数。**遵循 YAGNI**：当前 switch 已可读，拆子函数反而增加跳转。 — 2026-07-05

### Q6: 0 行为变化如何保证？
**A**: 3 重保险：(1) 现有 22 测试 0 修改必须 PASS；(2) 新增 byte-equivalent 测试用 `SpawnPolicyEvaluatorLegacy`（保存为旧实现 + build tag `legacy_spawn`）+ 新版对比 22 组合；(3) 新增子决策顺序锁定测试验证 3 子决策按预期顺序被调。 — 2026-07-05

### Q7: `SpawnPolicyEvaluatorLegacy` 是不是死代码？
**A**: **是**，但仅作为测试用 reference 保留在 `_legacy_test.go`（带 build tag `legacy_spawn` 仅 test 编译）；生产代码 `SpawnPolicyEvaluator` 100% 走 3 子决策代数。S5 验收通过后，下个 change (`mups-cleanup-legacy`) 删除 `_legacy_test.go`。 — 2026-07-05

### Q8: `EvaluateSpawnPolicy` / `spawnRationale` / 4 个 helper 要改吗？
**A**: **不**。`EvaluateSpawnPolicy` (line 145) 调 `SpawnPolicyEvaluator` 后填 `round.SpawnPolicy/SpawnRationale/RollupSynthRequested` 行为不变；`spawnRationale` (line 155) 6 case 文案不变；4 helper (line 225-265) 行为不变。**M5 改动面 = `SpawnPolicyEvaluator` 主函数体 50+→7-8 行 + 3 子决策 + 1 normalizeCtx helper + 1 byte-equivalent `_legacy_test.go` + 1 sub-decision test file**。 — 2026-07-05

## 5. L1-L5 映射

| 层级 | 资产 ID | 名称 | 状态 |
|------|---------|------|------|
| L1 | d7-orchestration | Spawn 节点 | 已有 |
| L2 | L2-ORCH-MUPS-SPAWN | 3 子决策代数化 | 改造 |
| L3-BE | D7-S15 | Spawn Node (MUPS v4.3 Phase 5) | 改造 |
| L4-BE | **D7-S15-A102** | **`spawn_decision_algebra` kernel（3 sub-decision + normalizeCtx + sub-decision order）** | **新增** |
| L4-BE | **D7-S15-A102** | **`SpawnPolicyEvaluator` 主函数 50+→7-8 行（3 步显式调用 checkBudget → checkRollupGuard → checkVerdictDirection）** | **新增** |
| L5 | **L5-MUPS-SDA-01** | **`checkBudget(round, ctx) (SpawnPolicy, bool)` 6 case (R0/R0.5/R1 max-depth w/ cont/R1 max-depth w/ exhaust/R1 max-depth no-schema/R2/fall-through) + 单测** | **草拟 P0** |
| L5 | **L5-MUPS-SDA-02** | **`checkRollupGuard(round, ctx) (SpawnPolicy, bool)` 4 case (Partial+at-limit/Fail+at-limit/Indeterminate+at-limit/fall-through) + 单测** | **草拟 P0** |
| L5 | **L5-MUPS-SDA-03** | **`checkVerdictDirection(round, ctx) SpawnPolicy` 5 case (Pass/R3+R4 + Partial/R5 + Fail/R6 + Indeterminate/R7 + default R8) + 单测** | **草拟 P0** |
| L5 | **L5-MUPS-SDA-04** | **`normalizeCtx(ctx TreeEvalContext) TreeEvalContext` 5 行 default 兜底 + 单测** | **草拟 P1** |
| L5 | **L5-MUPS-SDA-05** | **`SpawnPolicyEvaluator` 重构后 22 组合字节级等价旧实现（build tag `legacy_spawn`）** | **草拟 P0** |
| L5 | **L5-MUPS-SDA-06** | **现有 22 测试 0 修改 PASS** | **草拟 P0** |
| L5 | **L5-MUPS-SDA-07** | **3 子决策顺序锁定测试** | **草拟 P1** |

## 6. 验收标准

- **P0**：`go vet ./...` + `go test ./internal/layers/orchestration/workmodel/... -race -count=1` 全 PASS（22 现有测试 + 15 新子决策单测 + 1 byte-equivalent 测试覆盖 22 组合 + 1 顺序锁定测试）。
- **P0**：3 子决策命名清晰（`checkBudget` / `checkRollupGuard` / `checkVerdictDirection`），各自单一职责，单元测试覆盖 fire/fall-through。
- **P0**：`SpawnPolicyEvaluator` 主函数体 ≤ 10 行（含 nil round 兜底 + normalizeCtx + 3 步显式调用）。
- **P0**：rollup retry exhausted guard 逻辑 1 处实现（`checkRollupGuard`），不再 3 处重复。
- **P0**：22 组合字节级等价旧实现（含 nil round + 4 rollup guard + R8 default）。
- **P1**：`t-registry.md` D7-S15-A102 注册 7 T 点（IMPLEMENTED 304→311，P0 260→266）。
- **P1**：`specs/d7-orchestration/spec.md` §D7-S15 delta 新增 `spawn_decision_algebra` Requirement。
- **P1**：`CHANGELOG.md` d7-orchestration 追加 M5 行。
- **P1**：`openspec/specs/d7-orchestration/mups-5node-refactor-roadmap.md` 标记 M5 完成。

## 7. 规划状态

- [x] S1 `demand.md`（本文）
- [x] S2 `proposal.md` + `.openspec.yaml`
- [x] S3 `design.md` + `tasks.md` + `specs/d7-orchestration/spawn-decision-algebra.md` delta
- [ ] S4 实现（spawn_decision_algebra.go + 3 子决策 + 22 byte-equivalent + 1 顺序锁定）
- [ ] S5 验收（7 L5 + 22 现有 + 0 行为变化）
- [ ] S6-交付（PR squash → master）
- [ ] S6-归档（`archive/2026-07-05-d7-spawn-decision-algebra/`）
