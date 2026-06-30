# D4 Multi-Agent Layer Specification

**Capability:** multi-agent
**Status:** Active
**Version:** 3.1.0 (V3 S11–S16 价值流)
**Last Updated:** 2026-06-30 (DM-20260629-004 d4-dsaft-restructuring v3.1.0 S7_Archived)
**Domain SoT:** `d4-domain.md` v2.2.0 — North Star + 6 ValueFlow + DSAFT 资产 + 边界 SoT
**D7 Boundary:** `d7-boundary.md` — D4↔D7 跨域边界规范 + Boundary Debt Decisions

> **精简设计契约（Lite-Mode）**：本文档只放当前符合代码的设计契约（v3.1.0）。**过程需求迭代**（如 devrix-context-budget-phase-b "Sub-Agent Mode Field" 详细 Gherkin）不进入本文件，留在 `archive/<change>/specs/` 各 change 归档目录。详细时间线见 [CHANGELOG.md](CHANGELOG.md)。

---

## Overview

D4 多智能体域是 **Delegation Execution Follower**：供给 Agent/Worker 实例、运行主循环、隔离合并、执行 D7 派发的 Worker。**Hub-Spoke 编排、FlowEvent 发布、WorkPlan 聚合归 D7**（R1 D7-1，见 `d7-boundary.md`）。D4 仅承担 Agent 执行面（不拥有 Hub 决策 / 不 Publish FlowEvent）。

| 承诺 | Canonical S | ValueFlow Alias | 验证入口 |
|------|-------------|-----------------|----------|
| 按配额创建 Agent/Worker，注入协作模式 | D4-S11 ProvisionAgent | `D4_Provision_Agent` | `D4-S11-A01-T01~T04` |
| Agent 主循环可取消；CRITICAL 工具等权限 | D4-S12 RunAgentLoop | `D4_Run_Agent_Loop` | `D4-S12-A02-T01~T10` |
| Fork/Worker 执行不污染父 Session | D4-S13 IsolateAndMerge | `D4_Isolate_Merge` | `D4-S13-A03-T01~T08` |
| 给定 WorkerSpec 后 fork→run→join | D4-S14 ExecuteWorker | `D4_Execute_Worker` | `D4-S14-A04-T01~T12` |
| CLI/Cursor 外部 Agent Tool Session 隔离 | D4-S15 InvokeExternalAgent | `D4_External_Agent_Tool` | `D4-S15-A05-T01/T02` |
| multi_agent 配置加载与校验 | D4-S16 ConfigureAgents | `D4_Configure_Agents` | `D4-S16-A06-T01` |

### 核心设计原则

1. **Delegation Execution Follower**（D4 不拥有 Hub-Spoke 编排）：Hub-Spoke 路由、FlowEvent 发布、WorkPlan 聚合均归 D7（DM-20260614-018）
2. **canonical S = 6 + Legacy S1-S10 双轨**：S11-S16 价值流为 v2.0 registry；S1-S10 Legacy 冻结追溯（v1.0 runtime 仍映射 Legacy 路径）
3. **SessionView COW 隔离**：Fork/Worker 通过 `SessionView` COW 不污染父 Session metadata（R1）
4. **Permission Gate 前置**：CRITICAL 工具 MUST 经 `PermissionManager`（D1 注入），user_deny → TERMINATED（R1）
5. **Worker 不能 delegate**：Worker MUST NOT delegate or fork（D4-S14 hard constraint）
6. **AgentEvent emit const switch**：跨域 emit 收敛到 `orchtypes.EventAgent*`，**禁止** `flow.Hub.Publish`（`scripts/d4-span-coverage.sh` CI 守门）
7. **multi_agent 启动 fail-fast**：`ConfigureAgents` 配置加载失败 → `ErrMultiAgentConfigInvalid`（R3）
8. **Sub-Agent Mode 3-mode dispatch**：`delegate_*` / `free_fork` 输入 `mode ∈ {brief, fork, full}`，default brief（DM-20260620-001-B Phase B；详细 Gherkin 在 `archive/2026-06-20-devrix-context-budget-phase-b/`）

### S 层职责（canonical D4-S11..S16）

