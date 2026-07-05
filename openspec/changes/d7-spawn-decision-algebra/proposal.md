# Proposal: MUPS Spawn 决策代数化 (M5)

**Change ID:** `d7-spawn-decision-algebra`  
**Demand:** DM-20260705-006
**Status:** S3_Design

## Why

MUPS Decide 节点的 `SpawnPolicyEvaluator(round, ctx) SpawnPolicy`（`workmodel/spawn_policy.go:19`）当前用 50+ 行嵌套 `if` + `switch round.VerdictKind` 把 R0-R8 共 9 条规则散落在一个函数体里。R0/R0.5/R1/R2 是 budget/gating 早退决策，R5/R6/R7 verdict 块内嵌套 rollup retry exhausted guard（RH-MUPS-03 DM-20260701-001），R3/R4/R8 是 verdict-kind 路由。规则之间无显式边界；rollup guard 在 3 个 verdict 块里重复 3 次共 15 行；normalize 5 行 default 兜底写在函数体顶部，与决策混排。**3 个命名子决策**（`checkBudget` / `checkRollupGuard` / `checkVerdictDirection`）模式把 R0-R8 拆为 3 段单一职责函数 + 1 个 `normalizeCtx` helper，主函数 50+→7-8 行"checkBudget → checkRollupGuard → checkVerdictDirection" 3 步显式声明。**新加 spawn rule = 在对应子决策函数加 case**；rollup guard 行为变更 = 改 1 处（不再 3 处重复）。

## What

| Capability | Description |
|------------|-------------|
| **D7-S15-A102** | `workmodel/spawn_decision_algebra.go` (NEW, ~120 行) — `normalizeCtx` helper + `checkBudget(round, ctx) (SpawnPolicy, bool)` + `checkRollupGuard(round, ctx) (SpawnPolicy, bool)` + `checkVerdictDirection(round, ctx) SpawnPolicy` 4 函数 |
| **D7-S15-A102** | `workmodel/spawn_decision_algebra_test.go` (NEW, ~280 行) — 3 子决策单元测试（6+4+5=15 case）+ 1 子决策顺序锁定测试 + 1 normalizeCtx 单测 |
| **D7-S15-A102** | `workmodel/spawn_policy_legacy_test.go` (NEW, ~250 行, build tag `legacy_spawn`) — `SpawnPolicyEvaluatorLegacy` 保留旧 50+ 行实现，仅供 22 组合 byte-equivalent 测试；下个 change (`mups-cleanup-legacy`) 删除 |
| **D7-S15-A102** | `workmodel/spawn_policy.go` (MOD) — `SpawnPolicyEvaluator` 主函数体 50+→7-8 行（nil round 兜底 + `ctx = normalizeCtx(ctx)` + `if p, fired := checkBudget(...); fired { return p }` + `if p, fired := checkRollupGuard(...); fired { return p }` + `return checkVerdictDirection(...)`） |

## Scope

- **M5（本次落地）**：`spawn_decision_algebra.go` kernel + 3 子决策 + normalizeCtx helper + 0 行为变化验证（22 现有测试 0 修改 + 15 子决策单测 + 1 顺序锁定 + 1 byte-equivalent 22 组合）
- **M3（follow-on，`d7-mups-strategy-injection`）**：Strategy 抽象注入 WorkItemExecContext（行为增量，最后做）
- **`mups-cleanup-legacy`**（下下个 change）：删除 `spawn_policy_legacy_test.go` + `SpawnPolicyEvaluatorLegacy` 死代码

## Out of scope

