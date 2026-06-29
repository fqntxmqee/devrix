# D4 Multi-Agent Domain

**Domain ID:** D4
**Slug:** `multiagent`
**Type:** Core Domain
**Status:** Active — Canonical S11–S16 (v2.0 registry, DM-20260629-004 PR-4)
**Depends On:** D2 (`IEngine`), D1 (`PermissionManager`), D5 (tracer emit)
**Depended By:** D7 (`WorkerExecutor` consumer)
**Cross-Domain SoT:** `d7-boundary.md`
**Change:** `openspec/changes/devrix-d4-dsaft-restructuring/`

---

## North Star

**在 D7 给定 Worker 派发参数后，可靠地供给 Agent 实例、执行隔离的子任务循环、合并结果——作为 Delegation Execution Follower，不承担 Hub-Spoke 编排与 Flow 发布。**

| 可验证承诺 | Canonical S | ValueFlow Alias (用户感知) |
|-----------|-------------|---------------------------|
| 按配额创建 Agent/Worker，注入协作模式 | D4-S11 ProvisionAgent | `D4_Provision_Agent` |
| Agent 主循环可取消；CRITICAL 工具等权限 | D4-S12 RunAgentLoop | `D4_Run_Agent_Loop` |
| Fork/Worker 执行不污染父 Session | D4-S13 IsolateAndMerge | `D4_Isolate_Merge` |
| 给定 WorkerSpec 后 fork→run→join | D4-S14 ExecuteWorker | `D4_Execute_Worker` |
| CLI/Cursor 外部 Agent Tool Session 隔离 | D4-S15 InvokeExternalAgent | `D4_External_Agent_Tool` |
| multi_agent 配置加载与校验 | D4-S16 ConfigureAgents | `D4_Configure_Agents` |

---

## Out of Scope

| 能力 | 归属 | 备注 |
|------|------|------|
| Hub-Spoke 路由 / delegate_* 工具 | D7-S2 | `delegatetools/` |
| Spoke 派发矩阵 / fallback 决策 | D7-S2 | v2.0 `hubspoke/dispatch.go` |
| FlowEvent 发布 / FlowBridge | D7-S4 | v2.0 自 D4/D2 迁入 |
| WorkPlan / delegate-progress drain | D7-S4 | `flow/`, `executionflow/` (formerly `sessionqueue/`) |
| SubQuery Flow 发布 | D7-S4 | v2.0 自 D2 `flow_report` 迁入 |
| 权限 UI | D1 | Gateway 注入 ResolvePermission |
| Span/Metric 定义 | D5 | D4 仅 emit hook |
| 结论质量 Judge | D6 | — |
| LLM 调用 | D3 | 经 D2 QueryLoop |

---

## DSAFT 双轨

### Canonical 价值流（SoT）— D4-S11–S16

| S ID | Scenario | Responsibility | ValueFlow Alias (用户感知) | Status |
|------|----------|----------------|---------------------------|--------|
| D4-S11 | ProvisionAgent | 创建、配额、协作模式 prompt、Builtin 注册 | `D4_Provision_Agent` | REGISTRY |
| D4-S12 | RunAgentLoop | 生命周期、PermissionGate、状态机 | `D4_Run_Agent_Loop` | REGISTRY |
| D4-S13 | IsolateAndMerge | Fork/Join、SessionView COW、WorkerEngine overlay | `D4_Isolate_Merge` | REGISTRY |
| D4-S14 | ExecuteWorker | Worker fork→run→join（D7 派发） | `D4_Execute_Worker` | REGISTRY |
| D4-S15 | InvokeExternalAgent | CLI/Cursor Agent Tool | `D4_External_Agent_Tool` | REGISTRY |
| D4-S16 | ConfigureAgents | multi_agent 配置（横切） | `D4_Configure_Agents` | REGISTRY |

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

## Boundary Debt Decisions

> DM-20260629-004 PR-7 #5 boundary-decision — 3 项 D4 跨域边界债务审计。所有 3 项均 **RESOLVED**（PR-6 常量化后 emit 模式稳定 + 消费者路由可验证）。治理常量见 `internal/layers/multiagent/orchtypes/boundary_decision.go`，与 D2/D3/D7 `boundary-debt:` 命名空间一致。

