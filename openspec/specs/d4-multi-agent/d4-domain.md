# D4 Multi-Agent Domain

**Domain ID:** D4
**Slug:** `multiagent`
**Type:** Core Domain
**Status:** Active — Canonical S11–S16 (v1.0 registry, DM-20260614-018)
**Depends On:** D2 (`IEngine`), D1 (`PermissionManager`), D5 (tracer emit)
**Depended By:** D7 (`WorkerExecutor` consumer)
**Cross-Domain SoT:** `d7-boundary.md`
**Change:** `openspec/changes/devrix-d4-sa-refine/`

---

## North Star

**在 D7 给定 Worker 派发参数后，可靠地供给 Agent 实例、执行隔离的子任务循环、合并结果——作为 Delegation Execution Follower，不承担 Hub-Spoke 编排与 Flow 发布。**

| 可验证承诺 | Canonical S |
|-----------|-------------|
| 按配额创建 Agent/Worker，注入协作模式 | D4-S11 ProvisionAgent |
| Agent 主循环可取消；CRITICAL 工具等权限 | D4-S12 RunAgentLoop |
| Fork/Worker 执行不污染父 Session | D4-S13 IsolateAndMerge |
| 给定 WorkerSpec 后 fork→run→join | D4-S14 ExecuteWorker |
| CLI/Cursor 外部 Agent Tool Session 隔离 | D4-S15 InvokeExternalAgent |
| multi_agent 配置加载与校验 | D4-S16 ConfigureAgents |

---

## Out of Scope

| 能力 | 归属 | 备注 |
|------|------|------|
| Hub-Spoke 路由 / delegate_* 工具 | D7-S2 | `delegatetools/` |
| Spoke 派发矩阵 / fallback 决策 | D7-S2 | v2.0 `hubspoke/dispatch.go` |
| FlowEvent 发布 / FlowBridge | D7-S4 | v2.0 自 D4/D2 迁入 |
| WorkPlan / delegate-progress drain | D7-S4 | `flow/`, `sessionqueue/` |
| SubQuery Flow 发布 | D7-S4 | v2.0 自 D2 `flow_report` 迁入 |
| 权限 UI | D1 | Gateway 注入 ResolvePermission |
| Span/Metric 定义 | D5 | D4 仅 emit hook |
| 结论质量 Judge | D6 | — |
| LLM 调用 | D3 | 经 D2 QueryLoop |

---

## DSAFT 双轨

### Canonical 价值流（SoT）— D4-S11–S16

| S ID | Scenario | Responsibility | Status |
|------|----------|----------------|--------|
| D4-S11 | ProvisionAgent | 创建、配额、协作模式 prompt、Builtin 注册 | REGISTRY |
| D4-S12 | RunAgentLoop | 生命周期、PermissionGate、状态机 | REGISTRY |
| D4-S13 | IsolateAndMerge | Fork/Join、SessionView COW、WorkerEngine overlay | REGISTRY |
| D4-S14 | ExecuteWorker | Worker fork→run→join（D7 派发） | REGISTRY |
| D4-S15 | InvokeExternalAgent | CLI/Cursor Agent Tool | REGISTRY |
| D4-S16 | ConfigureAgents | multi_agent 配置（横切） | REGISTRY |

### Legacy Module Index（冻结追溯）— D4-S1–S10

| Module ID | Scenario | Status | Canonical 映射 |
|-----------|----------|--------|----------------|
| D4-S1 | Factory | Legacy | → S11 |
| D4-S2 | Agent | Legacy | → S12, S13（WorkerEngine） |
| D4-S3 | ForkJoin | Legacy | → S13 |
| D4-S4 | Collaboration | Legacy | → S11 |
| D4-S5 | Observer | Legacy | → kernel |
| D4-S6 | AgentTool | Legacy | → S15 |
| D4-S7 | Builtin | Legacy | → S11 + **D7 fallback 路由** |
| D4-S8 | Observability | Legacy | → **D5** |
| D4-S9 | SessionView | Legacy | → S13 |
| D4-S10 | Delegate | Legacy | 执行面 → S14；编排面 → **D7-S2/S4** |

---

## 与 D7 关系（Leader / Follower）

> 完整矩阵见 [`d7-boundary.md`](./d7-boundary.md)。

| D7 动作 | D4 响应 |
|---------|---------|
| `DispatchWorker` → Spoke=D4Worker | `ExecuteWorker`（S14） |
| `delegatetools` 调 WorkerExecutor | fork→run→join |
| FlowEvent 发布 | **D7 only**（D4 不 Publish） |
| Wave 调度外部 Runner | 不经 D4（D2 SubQuery） |

**禁止：** D4 import `orchestration/flow` 做 `hub.Publish`（v2.0-b 后 lint 强制）。

---

## 物理路径（v2.0 目标）

| Canonical S | scenario-slug | v1.0 当前 | v2.0 目标 |
|-------------|---------------|----------|-----------|
| S11 | `provision` | `factory/`, `collaboration/`, `builtin/` | `multiagent/provision/` |
| S12 | `run` | `agent/`（lifecycle, perm） | `multiagent/run/` |
| S13 | `isolate` | `forkjoin`, `sessionview/` | `multiagent/isolate/` |
| S14 | `execute` | `delegate/service.go` | `multiagent/execute/` |
| S15 | `external` | `tool/` | `multiagent/external/` |
| S16 | `configure` | `shared/config/multiagent.go` | `multiagent/configure/` |
| kernel | `kernel` | `contracts.go`, `observer/` | `multiagent/kernel/` |

---

## 规格文档索引

| 文档 | 用途 |
|------|------|
| `spec.md` | Gherkin 验收规格 |
| `terminal-state-guide.md` | D7 派发时序、S11–S16 A 树、硬约束 |
| `observability-guide.md` | Span↔T、Worker Trace 树、P0 Runbook |
| `design.md` | 六段式详细设计 |
| `dsaft-architecture.md` | Stub — DSAFT 五层计数 |
| `a-registry.md` / `f-registry.md` / `t-registry.md` | A/F/T 登记 SoT |
| `span-registry.md` | Span operation 登记 SoT |
| `d7-boundary.md` | **D4↔D7 跨域 SoT** |
| `layer-delta.md` | V1→V2 演进 Delta |

---

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-14 | 初版：S11–S16 + Hub-Spoke Out of Scope + Legacy 双轨（DM-20260614-018） |
