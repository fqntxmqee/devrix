# Delta: D2 Context Engine — Tool Metadata Control Plane + Filter v2

**Change ID:** `devrix-mups-tool-classification-and-channel-autonomy`
**Demand ID:** DM-20260701-007
**Affects:** D2-S15 (Prepare — Token Audit + new Tool Metadata Control Plane)

---

## ADDED

### Requirement: D2-S15-A02 ToolSpec v3 — 6 control plane fields

`ToolSpec` SHALL include 6 control plane fields in struct tail position (zero break for existing 9 fields):
- `EmissionClass EmissionClass` (Fact / Action / Probe / Experiment)
- `ConvergenceContract ConvergenceContract` (None / Bounded / OpenEnded)
- `IterationBound IterationBound` (MaxN int)
- `SourceUncertainty SourceUncertainty` (Prior float [0,1])
- `MaxResultSizeChars int` (截断阈值)
- `TruncateMarkerText string` (截断 marker 模板)

#### Scenario: ToolSpec v3 struct literal 兼容性
- GIVEN 现有 9 字段 ToolSpec literal 在 19 工具 surface 全部使用
- WHEN 加 6 control plane 字段在 struct 末尾位置
- THEN 所有现有 ToolSpec literal 编译 0 break
- AND grep `^type ToolSpec struct` 仅 1 命中

#### Scenario: 19 工具 orthogonal_flags 默认 metadata
- GIVEN 19 工具 read/grep/glob/write/edit/bash/lsp/free_fork/delegate_*
- WHEN 全部加 orthogonal_flags 默认 metadata
- THEN read/grep/glob = Probe + Bounded(15)（H12 共识）
- AND write/edit/bash = Action
- AND lsp 拆分 3 Fact (lsp_goto_definition/hover/references) + 2 Probe (lsp_workspace_symbol/code_action)
- AND free_fork = Experiment
- AND silent default CI gate: 缺字段 → go test FAIL

### Requirement: D2-S15-A02 Filter v2 — 三维过滤

Filter v2 SHALL 实现 per_emission_class + per_task_kind + per_agent 二级过滤:

#### Scenario: PerEmissionClassFilter
- GIVEN Fact/Action/Probe/Experiment/composite/empty
- WHEN apply filter
- THEN 严格按 emission_class 过滤 tool call

#### Scenario: PerTaskKindFilter + taskKindBound
- GIVEN task_kind ∈ {review, edit, test, observe, refactor}
- WHEN apply
- THEN review=Bounded(15), edit=Bounded(10), test=Bounded(12), observe=OpenEnded, refactor=Bounded(8)
- AND IsTighter: Bounded vs Bounded 收紧；Bounded vs OpenEnded 不得放宽

#### Scenario: PerAgentFilter v2
- GIVEN agent ∈ {explore, worker, delegate, planner, fix, main}
- WHEN apply
- THEN explore=Fact+Probe; worker=Fact+Action+Probe; delegate=Probe+Action
- AND 6 agent 兼容 + 9 既有 0 regression

### Requirement: D2-S15-A02 TruncateWithMarker

`TruncateWithMarker(text, maxChars, marker)` SHALL 截断超长 tool result + 追加 `complete=false` marker，对 LLM 透明。

#### Scenario: 长 output 截断
- GIVEN text=50000 chars, maxChars=8000
- WHEN truncate
- THEN 保留前 8000 chars + append marker 包含 complete=false
- AND marker 必含 complete=false (P0-AC-3)

#### Scenario: wired 进 kernel
- GIVEN context_engine_persist_v2.go 持久化路径
- WHEN tool result 超 MaxResultSizeChars
- THEN 走 TruncateWithMarker, marker 必含 complete=false
- AND LLM 看到截断 + marker 提示"未完待续", 治本 8K token 自我循环
