# Proposal: MUPS Verify 节点决策表化 (M4)

**Change ID:** `mups-verify-table-driven`  
**Demand:** DM-20260705-005
**Status:** S3_Design

## Why

MUPS Verify 节点当前由 `sessionorchestrator/item_verify.go` 三个手工 `verify*` 函数实现：嵌套 if/switch 把 4 种 `VerdictKind` × 12 类 trigger（nil / execute error / max_iters+tool / SideEffectStatus / user-gate / scope-only / deliverable contract / rollup child stats / empty summary / rollup contract）的决策逻辑散落在函数体中。trigger 顺序、置信度（10+ 魔数 0.55/0.6/.../0.95）、Reason 文案、SourceID 全部隐式，**无单一权威位置**。新增 trigger（如未来 LLM Verifier 注入、plan kind 升级、parser failure）必须修改 3 个函数；trigger 顺序错位（如 verifyArtifact 之后 user-gate 覆盖 max_iters 决策）→ 静默错误。

**trigger × verdict-template 决策表** 模式把 12 trigger 抽取为命名 `detectXxx` 函数，verify 函数变成"构建表 → 应用表"两步。trigger 顺序、置信度、Reason 文案集中在表里，新增 trigger = 1 个新函数 + 1 行表注册。

## What

| Capability | Description |
|------------|-------------|
| **D7-S10-A101** | `sessionorchestrator/verify_decision_table.go` (NEW, ~180 行) — `verifyContext` 不可变 ctx + `VerdictTrigger` struct (Name/Fire/Template) + `VerdictTemplate` struct (Kind/Confidence/Reason/IndeterminateReason) + 12 `detectXxx` 函数 + `applyDecisionTable` 顺序遍历 |
| **D7-S10-A101** | `sessionorchestrator/verify_decision_table_test.go` (NEW, ~180 行) — 12 detector 单元测试 + 3 verify 字节级 golden + detector 顺序断言 |
| **D7-S10-A101** | `sessionorchestrator/{item_verify,rollup_verify}.go` (MOD) — 3 verify 函数体替换为"建表 + applyDecisionTable"，行数减半 |
| **D7-S10-A101** | `sessionorchestrator/verify_legacy_test.go` (NEW, ~80 行) — `verifyArtifactLegacy` + `verifyArtifactForWorkItemLegacy` + `verifyRollupArtifactLegacy` 保留旧实现，仅供 byte-equivalent 测试；下个 change 删除 |

## Scope

- **M4（本次落地）**：`verify_decision_table.go` kernel + 3 verify 函数表驱动化 + 0 行为变化验证
- **M5（follow-on，`d7-spawn-decision-algebra`）**：SpawnDecision R0-R8 嵌套 if 拆为 checkBudget/checkDirection/checkEscalation 3 个命名子决策
- **M3（follow-on，`d7-mups-strategy-injection`）**：Strategy 抽象注入 WorkItemExecContext（行为增量，最后做）

## Out of scope

- 复活 ChannelRouter 4 个 channel 文件（v1 死代码，DM-20260626-009 已 decommissioned）
- 修改 `workmodel.Verdict` 4 字段 / `types.VerdictKind` 4 态枚举 / `VerdictToExitReason` 映射
- 真实 LLM Verifier 注入（仅留 `ItemPipelineDeps.Verifier` 现有接口；不调）
- 触发 PlanKind 路由（属 M3）
- 触发 `SpawnPolicy` 升级路径（属 M5）
- 任何 Execute / Observe / Plan 节点改造
- 跨域 LLM 节点（D3 LLMGateway）改造

## Architecture decision

| 方案 | 优点 | 缺点 |
|------|------|------|
| **A: trigger × verdict-template 决策表（采纳）** | 12 trigger 命名清晰；置信度集中在表里；新增 trigger = 1 处改动；trigger 顺序显式声明；byte-equivalent 测试易写 | 12 detector 函数 + 表遍历引入 ~150 行间接层 |
| B: 维持现状（手工 if/switch） | 零新代码 | trigger 散落 3 函数；新增 trigger 改 3 处；DM-20260701-001 RH-MUPS-04 rollup 缺 all-failed gate 修复已暴露该问题 |
| C: strategy/chain-of-responsibility 模式 | 更灵活，trigger 可运行时注册 | 当前 12 trigger 全部硬编码；运行时注册反而引入隐性配置；不解决"trigger 顺序"问题 |

**选 A**。理由：用户最在意"重复链路 / 二义性 / 散落魔数"；方案 B 的散落风险已被 DM-20260701-001 RH-MUPS-04 暴露（rollup 缺 `Failed==Total → Completed` 拦截 → 全失败被当完成）；方案 C 解决 50% 问题但引入运行时配置的隐性二义性。

## Risks & Mitigations

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| 行为漂移 | Med | High | 13 现有测试 0 修改 + 7 新 byte-equivalent 测试 (旧版 + 新版对比 6 artifact 组合) + 11 detector 顺序锁定测试 |
| Trigger 顺序错位 | Med | High | `applyDecisionTable` 顺序遍历，第一个 fired trigger 返回；新加 trigger 必须显式声明位置；order test 11 断言 |
| 置信度漂移 | Low | Med | 12 trigger 置信度集中在 `VerdictTemplate.Confidence`；改一个数字必须改 testdata 期望 |
| `verifyArtifactLegacy` 死代码 | Low | Low | 仅在 `_legacy_test.go` 保留 + build tag `legacy_verify` 仅 test 编译；下个 change (`mups-cleanup-legacy`) 删除 |
| M3 / M5 follow-on 破坏 M4 | Low | Med | M3/M5 是 0 行为变化（M5）或行为增量（M3 最后做）；M4 决策表 API 稳定 trigger 顺序，不会被 M5 spawn 决策破坏 |

## Success Metrics

- 0 行为变化（13 现有 + 7 byte-equivalent 测试）
- 12 detector 命名清晰 + 单测覆盖
- 3 verify 函数体 ≤ 30 行（≤ 现状 50%）
- trigger 顺序、置信度、Reason 文案集中在表
- 新增 trigger 工作量从 3 处 → 1 处（detectXxx + 1 行表注册）

## Follow-on changes (M3 + M5)

参见 `design.md` §6 与 `openspec/specs/d7-orchestration/mups-5node-refactor-roadmap.md`（M1 已写入）。M4 通过后，启动 M5（0 行为变化），M3 最后做（行为增量）。