- 复活 ChannelRouter 4 个 channel 文件（v1 死代码，DM-20260626-009 已 decommissioned）
- 修改 `SpawnPolicy` 6 态枚举
- 修改 `WorkItemPipelineRound` / `TreeEvalContext` 2 struct 字段
- 修改 `EvaluateSpawnPolicy` 写 `round.SpawnPolicy/SpawnRationale/RollupSynthRequested` 行为
- 修改 `spawnRationale` 6 case 文案
- 修改 `applicableDeliverableSchema` / `deliverableContinuationRequired` / `deliverableInlineWouldExhaust` / `IsRegisteredDeliverableSchema` / `IsDeliverableInlineBudgetExhaustedFromCtx` 5 个 helper 行为
- 真实 PlanKind 路由（属 M3）
- 修改 `spawnForDeliverableContinuation` / `RollupSynthEligible` / `IsExploratoryPlanKind` / `CanDecompose` 4 个依赖
- 任何 Execute / Observe / Plan / Verify 节点改造
- 跨域 LLM 节点（D3 LLMGateway）改造

## Architecture decision

| 方案 | 优点 | 缺点 |
|------|------|------|
| **A: 3 个命名子决策代数化（采纳）** | 3 段单一职责；rollup guard 单一权威位置（不再 3 处重复）；主函数 50+→7-8 行；新加 rule = 在对应子决策加 case | 3 函数 + normalizeCtx 引入 ~120 行间接层 |
| B: 维持现状（手工 if/switch 链） | 零新代码 | rollup guard 3 处重复 15 行；规则散落；DM-20260701-001 RH-MUPS-03 修复时已暴露该问题（3 处对称修改 + 4 测试） |
| C: strategy/chain-of-responsibility 模式 | 更灵活，子决策可运行时注册 | 当前 9 规则全部硬编码；运行时注册反而引入隐性配置；不解决"rollup guard 重复"问题 |

**选 A**。理由：用户最在意"重复链路 / 二义性 / 散落魔数"；方案 B 的散落 + 重复风险已被 DM-20260701-001 RH-MUPS-03 暴露（rollup retry exhausted 修复需要 3 处对称修改 + 4 测试）；方案 C 解决 50% 问题但引入运行时配置的隐性二义性。**M4 verify decision-table 已采纳类似模式**（12 trigger 决策表），M5 与 M4 风格保持一致：命名子决策 + 显式顺序 + 单一权威位置。

## Risks & Mitigations

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| 行为漂移 | Med | High | 22 现有测试 0 修改 + 1 byte-equivalent 测试（22 组合）+ 1 顺序锁定测试 |
| rollup guard 提取后漏 case | Med | High | 3 子决策顺序锁定测试 11 断言；`checkRollupGuard` 单测覆盖 4 case (Partial+at-limit/Fail+at-limit/Indeterminate+at-limit/Pass+at-limit fall-through) |
| normalizeCtx 抽离引入 ctx 复用风险 | Low | Med | `normalizeCtx` 返回新 ctx（value 不可变），不改入参 ctx；单测 5 字段覆盖 |
| `SpawnPolicyEvaluatorLegacy` 死代码 | Low | Low | 仅在 `_legacy_test.go` 保留 + build tag `legacy_spawn` 仅 test 编译；下个 change (`mups-cleanup-legacy`) 删除 |
| M3 follow-on 破坏 M5 | Low | Med | M3 是行为增量（PlanKind 路由恢复）；M5 子决策 API 稳定接口，不会被 M3 破坏 |

## Success Metrics

- 0 行为变化（22 现有 + 22 byte-equivalent 组合 + 15 子决策单测 + 1 顺序锁定 + 1 normalizeCtx）
- 3 子决策命名清晰 + 单一职责 + 单元测试覆盖
- `SpawnPolicyEvaluator` 主函数体 ≤ 10 行（≤ 现状 20%）
- rollup retry exhausted guard 单一权威位置（不再 3 处重复 15 行）
- 新增 spawn rule 工作量从 1 处函数体 → 1 个子决策函数加 case

## Follow-on changes (M3)

参见 `design.md` §9 与 `openspec/specs/d7-orchestration/mups-5node-refactor-roadmap.md`（M1+M2+M4 已写入）。M5 通过后，启动 M3（行为增量），最后做。
