# Demand: MUPS 上下文与工具决策归 D2 统一负责

- **Demand ID:** DM-20260704-001
- **Change ID:** mups-d2-context-tools-ownership
- **Priority:** P0
- **Domain:** D2 Context Engine + D7 Orchestration（跨域边界）
- **Status:** S2 Clarified
- **Source:** MUPS v4 五/六节点管道落地后 D2↔D7 职责漂移复盘；Filter v2 已实现未接线；D7 仍散落 context/tools 组装

---

## 1. 原始描述

> MUPS 编排（D7）负责 Observe/Plan/Execute/Verify/Learn/Decide 流程与 WorkItem 生命周期；上下文组装、工具过滤、节点级 system prompt、压缩与 token 预算应 **统一由 D2 Context Engine 负责**，而非分散在 D7 `sessionorchestrator`。
>
> 当前问题：Filter v2（PerEmissionClass / PerTaskKind）已在 D2 实现但未接入 Prepare 路径；`toolsForProfile` 硬编码在 D2 materialize 且 D7 仍二次过滤；Observe/Plan 的 phase appendix 写在 D7 proposer；`WithLocatorPhase` 由 D7 设置但 D2 未消费；`toolchannel` 骨架在 D7 未与 D2 ToolRound 集成。
>
> 期望：定义 `MaterializeForMUPS` 契约，D7 仅传 MUPS phase + WorkItem 上下文，D2 返回 `MUPSPreparedContext`（含已过滤 tools + 完整 system prompt），D7 再调 D3 LLM。

---

## 2. 澄清记录

### Q1: D7 是否仍保留 LLM 调用权？
**A:** **是。** D7 继续 InvokeLLM → D3；D2 **禁止** D2→D3（DM-020 不变）。D2 只产出 PreparedContext，不发起 LLM stream。 — 2026-07-04

### Q2: Verify/Learn/Decide 是否走 D2 Materialize？
**A:** **否。** 这三节点为 Go-only 确定性逻辑，不调 LLM，无需 D2 Materialize。Observe/Plan（LLM 路径）与 Execute（每轮 ReAct）才调用 `MaterializeForMUPS`。 — 2026-07-04

### Q3: ToolChannel 路由归 D2 还是 D7？
**A:** **Phase A** 仅接线 Filter v2 + Materialize API；**Phase B** 将 `toolchannel.Router` 终止/probe 逻辑迁入 D2-S18 `ExecuteToolRound`，D7 只传 TaskKind + 剩余 budget。PlanChannel（per-PlanKind 执行策略）仍留 D7 Execute 节点。 — 2026-07-04

### Q4: 现有 Turn 主循环（FastPath RunTurn）是否受影响？
**A:** **渐进迁移。** Phase A 新增 MUPS 专用 API，不破坏现有 `PrepareForTurn`；Phase B 起 MUPS WorkItem 路径切至 `MaterializeForMUPS`；FastPath 后续可复用同一 filter pipeline（out of scope 本 change Phase C 后 follow-up）。 — 2026-07-04

### Q5: D7 能否继续 block `ask_user_question` 等于 pipeline？
**A:** **否（目标态）。** `pipelineBlockedTools` 应变为 D2 的 MUPS-phase 过滤规则（Execute 自动化轮次 deny interactive tools），而非 D7 `filterPipelineTools`。Phase C 删除 D7 侧实现。 — 2026-07-04

---

## 3. 澄清范围

### 3.1 L1–L5 映射

| 层级 | 资产 ID | 名称 | 状态 |
|------|---------|------|------|
| L1 | D2 | Context Engine | 已有 |
| L1 | D7 | Orchestration | 已有 |
| L2 | D2-S15 | PrepareExecutionContext | 扩展 |
| L2 | D2-S18 | EnforceExecutionPolicy / ToolRound | 扩展 |
| L2 | D7-S2 | Session Orchestrator / MUPS Pipeline | 已有 |
| L3-BE | D7-S2-A06 | ItemPipelineRunner（Observe/Plan/Execute） | 改造 |
| L4-BE | D2-S15-A90 | MaterializeForMUPS | **新增** |
| L4-BE | D2-S15-A91 | MUPS Tool Filter Pipeline | **新增** |
| L4-BE | D2-S15-A92 | MUPS Phase Prompt Registry | **新增** |
| L4-BE | D2-S18-A90 | ToolRound + ToolChannel Router | **新增（Phase B）** |
| L5 | L5-D2-MUPS-01 | Observe phase: 无 tools + observation schema appendix | 草拟 |
| L5 | L5-D2-MUPS-02 | Plan phase: strategic plan appendix 来自 D2 | 草拟 |
| L5 | L5-D2-MUPS-03 | Execute phase: Filter v2 全链路 + ToolProfile | 草拟 |
| L5 | L5-D2-MUPS-04 | D7 不 import enforce/tools/filter | 草拟 |
| L5 | L5-D2-MUPS-05 | ToolRound probe termination 在 D2 | 草拟（Phase B） |

