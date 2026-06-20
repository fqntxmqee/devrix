# D4 Multi-Agent Layer Specification

**Capability:** multi-agent
**Version:** 3.0.0
**Status:** Canonical — source of truth (S11–S16 价值流)
**Last Updated:** 2026-06-14
**Change ID:** devrix-d4-sa-refine
**Demand ID:** DM-20260614-018
**Layering Spec:** `openspec/specs/architecture/layering.md`
**Domain SoT:** `d4-domain.md`
**D7 Boundary:** `d7-boundary.md`

---

## Overview

D4 多智能体域是 **Delegation Execution Follower**：供给 Agent/Worker 实例、运行主循环、隔离合并、执行 D7 派发的 Worker。**Hub-Spoke 编排、FlowEvent 发布、WorkPlan 聚合归 D7**（R1 D7-1，见 `d7-boundary.md`）。

---

## DSAFT 结构

### Canonical 价值流（SoT）— D4-S11–S16

| ID | Scenario | 承诺 | Status |
|----|----------|------|--------|
| D4-S11 | ProvisionAgent | C1：创建、配额、协作模式 | REGISTRY |
| D4-S12 | RunAgentLoop | C2：生命周期、PermissionGate | REGISTRY |
| D4-S13 | IsolateAndMerge | C3：Fork/Join、COW、Worker overlay | REGISTRY |
| D4-S14 | ExecuteWorker | C4：Worker fork→run→join | REGISTRY |
| D4-S15 | InvokeExternalAgent | C5：CLI/Cursor Agent Tool | REGISTRY |
| D4-S16 | ConfigureAgents | C6：multi_agent 配置 | REGISTRY |

### Legacy Module Index（冻结追溯）— D4-S1–S10

| ID | Scenario | Status | Canonical |
|----|----------|--------|-----------|
| D4-S1 | Factory | IMPLEMENTED | → S11 |
| D4-S2 | Agent | IMPLEMENTED | → S12, S13 |
| D4-S3 | ForkJoin | IMPLEMENTED | → S13 |
| D4-S4 | Collaboration | IMPLEMENTED | → S11 |
| D4-S5 | Observer | IMPLEMENTED | → kernel |
| D4-S6 | AgentTool | IMPLEMENTED | → S15 |
| D4-S7 | Builtin | IMPLEMENTED | → S11 + D7 fallback |
| D4-S8 | Observability | IMPLEMENTED | → D5 |
| D4-S9 | SessionView | IMPLEMENTED | → S13 |
| D4-S10 | Delegate | IMPLEMENTED | 执行 → S14；编排 → D7 |

> v1.0 运行时仍映射 Legacy 路径；规格 SoT = S11–S16。

---

## Architecture（目标态）

```
D7 DispatchWorker
    └─ D4-S14 ExecuteWorker
          ├─ D4-S11 ProvisionAgent (fork 前)
          ├─ D4-S12 RunAgentLoop (Worker 循环)
          ├─ D4-S13 IsolateAndMerge (COW + worktree)
          └─ D7 SpokeBridge.Publish (非 D4)

D4 独立面：
  D4-S15 InvokeExternalAgent (CLI/Cursor)
  D4-S16 ConfigureAgents
  kernel: contracts + Observer → D7 Bridge (v2.0)
```

---

## Cross-Domain Dependencies

| Domain | 依赖 | D4 使用 |
|--------|------|---------|
| D1 | PermissionManager | S12 PermissionGate |
| D2 | IEngine, LoopDeps | S12/S14 runLoop |
| D5 | observability.Bridge | tracer emit（metric 定义归 D5） |
| D7 | WorkerExecutor 消费方 | S14 被 Dispatch 调用 |
| Shared | contracts, config, types | 全模块 |

**Out of Scope：** D7 ExecutionFlowHub 直连 Publish（v2.0 迁出 `delegate/bridge.go`）。

---

## Package Map（v1.0 当前 / v2.0 目标）

| v1.0 路径 | Canonical S | v2.0 slug |
|----------|-------------|-----------|
| `factory/`, `collaboration/`, `builtin/` | S11 | `provision/` |
| `agent/` (lifecycle, perm) | S12 | `run/` |
| `forkjoin`, `sessionview/`, `worker_engine` | S13 | `isolate/` |
| `delegate/service.go` | S14 | `execute/` |
| `tool/` | S15 | `external/` |
| `shared/config/multiagent.go` | S16 | `configure/` |
| `contracts.go`, `observer/` | kernel | `kernel/` |

---

