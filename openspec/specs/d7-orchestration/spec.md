# Orchestration Read Model Specification

**Capability:** orchestration
**Change ID:** devrix-queryloop-context (archived 2026-06-10)
**Demand:** DM-20260610-012
**Layer:** Cross-cutting (v2 package, not top-level Domain)
**Version:** 1.0.0
**Status:** Canonical — source of truth
**Package:** `internal/layers/orchestration/`
**Layering Spec:** `openspec/specs/architecture/layering.md` (ORCH-S1/S2)

---

## Overview

v2.0 引入 **Hub-Spoke** 编排读模型：D2 SubQuery 与 D4 Delegate Worker 的运行时事件经 `ExecutionFlowHub` 聚合为 `WorkPlan` 快照，供 Leader `delegate_status` 与 D1 IM 进度树只读消费。

写侧仍分散在 D2（Task、SubQuery）、D4（Delegate）；v3 若写模型膨胀可升格为 D7 Work Orchestration（见归档 `design-orchestration-v3.md`）。

---

## ADDED Requirements

### Requirement: WorkPlan Read Model

`WorkPlanService` MUST maintain an in-memory read model per session aggregating Task snapshots and ExecutionFlow snapshots. `Snapshot(sessionID)` MUST return current tasks and active/completed flows with recent events.

**Priority:** P0  
**L4:** workplan  
**T:** ORCH-S2-T01

#### Scenario: Snapshot includes tasks and flows

- GIVEN FlowStarted and TaskManager updates for a session
- WHEN WorkPlan.Snapshot is called
- THEN response includes ExecutionFlows with status and RecentEvents
- AND linked Task snapshots reflect owner and in_progress status

---

### Requirement: ExecutionFlowHub Publish

`ExecutionFlowHub.Publish` MUST accept unified `FlowEvent` from SubQuery FlowTap and D4 DelegateFlowBridge, apply to WorkPlan, enqueue Leader delegate-progress, and optionally emit IM worker_progress.

**Priority:** P0  
**L4:** execution_flow_hub  
**T:** D4-S10-T04, D4-S10-T06

#### Scenario: Dual publish on flow event

- GIVEN execution_flow enabled with im_progress and link_tasks
- WHEN Hub.Publish receives FlowStarted
- THEN WorkPlan is updated
- AND SessionQueue receives delegate-progress for Leader drain
- AND IM sink receives worker_progress when configured

---

## Configuration

```yaml
context_engine:
  execution_flow:
    enabled: true
    link_tasks: true
    im_progress: true
```

---

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-10 | Initial ORCH read model (DM-20260610-012 v2.0) |
