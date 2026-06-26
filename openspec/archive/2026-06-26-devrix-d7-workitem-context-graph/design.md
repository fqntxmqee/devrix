# Design: WorkItem × ContextGraph

**Change ID:** `devrix-d7-workitem-context-graph`
**SoT:** `openspec/specs/d7-orchestration/workitem-context-graph-design.md` v0.1.0

本 Change 的详细架构设计以 SoT 文档为准。Proposal §4 Review 决议已锁定 OQ-CG-2（share_summary 仅 completed→pending 单向）。

## Phase 映射

| Phase | 本 Change tasks | 运行时 |
|-------|-----------------|--------|
| F1 | T01–T06 | 契约 + 规则单测 |
| F2 | T07–T08 | BubbleEvaluator 接线 |
| F3 | T09–T11 | `D7_WORKITEM_CONTEXT_GRAPH=1` |
| F4–F6 | T12–T15 | 同上 |

## 包边界

- 契约与规则：`workmodel/`（与 SpawnPolicy 同层）
- MaterializeContext / Pipeline 接线：`sessionorchestrator/`（F3+）
- Wave 投影：`worktree_wave.go`（F3 改 ContextFresh 硬编码）