## Requirements（Canonical 摘要）

### Requirement: Provision Agent

Agent 工厂 MUST 校验配额并支持协作模式 prompt 增强。

#### Scenario: Create agent with quota
- GIVEN valid session_id
- WHEN AgentFactory.Create
- THEN Agent in CREATED state with unique ID

### Requirement: Run Agent Loop

Agent MUST 遵循状态机；CRITICAL 工具 MUST 经 PermissionGate。

#### Scenario: Permission denied terminates agent
- GIVEN CRITICAL tool pending
- WHEN user denies
- THEN TERMINATED with permission error

### Requirement: Isolate Parallel Work

Fork/Worker MUST NOT 污染父 Session metadata（COW）。

#### Scenario: Child metadata isolated
- GIVEN parent session
- WHEN child writes via SessionView
- THEN parent metadata unchanged

### Requirement: Execute Worker

D7 派发 WorkerSpec 后，D4 MUST fork→run→join；Worker MUST NOT delegate or fork。

#### Scenario: Worker cannot delegate
- GIVEN Worker from ExecuteWorker
- WHEN delegate_* invoked
- THEN rejected

### Requirement: External Agent Tools

CLI/Cursor tools MUST isolate subprocess per D1 session.

#### Scenario: Session isolation
- GIVEN two sessions
- WHEN both use CLI tool
- THEN separate subprocesses

---

### Requirement: Sub-Agent Mode Field on delegate/free_fork Tools (devrix-context-budget-phase-b)

The `delegate_*` and `free_fork` LLM tools MUST expose a `mode` field in
their input schema with enum values `[brief, fork, full]`, default
`brief`. The mode controls sub-agent context inheritance via
`SubTurnRunner` (D7-S2-A06 3-mode dispatch). Empty / unknown values
MUST defer to `SubagentConfig.DefaultMode`.

This is the surface-level counterpart to the SubTurnRunner 3-mode
dispatch — without exposing `mode` to the LLM, callers cannot opt
out of the new `brief` default. The D4 worker spec mirrors the same
field for the D4-execution path.

**Priority**: P0
**T mapping**: D4-S14-A07-T01 (delegate schema), D4-S14-A07-T02 (free_fork schema),
`internal/layers/orchestration/delegatetools/delegate_schema_test.go`,
`internal/layers/orchestration/sessionorchestrator/turn_tools_test.go`

#### Scenario: delegate_* schema declares mode enum

- GIVEN the `delegate_explore` / `delegate_plan` / `delegate_implement`
  tool input schema
- THEN the schema has a `mode` property
- AND its `enum` is `[brief, fork, full]`
- AND its `default` is `brief`

<!-- T: D4-S14-A07-T01 -->

#### Scenario: free_fork schema declares mode enum on request items

- GIVEN the `free_fork` tool input schema
- THEN `requests[].properties.mode` exists
- AND its `enum` is `[brief, fork, full]`
- AND its `default` is `brief`

<!-- T: D4-S14-A07-T02 -->

#### Scenario: mode propagates from delegate tool input to SubTurnRequest

- GIVEN a delegate_* call with `mode=full` in input
- WHEN the dispatcher resolves to the D2 SubQuery fallback
- THEN `SubTurnRequest.Mode` equals `full`
- AND `SubTurnRunner` dispatches as `mode=full` (legacy behavior)

<!-- T: D4-S14-A07-T01 boundary -->

#### Scenario: missing mode defers to SubagentConfig default

- GIVEN a delegate_* call with no `mode` in input
- AND `SubagentConfig.DefaultMode="brief"`
- WHEN the dispatcher resolves to the D2 SubQuery fallback
- THEN `SubTurnRequest.Mode` is empty
- AND `SubTurnRunner` resolves via DefaultMode → `brief`

---

## Registries

- **Domain:** `d4-domain.md`
- **Boundary:** `d7-boundary.md`
- **A 层:** `a-registry.md`
- **F 层:** `f-registry.md`
- **T 层:** `t-registry.md`（38 IMPLEMENTED，Legacy Archive）
- **Span:** `span-registry.md`（S8 迁 D5 声明）

---

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-08 | Initial V1 (DM-20260608-005) |
| 1.1.0 | 2026-06-10 | D4-S10 Delegate (DM-20260610-012) |
| 2.0.0 | 2026-06-13 | SessionView, Agent Tools, Observability |
| 3.0.0 | 2026-06-14 | S11–S16 价值流；Hub-Spoke 迁 D7；Legacy S1–S10 冻结（DM-20260614-018） |
