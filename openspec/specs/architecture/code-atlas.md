# Devrix Code Atlas

**Capability:** code-atlas
**Status:** Active
**Version:** 1.1.0
**Last Updated:** 2026-06-13
**Parent:** `openspec/specs/architecture/layering.md`
**Demand:** DM-20260610-012 (QueryLoop v2)

---

## Overview

Devrix 代码图谱：D-S 到包路径的快速索引。新建文件时 MUST 先查 D-S 归属，再落盘到对应目录。

**可读版：** `docs/architecture/code-map.md`

---

## QueryLoop v2 Module Index

| L4 ID | 名称 | D-S | 包路径 | 关键类型 |
|-------|------|-----|--------|----------|
| query_loop | QueryLoop 主循环 | D2-S10 | `contextengine/query/` | `Loop`, `SubQuery`, `FlowTap` |
| toolrunner | 工具执行与沙箱 | D2-S5/D2-S8 | `contextengine/policy/toolrunner/` | `ToolRunner`, `Sandbox`, builtins |
| user_context | UserContext prepend | D2-S10 | `contextengine/usercontext/` | `Provider`, `PrependForAPI` |
| attachments | Plan mode 附件 | D2-S10 | `contextengine/attachments/` | `Registry` |
| permission_mode | Plan 写过滤 | D2-S10 | `contextengine/policy/permission/` | `Mode`, `PlanFilePath` |
| task_tools | Task 磁盘工具 | D2-S10 | `contextengine/tasks/` | `Manager`, `DiskStore` |
| subquery | SubQuery / Fork | D2-S10 | `contextengine/query/` | `Run`, `BuildForkedMessages` |
| sidechain_transcript | Sidechain JSONL | D2-S10 | `contextengine/persist/transcript/` | `SidechainStore` |
| background_tasks | Background 通知 | D2-S11 | `contextengine/queue/` | `SessionQueue`, `ModeTaskNotification` |
| execution_flow | FlowEvent 双通道 | ORCH-S2 | `orchestration/flow/` | `Hub`, `Publish` |
| workplan | WorkPlan 读模型 | ORCH-S1 | `orchestration/workplan/` | `Service`, `Snapshot` |
| im_flow_sink | IM worker_progress | ORCH-S2 | `orchestration/imsink/` | `GatewaySink` |
| delegate | Hub-Spoke 委派 | D4-S10 | `multiagent/delegate/` | `Service`, `FlowBridge`, `WorkerSpec` |
| worker_engine | Worker 隔离引擎 | D4-S10 | `multiagent/agent/` | `WorkerEngine`, `ProcessOverlay` |
| worktree | 沙箱工作目录 | D2-S12 | `contextengine/worktree/` | `Manager`, `Enter`, `Exit` |

---

## Shared Contracts (Cross-Layer)

| 契约 | 路径 | 消费方 |
|------|------|--------|
| `FlowEvent`, `ExecutionFlowHub` | `shared/contracts/execution_flow.go` | D2 SubQuery, D4 Delegate, ORCH Hub |
| `WorkPlanSnapshot` | `shared/contracts/execution_flow.go` | ORCH WorkPlan, delegate_status |
| `IPermissionGate`, `FileAutoApprover` | `shared/contracts/permission.go` | D2 ContextEngine, D1 Gateway, D4 Agent |
| `ILLMGateway`, `ITierResolver` | `llmgateway/contracts.go` | D2 ContextEngine (via bridges/llm) |
| `IToolRunner`, `IToolRegistry` | `contextengine/policy/toolrunner/` | D2 QueryLoop, registry |
| `ToolRegistry`, builtins | `contextengine/policy/toolrunner/` | bash, read_file, glob, grep, edit |
| `ExecutionFlowConfig` | `shared/config/execution_flow.go` | bootstrap, Hub |
| `WorktreeConfig` | `shared/config/worktree.go` | delegate, worktree |
| `DelegateConfig` | `shared/config/` (multi_agent) | delegate Service |

---

## Bootstrap Wiring

| 文件 | 职责 |
|------|------|
| `internal/bootstrap/execution_flow.go` | ExecutionFlowHub 全局注册、WorkPlan 注入 |
| `internal/bootstrap/delegate.go` | DelegateService、SubQuery fallback、worktree |
| `internal/bootstrap/cli_events.go` | CLI worker_progress 渲染 |

入口：`cmd/devrix/main.go` 在 `multi_agent.enabled` 时调用 `WireDelegate`。

---

## Dependency Direction

```
D1 Gateway ──read──► ORCH WorkPlan / imsink
D2 QueryLoop ──publish──► ORCH ExecutionFlowHub
D4 Delegate ──publish──► ORCH ExecutionFlowHub
ORCH Hub ──drain──► D2 SessionQueue (Leader)
ORCH Hub ──emit──► D1 Gateway (worker_progress)
D4 Delegate ──fork──► D2 WorkerEngine ──► QueryLoop
```

禁止：ORCH 包 import D1 adapter 实现；D4 Worker import delegate 工具注册。

---

## Test Placement

| T 层域 | 测试目录 |
|-------|----------|
| D2-S10 QueryLoop | `contextengine/query/*_test.go`, `tests/integration/query_loop_*` |
| D2-S12 Worktree | `contextengine/worktree/manager_test.go` |
| D4-S10 Delegate | `multiagent/delegate/*_test.go`, `contextengine/delegate_*_test.go` |
| ORCH | `orchestration/flow/hub_test.go`, `orchestration/workplan/service_test.go` |

---

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-10 | QueryLoop v2 module index (DM-20260610-012) |
| 1.1.0 | 2026-06-13 | +toolrunner; docs/architecture 可读版同步 |