### 3.2 根因清单

| ID | 严重度 | 一句话 | 主要代码位 |
|----|--------|--------|-----------|
| RH-D2-MUPS-01 | P0 | Filter v2 实现未接入 Prepare/Materialize | `enforce/tools/filter/*.go` vs `prepare/orchestrator.go` |
| RH-D2-MUPS-02 | P0 | `toolsForProfile` 硬编码工具名，绕过 registry + Filter v2 | `materialize/compressor.go:105` |
| RH-D2-MUPS-03 | P0 | Observe/Plan appendix 在 D7 proposer 拼接 | `llm_observation_proposer.go`, `strategic_plan_proposer.go` |
| RH-D2-MUPS-04 | P1 | `WithLocatorPhase` 设置但 D2 未读 | `item_pipeline.go` vs D2 prepare |
| RH-D2-MUPS-05 | P1 | D7 `filterPipelineTools` 二次过滤 | `workitem_tools.go`, `workitem_executor.go` |
| RH-D2-MUPS-06 | P1 | `toolchannel` 骨架未与 Execute ReAct 集成 | `mups/execute/toolchannel/` |

### 3.3 In Scope / Out of Scope

**In scope**
- `MaterializeForMUPS` API 与类型定义（shared/contracts 或 D2 materialize 包）
- D2 侧 7 步 Tool Filter Pipeline 接线
- MUPS phase prompt registry（observe/plan/execute/rollup_synth）
- D7 Observe/Plan/Execute 调用方迁移
- Phase B：ToolChannel Router → D2-S18
- Lint 边界：`d7_no_tool_filter_import_test`
- OpenSpec design + tasks + T 层登记

**Out of scope**
- 重写 MUPS 五/六节点语义
- D7 PlanChannel（per-PlanKind）执行策略变更
- FastPath Turn 全量切 MaterializeForMUPS（follow-up）
- LLM 战术 prose 变更（仅搬迁 appendix 位置，内容等价迁移至 i18n/format_hints）

---

## 4. 验收口径（L5 Given-When-Then）

### L5-D2-MUPS-01 — Observe 无 tools（P0）
- **GIVEN** `MUPSContextRequest.Phase == observe`
- **WHEN** D7 调用 `MaterializeForMUPS`
- **THEN** `MUPSPreparedContext.Tools` 为空 AND `PhaseAppendix` 含 observation JSON schema

### L5-D2-MUPS-02 — Plan appendix 来自 D2（P0）
- **GIVEN** `Phase == plan` AND locale=zh
- **WHEN** `MaterializeForMUPS` 返回
- **THEN** `SystemPrompt` 含 strategic plan deliverable_contract 维度说明 AND D7 proposer 不再 append 本地常量

### L5-D2-MUPS-03 — Execute Filter v2 全链路（P0）
- **GIVEN** `Phase == execute`, `TaskKind == review`, `ToolProfile == readonly`
- **WHEN** Filter pipeline 运行
- **THEN** 输出 tools ⊆ registry AND Probe 类保留 OpenEnded AND 无 write/bash

### L5-D2-MUPS-04 — D7 边界 lint（P0）
- **GIVEN** `internal/layers/orchestration/sessionorchestrator/` 生产代码
- **WHEN** import 图分析
- **THEN** 不 import `contextengine/enforce/tools/filter`

### L5-D2-MUPS-05 — ToolRound probe 终止（P1，Phase B）
- **GIVEN** Probe tool iter ≥ bound
- **WHEN** D2 `ExecuteToolRound` 处理
- **THEN** 注入 synthesize pressure AND 不再 hard-reject（T09 行为保持）

---

## 5. 关联变更

| 变更 | 关系 |
|------|------|
| DM-20260701-007 | 前置：Filter v2 + ToolSpec v3 实现（本 change 接线） |
| DM-20260630-012 | 互补：Deliverable schema / format_hints 已有，本 change 统一注入路径 |
| DM-20260614-020 | 约束：D7 保留 LLM 调用权，D2 禁止 D3 |
| DM-20260614-009 | 约束：D2 thin / D7 Leader 边界基线 |

---

## 6. 追溯

```
DM-20260704-001 (demand.md)
  → proposal.md / design.md
  → tasks.md (Phase A/B/C)
  → code: materialize/mups.go, filter pipeline, D7 caller migration
  → acceptance-report.md (S5)
  → openspec/specs/d2-context-engine/d7-boundary.md 更新
```
