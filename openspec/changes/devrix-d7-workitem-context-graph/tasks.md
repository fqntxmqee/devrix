# Tasks: WorkItem × ContextGraph

**Change ID:** `devrix-d7-workitem-context-graph`
**Total Tasks:** 15
**Sprint:** d7-workitem-context-graph

---

## F1 — 契约 + Sibling taxonomy + CL/CB 规则单测

| ID | Description | Status |
|----|-------------|--------|
| T01 | `context_scope.go` — ContextScope + SidechainKey helper | DONE |
| T02 | `context_graph.go` — Link/Bubble kinds, Spec, Record | DONE |
| T03 | `sibling_relation.go` — ClassifySiblingRelation R1–R6 | DONE |
| T04 | `context_link_evaluator.go` — CL0–CL8 | DONE |
| T05 | `context_bubble_evaluator.go` — CB0–CB6 | DONE |
| T06 | WorkItem v3 字段 + feature flag + 单测 PASS | DONE |

## F2 — BubbleStructured 父 Observe

| ID | Description | Status |
|----|-------------|--------|
| T07 | `WorkItemPipelineRound.ContextBubbleKind` 字段 | TODO |
| T08 | 父 Observe 读 structured bubble | TODO |

## F3 — R2 自动 upstream + feature flag

| ID | Description | Status |
|----|-------------|--------|
| T09 | `InferDependencyContextLinks` 接线 TaskManager | TODO |
| T10 | `MaterializeContext` stub + WaveNodesFromSubtree 映射 | TODO |
| T11 | `D7_WORKITEM_CONTEXT_GRAPH=1` 集成测试 | TODO |

## F4 — LLM proposer + Plan 扩展

| ID | Description | Status |
|----|-------------|--------|
| T12 | `ItemPlanOutput` ContextLinkSpecs / BubbleSpec | TODO |
| T13 | Decide 阶段 EvaluateContextLinks/Bubble 顺序 | TODO |

## F5 — D2 sidechain 分区

| ID | Description | Status |
|----|-------------|--------|
| T14 | CreateContextScopeForWorkItem + `wi_<id>` sidechain | TODO |

## F6 — 运维与验收

| ID | Description | Status |
|----|-------------|--------|
| T15 | `/task context show` + ResolveHint + 集成 Gherkin | TODO |
