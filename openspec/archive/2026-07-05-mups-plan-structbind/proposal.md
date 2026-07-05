# Proposal: MUPS Go-struct-driven I/O contract (M2 Plan)

**Change ID:** `mups-plan-structbind`  
**Demand:** DM-20260705-004
**Status:** Archived (s7_archived)

## Why

DM-20260705-003 (M1) 已落地 `prompttags/structbind.go` 反射 kernel 并迁移 Observe 节点。Plan 节点 `StrategicPlanInput` 仍走 35+ 行手工 `fields := map[TagName]any{...}` 拼接，与 Observe 节点存在**完全对称的三处描述漂移风险**。M1 的 Go-struct-driven 模式已证明能消除 silent drift，**Plan 节点必须用相同模式迁移**。

M2 独有挑战：`StrategicPlanInput.Budget workmodel.DivergenceBudget` 是嵌套 struct（9 字段），而 `PlanUserFrame` 要求 16 字段平铺。M1 走"input 转换 + 平 struct 反射"模式；M2 沿用，**kernel 零代码增量**。

## What

| Capability | Description |
|------------|-------------|
| **D7-S5-A100** | `StrategicPlanFrame` 16 字段 + `pt:"..."` struct tag（与 `PlanUserFrame` 1:1） |
| **D7-S5-A100** | `buildStrategicPlanFrame(in StrategicPlanInput) StrategicPlanFrame` 转换函数（含 Budget 嵌套展平） |
| **D7-S5-A100** | `buildStrategicPlanUserPrompt` 35+ 行 → 1 行 `prompttags.BuildLineFrameFromStruct("plan_user", frame)` |
| **D7-S5-A100** | `init()` 调 `MustRegisterFrame[StrategicPlanFrame]("plan_user")` |
| **i18n** | 补 11 条 `plan.input.*.when_use` 翻译（en + zh 各 11 条） |
| **kernel 零代码增量** | `structbind.go` / `linefield.go` / `semantics.go` 全部不动 |

## Scope

- **M2（本次落地）**：`StrategicPlanFrame` + `buildStrategicPlanFrame` + 1 行反射调用 + 11 条 i18n 翻译 + 0 行为变化验证
- **M3-M5（follow-on）**：仅在 `design.md` §6 列出 follow-on 计划

## Out of scope

- 修改 `workmodel.DivergenceBudget` 字段
- 修改 `PlanUserFrame` 字段顺序
- 修改 `applyBudgetCap` / `applySingleModeUncertaintyGate` 业务逻辑
- 复活 ChannelRouter 4 个 channel 文件
- 跨域 LLM 节点改造（D3 LLMGateway、D4 Delegate）
- M3 Strategy 抽象、M4 Verify 表驱动、M5 SpawnDecision 代数化

## Architecture decision

| 方案 | 优点 | 缺点 |
|------|------|------|
| **A: 转换 struct 模式（采纳）** | kernel 零代码增量；D7 域内可控；与 M1 模式对称；StrategicPlanInput 保持 domain 语义 | 多一个 `StrategicPlanFrame` 类型（16 字段平铺） |
| B: kernel 扩展 `pt:",flatten"` 嵌套 flag | 反射时自动拍平嵌套 struct 字段 | 改 kernel 破坏 M1 0 行为变化承诺；嵌套深度/类型需更多 panic 校验 |
| C: 保持手工 map | 零改动 | 漂移风险持续累积；与 Observe 节点模式不对称 |

**选 A**。理由：M1 已落地对称模式，**复用 > 重设计**；`StrategicPlanInput` 是 domain 概念（编排域用），`StrategicPlanFrame` 是 LLM 视图（共享域用），二者语义不同，不应合并；kernel 改动会触发 M1 重测。

## Risks & Mitigations

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| Budget 字段平铺顺序漂移 | Med | High | `buildStrategicPlanFrame` 是唯一平铺点；golden snapshot 4 组合覆盖；与现状手工 map 字段顺序逐字段对照 |
| i18n 翻译覆盖度不足 | Med | Med | MustRegisterFrame init 校验 16 字段都有翻译条目；缺则 panic |
| 条件字段跳过逻辑漂移（Budget=0 时不输出 9 行） | Med | Med | Budget.MaxChildren > 0 守卫保留（与现状一致）；Budget 字段全部 omitempty |
| 与 PR #403 (M1) merge 顺序冲突 | Low | Low | M2 在 M1 merge 后 rebase；M2 PR 标注 "depends on #403" |
| StrategicPlanInput 字段新增未同步到 Frame | Low | Med | 字段数 == FrameSpec 长度 == 16 由 init panic 强制；新增字段需同时改 3 处 |

## Success Metrics

- 0 行为变化（golden diff = 0 + E2E 全 PASS）
- `buildStrategicPlanUserPrompt` 函数体 ≤ 5 行
- 16 字段 struct 与 16 字段 FrameSpec 与 ≥ 16 个 i18n 条目三方一致
- `structbind.go` / `linefield.go` / `semantics.go` 改动 = 0 行（kernel 零代码增量）
- 新增 tag 工作量从 5 处 → 1 处

## Follow-on changes (M3-M5)

参见 `design.md` §6 与 `openspec/specs/d7-orchestration/mups-5node-refactor-roadmap.md`：

- **M3** `d7-mups-strategy-injection` — Strategy 抽象注入 WorkItemExecContext（**行为增量**，最后做）
- **M4** `mups-verify-table-driven` — 4 VerdictKind × N trigger 表驱动
- **M5** `d7-spawn-decision-algebra` — R0-R8 嵌套 if 拆为 3 个命名子决策

M2 通过后，依次启动 M4/M5（可并行），M3 最后做。