| ID | Debt | Status | Resolution | Governance Constant | 重新评估触发 |
|----|------|--------|------------|---------------------|--------------|
| `boundary-debt:d4-to-d7-agent-event-bridge-v1.0` | D4 emit `agent.{started,error,terminated,iterating,forked,joined}` 6 字面量 → D7 FlowEvent（订阅侧） | **RESOLVED** | PR-6 #4 span-coverage：6 字面量常量化 `orchtypes.EventAgent*`，消费者 `agent_bridge.go` const switch | `orchtypes.BoundaryD4ToD7AgentEventBridge` | v4.0+ 新增 AgentEvent 类型（如 `agent.paused`）需重审 |
| `boundary-debt:d4-to-d6-evolution-observer-v1.0` | D4 emit `agent.{forked,joined}` + `permission_required` → D6 evolution/guard/observer（fail-fast + reputation） | **RESOLVED** | PR-6 #4 span-coverage：3 字面量常量化 `orchtypes.EventAgentForked/Joined/EventPermissionRequired`，消费者 `observer.go` const switch | `orchtypes.BoundaryD4ToD6EvolutionObserver` | D6 增加 fail-fast 维度（如 `agent.quota_exceeded`）需重审 |
| `boundary-debt:d4-forbidden-flow-hub-publish-v2.0` | D4 跨域 emit 必须走 const switch，**禁止** `flow.Hub.Publish`（D7 v2.0-b 后 lint 强制） | **RESOLVED** | PR-6 #4 span-coverage：跨域 emit 收敛到 `orchtypes.EventAgent*`；`scripts/d4-span-coverage.sh` 守门；layout guard `internal/lint/layer/` 维持 `D4 forbidden flow.Hub.Publish` | `orchtypes.BoundaryD4ForbiddenFlowHubPublish` | D4 需直接发 FlowEvent 时必须先升级 spec |

**格式约定**：`^boundary-debt:[a-z0-9\-]+-v\d+\.\d+$`（与 D2/D3/D7 命名空间一致；版本记录**决议时间**而非债务发生时）。
**唯一性**：3 项决策字符串全局唯一（`orchtypes.AllBoundaryDecisions()` 在 `boundary_decision_test.go` 中守门）。
**重新评估触发**：每次 devrix-d4-* Change 启动 S3-Gate 时，须先 `grep -r 'boundary-debt:' openspec/specs/d4-multi-agent/` 检查是否有命中上述 ID；如命中且新需求冲突，回退到 OPEN 状态并补 plan。

---

## 物理路径（v2.0 目标）

| Canonical S | scenario-slug | v2.0 实际 |
|-------------|---------------|----------|
| S11 | `provision` | `multiagent/provision/`（factory.go, freefork/，含 WorkerEngine inline） |
| S12 | `run` | `multiagent/run/`（lifecycle.go, agent.go, state.go, perm_gate.go, forkjoin.go） |
| S13 | `isolate` | `multiagent/isolate/`（sessionview.go） |
| S14 | `execute` | `multiagent/execute/`（worker.go, metrics.go, contracts.go） |
| S15 | `external` | `multiagent/external/`（registry.go, stream_json.go, cli_session.go + cli_execute.go, cursor_session.go + cursor_execute.go） |
| S16 | `configure` | `multiagent/configure/`（configure.go） |
| kernel | `kernel` | `multiagent/kernel/`（contracts.go, noop.go） |

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
| 2.2.0 | 2026-06-30 | DM-20260629-004 PR-7 #5 boundary-decision：新增 §Boundary Debt Decisions 章节（3 项 RESOLVED）+ 治理常量 `orchtypes.BoundaryD4*` 引用 + 格式/唯一性 lint 守门 |
| 2.1.0 | 2026-06-30 | DM-20260629-004 PR-5 #3 value-flow-rename：§North Star + §Canonical 价值流加 ValueFlow Alias 列（5 S + 1 横切 = 6 alias：`D4_Provision_Agent` / `D4_Run_Agent_Loop` / `D4_Isolate_Merge` / `D4_Execute_Worker` / `D4_External_Agent_Tool` / `D4_Configure_Agents`） |
| 2.0.0 | 2026-06-30 | DM-20260629-004 PR-4 #2 registry-sync：物理路径表对齐 code；version bump 1.0→2.0 |
| 1.0.0 | 2026-06-14 | 初版：S11–S16 + Hub-Spoke Out of Scope + Legacy 双轨（DM-20260614-018） |