| S ID | Scenario | 职责 | Status |
|------|----------|------|--------|
| D4-S11 | ProvisionAgent | 创建、配额、协作模式 prompt、Builtin 注册 | **REGISTRY** |
| D4-S12 | RunAgentLoop | 生命周期、PermissionGate、状态机 | **REGISTRY** |
| D4-S13 | IsolateAndMerge | Fork/Join、SessionView COW、WorkerEngine overlay | **REGISTRY** |
| D4-S14 | ExecuteWorker | Worker fork→run→join（D7 派发） | **REGISTRY** |
| D4-S15 | InvokeExternalAgent | CLI/Cursor Agent Tool（Session 隔离） | **REGISTRY** |
| D4-S16 | ConfigureAgents | multi_agent 配置加载与校验（横切） | **REGISTRY** |

**Legacy 双轨（D4-S1..S10）**：冻结追溯，详见 `archive/2026-06-15-devrix-d4-sa-refine/legacy-s1-s10.md`（D4-S1 Factory → S11 / S2 Agent → S12-S13 / S3 ForkJoin → S13 / S4 Collaboration → S11 / S5 Observer → kernel / S6 AgentTool → S15 / S7 Builtin → S11 + D7 fallback / S8 Observability → D5 / S9 SessionView → S13 / S10 Delegate → S14 执行 + D7-S2/S4 编排）

---

## DSAFT 结构

| 层级 | ID | 名称 | 物理路径 / SoT |
|------|----|------|----------------|
| D | D4 | Multi-Agent | `internal/layers/multiagent/` |
| S | D4-S11 | Provision Agent | `multiagent/provision/` (factory.go + freefork/ + WorkerEngine inline) |
| S | D4-S12 | Run Agent Loop | `multiagent/run/` (lifecycle.go + agent.go + state.go + perm_gate.go + forkjoin.go) |
| S | D4-S13 | Isolate Merge | `multiagent/isolate/` (sessionview.go) + `multiagent/run/forkjoin.go` |
| S | D4-S14 | Execute Worker | `multiagent/execute/` (worker.go + metrics.go + contracts.go) |
| S | D4-S15 | External Agent | `multiagent/external/` (registry.go + stream_json.go + cli/cursor session/execute) |
| S | D4-S16 | Configure | `multiagent/configure/` (configure.go) + `shared/config/multiagent.go` |
| kernel | — | contracts + noop | `multiagent/kernel/` + `multiagent/orchtypes/` |
| A | A1-A99 | 6 个核心活动（每 S 1 A） | 见 `a-registry.md` |
| F | F1-F999 | 域内 F | 见 `f-registry.md` |
| T | T1-T200 | 38 IMPLEMENTED（Legacy Archive） | 见 `t-registry.md` |
| Span | — | S8 迁 D5 | 见 `span-registry.md` |

**当前计数（v3.1.0）**：D=1, S=6 (canonical: S11-S16) + S=10 (Legacy tombstone), A=6, F=域内, T=38, Span=5 ops。

---

## Scenarios

| ID | Scenario | Responsibility | Status | 验证入口 |
|----|----------|----------------|--------|----------|
| D4-S11 | ProvisionAgent | AgentFactory.Create + quota + 协作模式 prompt 增强 | **REGISTRY** | `D4-S11-A01-T01~T04` |
| D4-S12 | RunAgentLoop | 状态机 + PermissionGate + user_deny → TERMINATED | **REGISTRY** | `D4-S12-A02-T01~T10` |
| D4-S13 | IsolateAndMerge | SessionView COW + Fork/Worker 不污染父 metadata | **REGISTRY** | `D4-S13-A03-T01~T08` |
| D4-S14 | ExecuteWorker | D7 DispatchWorker → fork→run→join；Worker cannot delegate | **REGISTRY** | `D4-S14-A04-T01~T12` |
| D4-S15 | InvokeExternalAgent | CLI/Cursor subprocess per session（R1 d4-to-d7 hard constraint） | **REGISTRY** | `D4-S15-A05-T01/T02` |
| D4-S16 | ConfigureAgents | multi_agent 配置加载 + 启动 fail-fast | **REGISTRY** | `D4-S16-A06-T01` |

---

## Architecture

