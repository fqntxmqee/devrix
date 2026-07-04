# Implementation Tasks: MUPS 上下文与工具决策归 D2

**Change ID:** `mups-d2-context-tools-ownership`  
**Demand ID:** DM-20260704-001  
**Status:** S5 Accepted  
**Design:** [`design.md`](design.md)

---

## T 点注册表（本 change 新增，归档时写入 `t-registry.md`）

| T ID | L5 | Phase | 描述 | 优先级 |
|------|-----|-------|------|--------|
| D2-S15-A90-T01 | L5-D2-MUPS-01 | A | MaterializeForMUPS(observe) → Tools 空 + obs schema appendix | P0 |
| D2-S15-A90-T02 | L5-D2-MUPS-02 | A | MaterializeForMUPS(plan) → Tools 空 + strategic plan appendix | P0 |
| D2-S15-A90-T03 | L5-D2-MUPS-03 | A | MaterializeForMUPS(execute, implement) → 完整工具集 | P0 |
| D2-S15-A90-T04 | L5-D2-MUPS-03 | A | MaterializeForMUPS(execute, readonly) → 无 write/bash | P0 |
| D2-S15-A90-T05 | L5-D2-MUPS-03 | A | MaterializeForMUPS(execute, rollup_synth) → Tools 空 | P0 |
| D2-S15-A90-T06 | L5-D2-MUPS-01 | A | verify/learn/decide → ErrPhaseNotMaterializable | P0 |
| D2-S15-A91-T01 | L5-D2-MUPS-03 | A | Filter Step 4: explore agent → Fact+Probe only | P0 |
| D2-S15-A91-T02 | L5-D2-MUPS-03 | A | Filter Step 5: review task_kind → Bounded hint | P0 |
| D2-S15-A91-T03 | L5-D2-MUPS-03 | A | Filter Step 6: readonly profile + MUPS blocked tools | P0 |
| D2-S15-A91-T04 | L5-D2-MUPS-03 | A | Pipeline order invariant test | P0 |
| D2-S15-A92-T01 | L5-D2-MUPS-02 | A | Phase appendix zh/en parity | P0 |
| D2-S15-A92-T02 | L5-D2-MUPS-03 | A | Execute OutputHints 含 deliverable_schema | P0 |
| D2-S18-A90-T01 | L5-D2-MUPS-05 | B | Probe iter≥bound → pressure injection | P1 |
| D2-S18-A90-T02 | L5-D2-MUPS-05 | B | ToolRound Router 4-channel dispatch | P1 |
| D7-S2-A90-T01 | L5-D2-MUPS-02 | B | LLMObservationProposer 无 appendix 常量 | P0 |
| D7-S2-A90-T02 | L5-D2-MUPS-02 | B | LLMStrategicPlanProposer 无 appendix 拼接 | P0 |
| D7-S2-A90-T03 | L5-D2-MUPS-03 | B | WorkItemExecutor 仅 MaterializeForMUPS | P0 |
| D7-S2-A91-T01 | L5-D2-MUPS-04 | C | d7_no_tool_filter_test CI | P0 |
| D7-S2-A91-T02 | L5-D2-MUPS-04 | C | grep dead code: toolsForProfile / filterPipelineTools | P0 |

---

## Phase A: Wire Filter v2 + MaterializeForMUPS API — P0

> **目标：** D2 新 API 可用；7 步 filter pipeline 接线；phase prompt registry。

### A.1 契约类型 — `@L5(L5-D2-MUPS-03)` `@T(D2-S15-A90-T03)`

- [x] **A.1.1** 新增 `shared/contracts/mups_context.go`：`MUPSPhase`, `MUPSContextRequest`, `MUPSPreparedContext`, `IMUPSContextMaterializer`
- [x] **A.1.2** 扩展 `materialize/types.go`：`MaterializePolicy` 补 MUPS 字段（locale, agentProfile）

### A.2 Filter Pipeline — `@T(D2-S15-A91-T01..T04)`

- [x] **A.2.1** 新增 `materialize/filter_pipeline.go`
- [x] **A.2.2** 新增 `filter_pipeline_test.go` — golden + order invariant

### A.3 MaterializeForMUPS 实现 — `@T(D2-S15-A90-T01..T06)`

- [x] **A.3.1** 新增 `materialize/mups_materializer.go`
- [x] **A.3.2** observe/plan: skip tool pipeline, 组装 directive-only messages
- [x] **A.3.3** execute: private chain merge + `compressMessages`
- [x] **A.3.4** `mups_materializer_test.go`

### A.4 Phase Prompt Registry — `@T(D2-S15-A92-T01..T02)`

