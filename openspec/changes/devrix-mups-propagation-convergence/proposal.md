# Proposal: MUPS+WorkTree 传播闭环修复

- **Demand ID:** DM-20260701-001
- **Change ID:** devrix-mups-propagation-convergence

## 1. 目标

让 MUPS+WorkTree 的"发散-收敛"闭环在三个维度成立：

1. **收敛可见且一致**：不确定性数值能反映收敛（RH-MUPS-01/02）。
2. **收敛有终止保证**：rollup 有重试上限与升级出口，全失败不被洗成成功（RH-MUPS-03/04）。
3. **发散范围与验收标准对 LLM 可见、可反馈**：发散提案知道树预算与父 scope，验收 bar 在生产侧可见、失败可回灌（RH-MUPS-07/08/09/10/11）。

## 2. Capabilities 清单

| Capability | 覆盖 Finding | 说明 |
|-----------|-------------|------|
| `UncertaintyReconcile` | RH-MUPS-01, 02 | 单一 reconcile 函数合并 pipeline/reevaluate 两路写入，移除盲目 ratchet，收敛允许下降 |
| `RollupTerminationGuard` | RH-MUPS-03 | rollup 重试计数器 + 上限 → `SpawnEscalateHuman`/显式 Failed |
| `RollupOutcomeAggregation` | RH-MUPS-04 | rollup gate/verify 感知子裁决成功率，全失败禁止 Completed |
| `DivergenceBudgetVisibility` | RH-MUPS-07, 08 | Plan prompt 注入 depth/配额/上限（统一配置），超额结构化 reject |
| `ScopeSubdivisionContract` | RH-MUPS-05, 09 | child scope 必须是父真子集 + 覆盖校验 + prompt 指引 |
| `AcceptanceCriteriaVisibility` | RH-MUPS-10, 12 | execute prompt 注入可读验收要点；verify 失败 Reason 回灌；denylist 移入 i18n |
| `SchemaMonotonicNarrowing` | RH-MUPS-11 | LLM 只能收紧不能放宽 inferred schema |
| `ChildUncertaintyBubble` | RH-MUPS-06 | 子不确定性以 ObsUncertainty 上浮 |

## 3. 分阶段交付

### Phase P0 — 收敛正确性（阻断性）

> 目标：闭环不再有死路、不再洗白失败、收敛数值可信。

- **C1 `UncertaintyReconcile`（RH-MUPS-01/02）**：新增 `workmodel.ReconcileUncertainty(prevStored, pipelineRound, childStats)`，作为 `item.Uncertainty` 的唯一写入入口；移除 `item_pipeline.go:366-368` 的裸 max ratchet，改为"收敛允许下降、单轮乐观需经 historical 抑制"。
- **C2 `RollupTerminationGuard`（RH-MUPS-03）**：`WorkItemPipelineRound` 增 `RollupRetries`；`SpawnPolicyEvaluator` 在 `ctx.RollupRound && RollupRetries >= MaxRollupRetries` 时返回 `SpawnEscalateHuman`；`session_turn_loop` 在 break 前检查未收敛 rollup parent。
- **C3 `RollupOutcomeAggregation`（RH-MUPS-04）**：`rollup_gate`/`rollup_verify` 读取 `ChildOutcomeStats`，`Failed==Total` 时 rollup verdict 不得 Pass（标 Partial/Failed + 在 deliverable 注明失败子集）。
- **C4 `AcceptanceCriteriaVisibility`（RH-MUPS-10）**：扩展 `AppendDeliverableExecuteHint` 注入"人类可读验收要点"（来源 i18n/format_hints）；verify Partial 时把 `Reason` 通过 round → 下一轮 execute prompt 回灌。

### Phase P1 — 发散可控性

- **D1 `DivergenceBudgetVisibility`（RH-MUPS-07/08）**：`buildStrategicPlanUserPrompt` 注入 `depth/max_depth/remaining_children/remaining_daily/parent_scope_in`；发散上限常量集中到单一配置并同时喂给 prompt 与 cap；LLM 超额时返回结构化 reject 供下一轮自纠。
- **D2 `SchemaMonotonicNarrowing`（RH-MUPS-11）**：`item_pipeline.go` schema 选择改为 `narrowest(inferred, strategic)`——LLM 只能把 `not_applicable` 收紧为具体 schema，不能反向放宽。
- **C5 denylist 去硬编码（RH-MUPS-12）**：`rollupPlanningDenylist` 移入 i18n / `format_hints`，并在 `t-registry` 登记。

### Phase P2 — 发散有效性（设计增强）

- **E1 `ScopeSubdivisionContract`（RH-MUPS-05/09）**：`DefaultChildDownlink` 不再无脑继承父全量 scope；`mapRawChildSpecs` 增 scope 子集 + 覆盖校验；prompt 增"真子集/不重叠/全覆盖"指引。
- **E2 `ChildUncertaintyBubble`（RH-MUPS-06）**：高不确定性子 bubble 以 `ObsUncertainty`（而非弱 ObsFact）上浮，驱动父发散判断。

## 4. 成功标准

- P0 全绿：新增 L5-MUPS-01/02/03/04/10 测试通过；rollup 注入故障注入测试不再静默 break。
- P1：发散提案 prompt 快照测试含全部预算字段；schema 单调收紧契约测试通过。
- 无 D7→D3 直依赖新增；无新增战术硬编码（denylist/上限均来自配置/i18n）。
- 单 PR ≤ 400 行：按 Phase 拆 3+ PR。

## 5. 风险

| 风险 | 缓解 |
|------|------|
| 移除 ratchet 导致不确定性抖动 | reconcile 内保留 historical 抑制项，单轮乐观不可独占下降 |
| rollup 升级人工改变现有 session 终止行为 | MaxRollupRetries 默认值取较大值（如 3），仅替换"静默 break"为显式结局 |
| prompt 注入预算字段增加 token | 字段为短整数 + 单行 scope，预算可控；走 D2 Prepare 压缩 |
| schema 单调收紧误伤合法放宽场景 | 仅对 directive 命中 review 关键词的强信号生效 |
