# Tasks: MUPS+WorkTree 传播闭环修复

- **Demand ID:** DM-20260701-001
- **Change ID:** devrix-mups-propagation-convergence

> 任务按 Phase 分组；每任务标注关联 OpenSpec Requirement 与优先级。单 PR ≤ 400 行，建议按 Phase 切 PR。

## Phase P0 — 收敛正确性（阻断性）

| 任务 | 描述 | Requirement | 优先级 |
|------|------|-------------|--------|
| T-P0-1 | 新增 `workmodel.ReconcileUncertainty` 纯函数 + 表驱动单测 | D7-S1-A88 | P0 |
| T-P0-2 | `item_pipeline.go` 移除裸 max ratchet，改调 `ReconcileUncertainty` | D7-S1-A88 | P0 |
| T-P0-3 | `reevaluateParentAfterChild` 统一走 `ReconcileUncertainty` + `SetUncertainty` | D7-S1-A88 | P0 |
| T-P0-4 | `WorkItemPipelineRound` 增 `RollupRetries`；`TreeEvalContext` 透传 | D7-S15-A89 | P0 |
| T-P0-5 | `SpawnPolicyEvaluator` rollup 分支加 `MaxRollupRetries → SpawnEscalateHuman` | D7-S15-A89 | P0 |
| T-P0-6 | `session_turn_loop` break 前检查未收敛 rollup parent，emit 显式结局 | D7-S15-A89 | P0 |
| T-P0-7 | rollup 故障注入测试：verify 恒 fail → 达上限 escalate，不超 loop | D7-S15-A89 | P0 |
| T-P0-8 | `rollup_gate`/`rollup_verify` 读 `ChildOutcomeStats`，`Failed==Total` 禁 Completed | D7-S15-A90 | P0 |
| T-P0-9 | `buildRollupDirective` 增"失败子集"区段（不洗白） | D7-S15-A90 | P0 |
| T-P0-10 | `AppendDeliverableExecuteHint` 注入 i18n 可读验收要点 | D7-S9-A91 | P0 |
| T-P0-11 | `WorkItemExecContext` 增 `PriorVerifyReason`；inline 重试回灌 verdict.Reason | D7-S9-A91 | P0 |
| T-P0-12 | execute prompt 快照测试：含验收要点 + 回灌 reason | D7-S9-A91 | P0 |

## Phase P1 — 发散可控性 + 去硬编码

| 任务 | 描述 | Requirement | 优先级 |
|------|------|-------------|--------|
| T-P1-1 | 发散上限常量集中到 `workmodel` 单一来源（children/iters） | D7-S5-A92 | P1 |
| T-P1-2 | `buildStrategicPlanUserPrompt` 注入 depth/max_depth/remaining_children/remaining_daily/parent_scope_in | D7-S5-A92 | P1 |
| T-P1-3 | LLM 超额提案 → 结构化 reject（含 max_allowed），下一轮自纠 | D7-S5-A92 | P1 |
| T-P1-4 | Plan prompt 快照测试含全部预算字段 | D7-S5-A92 | P1 |
| T-P1-5 | schema 选择改 `NarrowestSchema(inferred, strategic)`，禁放宽 | D7-S9-A93 | P1 |
| T-P1-6 | `rollupPlanningDenylist` 迁 i18n/`format_hints` + t-registry 登记 | D7-S9-A91 | P1 |

## Phase P2 — 发散有效性（设计增强）

| 任务 | 描述 | Requirement | 优先级 |
|------|------|-------------|--------|
| T-P2-1 | `DefaultChildDownlink` 移除"无脑继承父全量 scope" | D7-S16-A94 | P2 |
| T-P2-2 | `ValidateChildScopes(parent, children)` 真子集 + 覆盖校验 + prompt 指引 | D7-S16-A94 | P2 |
| T-P2-3 | 高不确定性子 bubble 以 `ObsUncertainty` 上浮 | D7-S8-A95 | P2 |

## 审计映射（Finding → Requirement → Task）

| Finding | 严重度 | Requirement | Task |
|---------|--------|-------------|------|
| RH-MUPS-01 | Critical | D7-S1-A88 | T-P0-1/2 |
| RH-MUPS-02 | Critical | D7-S1-A88 | T-P0-1/3 |
| RH-MUPS-03 | Warning+ | D7-S15-A89 | T-P0-4/5/6/7 |
| RH-MUPS-04 | Warning | D7-S15-A90 | T-P0-8/9 |
| RH-MUPS-05 | Info | D7-S16-A94 | T-P2-1/2 |
| RH-MUPS-06 | Info | D7-S8-A95 | T-P2-3 |
| RH-MUPS-07 | Critical | D7-S5-A92 | T-P1-1/2/3/4 |
| RH-MUPS-08 | Warning | D7-S5-A92 | T-P1-1/2 |
| RH-MUPS-09 | Warning | D7-S16-A94 | T-P2-2 |
| RH-MUPS-10 | Critical | D7-S9-A91 | T-P0-10/11/12 |
| RH-MUPS-11 | Warning | D7-S9-A93 | T-P1-5 |
| RH-MUPS-12 | Info | D7-S9-A91 | T-P1-6 |

## 完成定义（DoD）

- [ ] P0 全部任务完成，新增 L5-MUPS-01/02/03/04/10 测试通过
- [ ] `go test -race ./internal/layers/orchestration/...` 全绿
- [ ] 无新增 D7→D3 直依赖；无新增战术硬编码（denylist/上限均来自配置/i18n）
- [ ] 新增 T 点登记 `openspec/specs/d7-orchestration/t-registry.md`
- [ ] 验收 ACCEPTED 后、合入前同步 `openspec/specs/d7-orchestration/spec.md`