```
D7 DispatchWorker → SpokeBridge.Publish (D7 only)
    └─ D4-S14 ExecuteWorker (Worker fork→run→join)
        ├─ D4-S11 ProvisionAgent (fork 前)
        ├─ D4-S12 RunAgentLoop (Worker 循环 + PermissionGate)
        ├─ D4-S13 IsolateAndMerge (COW + SessionView)
        └─ D4 emits orchtypes.EventAgent* (D7 订阅侧 → FlowEvent)

D4 独立面：
  D4-S15 InvokeExternalAgent (CLI/Cursor subprocess)
  D4-S16 ConfigureAgents
  kernel: contracts + Observer (v2.0)
```

### 域边界

| D4 拥有 | D4 调用（不拥有） | D4 不拥有 |
|---------|------------------|----------|
| Agent/Worker 实例 + 主循环 | D7 DispatchWorker（消费者） | Hub-Spoke 路由 / delegate_* 工具（D7-S2） |
| Permission Gate + CRITICAL check | — | FlowEvent 发布 / FlowBridge（D7-S4） |
| SessionView COW + Fork/Join | D2 QueryLoop 经 SubQuery | WorkPlan / delegate-progress drain（D7-S4） |
| multi_agent 配置加载 + fail-fast | D5 observability emit | 权限 UI（D1） |
| External Agent subprocess | D1 PermissionManager | Span/Metric 定义（D5） |
| AgentEvent emit (const switch) | — | 结论质量 Judge（D6） |

**Hard Ban**：
- D4 import `orchestration/flow` 做 `hub.Publish`（v2.0-b 后 `scripts/d4-span-coverage.sh` lint 强制）
- Worker delegate or fork（D4-S14 hard constraint）

**Boundary Debt**（3 项 RESOLVED，治理常量 in `orchtypes/boundary_decision.go`）：
- `boundary-debt:d4-to-d7-agent-event-bridge-v1.0` — D4 emit `agent.*` 6 字面量 → D7 FlowEvent（订阅侧）
- `boundary-debt:d4-to-d6-evolution-observer-v1.0` — D4 emit `agent.{forked,joined}` + `permission_required` → D6 evolution/guard
- `boundary-debt:d4-forbidden-flow-hub-publish-v2.0` — D4 禁止 `flow.Hub.Publish`

---

## 关键 Scenario 范式

### 范式：D4-S14 ExecuteWorker happy 路径（DSAFT S14-A04）

#### Scenario: D7 派发 WorkerSpec 后 Worker fork→run→join

- **GIVEN** D7 DispatchWorker Spoke=D4Worker + WorkerSpec
- **WHEN** D4-S14 ExecuteWorker.forkRunJoin(spec)
- **THEN** Worker 实例创建（fork）+ run 主循环（run）+ 结果合并（join）
- **AND** Worker MUST NOT delegate or fork（hard constraint）
- **AND** emit `orchtypes.EventAgentForked` + `EventAgentJoined` 经 D7 FlowEvent 订阅侧

---

## 关键链路口

1. **主链**：D7-S2 DispatchWorker → D4-S14 ExecuteWorker → S11 ProvisionAgent + S12 RunAgentLoop + S13 IsolateAndMerge
2. **跨域事件链**：D4 emit `orchtypes.EventAgent*`（6 字面量）→ D7 Bridge consumer（const switch）→ FlowEvent
3. **Observe 链**：D4 emit `agent.{forked,joined}` + `permission_required` → D6 evolution/guard/observer（fail-fast + reputation）
4. **External Agent 链**：D4-S15 CLI/Cursor subprocess ↔ D1 session 隔离（每 session 独立 subprocess）
5. **配置加载链**：D4-S16 ConfigureAgents → `shared/config/multiagent.go` + 启动 fail-fast
6. **Hard Ban 链**：D4 `flow.Hub.Publish` = 0（`scripts/d4-span-coverage.sh` 守门）+ Worker delegate = 0

---

## 附录：总览

- **当前活跃 Requirement 数**：6 canonical（每段 1 句 + 1 canonical Gherkin，详见 archive 详细文本）
- **历史 Requirement 详细文本**：在 `archive/<change>/specs/` 各 change 目录（详见 CHANGELOG.md）
- **当前 spec 版本**：v3.1.0
- **下一次架构级变更触发**：D4 域升级 v4.0+ 或 Hub-Spoke 跨域契约变化时重新审计 Boundary Debt Decisions