- [x] **A.4.1** 新增 `materialize/phase_prompts.go`
- [x] **A.4.2** 迁移 observe appendix → `i18n/format_hints_mups.go`
- [x] **A.4.3** plan: 调用已有 `i18n.StrategicPlanAppendix`
- [x] **A.4.4** execute: `buildExecuteOutputHints(wi)` 含 deliverable_schema
- [x] **A.4.5** rollup_synth: 新增 synthesis appendix
- [x] **A.4.6** `phase_prompts_test.go` snapshot

### A.5 Wiring — `@T(D2-S15-A90-T03)`

- [x] **A.5.1** `kernel/context_engine.go` 暴露 `MaterializeForMUPS`
- [x] **A.5.2** `bootstrap/turn_adapter.go` adapter 实现
- [x] **A.5.3** ~~Feature flag~~ 已移除（Phase C 直接启用）

**Quality Gate Phase A:**
- [x] `go test -race ./internal/layers/contextengine/materialize/...`
- [x] `d2_thin_test` PASS

---

## Phase B: Migrate D7 Callers + ToolRound — P0/P1

### B.1 D7 Observe 迁移 — `@T(D7-S2-A90-T01)`

- [x] **B.1.1** `LLMObservationProposer.ProposeObservations` 改调 `MaterializeForMUPS(phase=observe)`
- [x] **B.1.2** 删除 D7 `observationTaskAppendixZH/EN` 常量
- [x] **B.1.3** 更新 `llm_observation_proposer_test.go`

### B.2 D7 Plan 迁移 — `@T(D7-S2-A90-T02)`

- [x] **B.2.1** `LLMStrategicPlanProposer` 改调 `MaterializeForMUPS(phase=plan)`
- [x] **B.2.2** 删除 D7 侧 `StrategicPlanAppendix` 拼接
- [x] **B.2.3** 更新 `strategic_plan_proposer_test.go`

### B.3 D7 Execute 迁移 — `@T(D7-S2-A90-T03)`

- [x] **B.3.1** `DefaultWorkItemExecutor.prepareContext` 统一 `MaterializeForMUPS(phase=execute)`
- [x] **B.3.2** 删除双轨：`Materializer.Materialize` + `ContextPreparer.Prepare` fallback merge
- [x] **B.3.3** 删除 `appendWorkItemFormatHints`
- [x] **B.3.4** 更新 `item_pipeline_materialize_test.go`, `workitem_executor_test.go`

### B.4 ToolRound + ToolChannel — `@T(D2-S18-A90-T01..T02)`

- [x] **B.4.1** 新增 `contextengine/enforce/toolround/`
- [x] **B.4.2** 扩展 D2-S18 `ExecuteToolRound` 接受 TaskKind + RemainingBudget
- [x] **B.4.3** Probe PromptPressure 三档
- [x] **B.4.4** D2 toolround 默认 ModeShadow（并行 old/new diff 留 follow-up）
- [x] **B.4.5** `channel_router_test.go`

### B.5 Feature flag 切换

- [x] **B.5.1** MaterializeForMUPS 始终启用（flag 已移除）
- [x] **B.5.2** 集成测试全绿

**Quality Gate Phase B:**
- [x] `go test -race ./internal/layers/orchestration/sessionorchestrator/... ./internal/layers/contextengine/...`

---

## Phase C: Remove D7 Dead Code + Boundary Lint — P0

### C.1 删除 D7/D2 死代码 — `@T(D7-S2-A91-T02)`

- [x] **C.1.1** 删除 `materialize/compressor.go::toolsForProfile`
- [x] **C.1.2** 删除 `sessionorchestrator/workitem_tools.go`
- [x] **C.1.3** 清理 `workitem_executor.go` 中 filterPipelineTools 调用
- [x] **C.1.4** D7 `mups/execute/toolchannel/` 保留测试兼容（S7 follow-up thin）

### C.2 Lint — `@T(D7-S2-A91-T01)`

- [x] **C.2.1** 新增 `internal/lint/layer/d7_no_tool_filter_test.go`
- [x] **C.2.2** grep gate: sessionorchestrator 无硬编码 tool name 列表
- [x] **C.2.3** 回归 `d2_thin_test`, `d7_boundary_test`

### C.3 OpenSpec 同步（S5 验收后、S6 归档前）

- [x] **C.3.1** 更新 `openspec/specs/d2-context-engine/d7-boundary.md` §10
- [x] **C.3.2** 登记 T 点至 `d2-context-engine/t-registry.md` + `d7-orchestration/t-registry.md`
- [x] **C.3.3** 移除 feature flag
- [x] **C.3.4** `acceptance-report.md`

**Quality Gate Phase C:**
- [x] 全量 `go test -race ./internal/...`
- [x] lint tests PASS

---

## 验收清单（S5）

- [x] L5-D2-MUPS-01..05 全部 P0 PASS
- [x] 19 新 T 点 IMPLEMENTED
- [x] `d7-boundary.md` §10 与 design.md 一致
- [x] 无 D7 import `enforce/tools/filter`
- [x] 无新增 D2→D3 依赖
