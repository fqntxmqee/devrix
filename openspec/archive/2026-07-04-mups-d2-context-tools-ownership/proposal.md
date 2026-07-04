# Proposal: MUPS 上下文与工具决策归 D2 统一负责

**Change ID:** `mups-d2-context-tools-ownership`  
**Demand ID:** DM-20260704-001  
**Created:** 2026-07-04  
**Status:** Draft  
**Related:** DM-20260701-007 (Filter v2), DM-20260630-012 (format_hints), `d7-boundary.md` v2.1  
**Demand:** [`demand.md`](demand.md)

---

## Problem Statement

MUPS v4 管道已在 D7 落地 Observe→Plan→Execute→Verify→Learn→Decide，但 **上下文组装与工具决策仍分散在 D7**，与 D2↔D7 边界规范（D7 Leader 编排 / D2 Follower 执行原语）不一致：

1. **Filter v2 已实现未接线** — `PerEmissionClassFilter`、`PerTaskKindFilter` 在 `enforce/tools/filter/` 有完整测试，但 `PrepareOrchestrator` 与 `DefaultMaterializer` 未调用。
2. **`toolsForProfile` 硬编码** — `materialize/compressor.go` 按字符串返回固定 `read_file`/`grep` 或空集，绕过 registry 与 v3 ToolSpec metadata。
3. **Phase appendix 在 D7** — `LLMObservationProposer`、`LLMStrategicPlanProposer` 在 D7 拼接 system prompt appendix，违反「D7 不组装 MUPS 节点 prompt」原则。
4. **`WithLocatorPhase` 未消费** — D7 `item_pipeline.go` 设置 locator phase，D2 Prepare 路径不读取，无法按 MUPS 节点差异化过滤/提示。
5. **`toolchannel` 未集成** — D7 `mups/execute/toolchannel/` 有接口与 Router 骨架，Execute ReAct 仍走 D7 `filterPipelineTools` + 裸 ToolRound，probe 终止逻辑未归 D2-S18。

## Proposed Solution

引入 **`MaterializeForMUPS`** 作为 D2 对 MUPS 节点的唯一上下文出口：

```
D7 ItemPipeline (phase, WorkItem, Turn)
    └── D2.MaterializeForMUPS(MUPSContextRequest)
            ├── PromptAssembler (7 static sections)
            ├── Phase appendix registry (observe/plan/execute/rollup_synth)
            ├── 7-step Tool Filter Pipeline
            ├── Compression + TokenBudget
            └── → MUPSPreparedContext
    └── D7.InvokeLLM(D3)  // D7 仍持有 LLM 调用权
    └── D2.ExecuteToolRound (Phase B: + ToolChannel Router)
```

详细架构见 [`design.md`](design.md)。

## Capabilities

| Capability ID | 描述 | 域 |
|---------------|------|-----|
| **D2-S15-A90** | `MaterializeForMUPS` API — MUPS 节点上下文 materialize 入口 | D2 |
| **D2-S15-A91** | MUPS Tool Filter Pipeline — registry → permission → agent → emission → task_kind → profile | D2 |
| **D2-S15-A92** | MUPS Phase Prompt Registry — observe/plan/execute/rollup_synth appendix | D2 |
| **D2-S18-A90** | ToolRound + ToolChannel Router 集成（Phase B） | D2 |
| **D7-S2-A90** | MUPS caller 迁移 — Observe/Plan/Execute 改调 MaterializeForMUPS | D7 |
| **D7-S2-A91** | D7 死代码清除 — toolsForProfile 调用链、filterPipelineTools、 scattered appendix | D7 |

## Scope

### In Scope

- 新 API 类型：`MUPSPhase`, `MUPSContextRequest`, `MUPSPreparedContext`
- D2 7 步 filter pipeline 接线 + MUPSPhase→EmissionClass 映射表
- Phase appendix 从 D7 迁至 D2 i18n / `format_hints.go`
- D7 `LLMObservationProposer` / `LLMStrategicPlanProposer` / `DefaultWorkItemExecutor` 调用迁移
- Phase B：`toolchannel.Router` → D2-S18
- Lint：`sessionorchestrator` 禁止 import `enforce/tools/filter`
- T 层测试点 + 边界回归（`d2_thin_test`, 新 `d7_no_tool_filter_test`）

### Non-Goals

- 修改 MUPS 节点语义或 SpawnPolicy
- 修改 PlanChannel（commit/protocol/scenario/exploration）执行策略
- FastPath `RunTurn` 全量替换 `PrepareForTurn`（后续 change）
- 新增 LLM 战术 prose（仅搬迁已有 appendix）
- D2→D3 调用（永久禁止）
- workspace 维度 filter（OOS-10，DM-20260701-007 共识）

## Impact Analysis

| Component | Change | Details |
|-----------|--------|---------|
| `contextengine/materialize/` | **新增** | `mups_materializer.go`, `filter_pipeline.go`, `phase_prompts.go` |
| `contextengine/kernel/context_engine.go` | **扩展** | 暴露 `MaterializeForMUPS` on `IEngine` / adapter |
| `contextengine/materialize/compressor.go` | **删除** | `toolsForProfile`（Phase C） |
| `sessionorchestrator/llm_*_proposer.go` | **简化** | 删除 appendix 拼接，改调 MaterializeForMUPS |
| `sessionorchestrator/workitem_executor.go` | **简化** | `prepareContext` 统一走 D2 MUPS materialize |
| `sessionorchestrator/workitem_tools.go` | **删除** | `filterPipelineTools`（Phase C） |
| `mups/execute/toolchannel/` | **迁移** | Router 逻辑 → D2-S18（Phase B） |
| `openspec/specs/d2-context-engine/d7-boundary.md` | **更新** | §10 MUPS context ownership |
| `internal/lint/layer/` | **新增** | `d7_no_tool_filter_test.go` |

## Success Criteria

- [ ] Observe/Plan/Execute LLM 路径均通过 `MaterializeForMUPS` 获取 context
- [ ] Filter v2 三维过滤在 Execute phase 集成测试 PASS
- [ ] D7 proposer 无本地 observation/strategic appendix 常量
- [ ] `d2_thin_test` + `d7_no_tool_filter_test` CI PASS
- [ ] `d7-boundary.md` 与 design.md 一致

## Risks & Mitigations

| Risk | Prob. | Impact | Mitigation |
|------|-------|--------|------------|
| 双路径并存导致 tools 不一致 | Med | High | Phase B 前 feature flag；golden test 对比旧/新输出 |
| MaterializeForMUPS 性能回归 | Low | Med | 复用现有 Prepare 缓存；span 对比 token_est |
| ToolChannel 迁移破坏 Execute | Med | High | Phase B shadow mode（DM-20260701-007 T07 模式） |
| appendix 搬迁遗漏 locale | Low | Med | 现有 i18n 测试迁移 + snapshot test |
