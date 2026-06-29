# Spec Delta: devrix-d4-dsaft-restructuring (DM-20260629-004)

**Change ID:** devrix-d4-dsaft-restructuring
**Demand ID:** DM-20260629-004
**Status:** S2_Proposal → S3_Design → S4_Implemented → S5_Accepted → S7_Archived

---

## §1 Modified Specs

| Spec | Version | Status |
|------|---------|--------|
| `d4-domain.md` | v2.0.0 → **v2.2.0** | 修订 (ValueFlow Alias + Boundary Debt Decisions) |
| `a-registry.md` | v3.2.0 → **v3.3.0** | 修订 (ValueFlow Alias) |
| `f-registry.md` | v3.1.0 → **v3.2.0** | 修订 (ValueFlow Alias + 18 F path 修正 + Historical S 沉 archive) |
| `t-registry.md` | v3.4.0 → **v3.5.0** | 修订 (ValueFlow Alias + Span Evidence 列 + T-Without-Span Tracker) |
| `span-registry.md` | v2.x → **v3.x** | 6 OpD4_S4_* + 7 EventAgent* const 登记 |
| `layer-delta.md` | v1.x → **v2.x** | §Canonical S → ValueFlow Alias 表 |
| `d7-boundary.md` | v2.x → **v2.x** | 同步 D4 跨域边界 + 3 boundary decision |

---

## §2 Delta — d4-domain.md §North Star

**ADDED** ValueFlow Alias 列 (5 S + 1 横切):

| 可验证承诺 | Canonical S | ValueFlow Alias (用户感知) |
|-----------|-------------|------------------------------|
| 按配额创建 Agent/Worker | D4-S11 ProvisionAgent | `D4_Provision_Agent` |
| Agent 主循环可取消；CRITICAL 工具等权限 | D4-S12 RunAgentLoop | `D4_Run_Agent_Loop` |
| Fork/Worker 不污染父 Session | D4-S13 IsolateAndMerge | `D4_Isolate_Merge` |
| 给定 WorkerSpec 后 fork→run→join | D4-S14 ExecuteWorker | `D4_Execute_Worker` |
| CLI/Cursor 外部 Agent Tool Session 隔离 | D4-S15 InvokeExternalAgent | `D4_External_Agent_Tool` |
| multi_agent 配置加载与校验 | D4-S16 ConfigureAgents | `D4_Configure_Agents` (横切) |

## §3 Delta — d4-domain.md §Boundary Debt Decisions

**ADDED** §Boundary Debt Decisions 章节 (PR-7):

| Boundary ID | 状态 | 内容 | 重新评估 |
|-------------|------|------|----------|
| `boundary-debt:d4-to-d7-agent-event-bridge-v1.0` | ✅ RESOLVED | D4 emit 6 字面量 → D7 FlowEvent | v4.0+ 新增 AgentEvent 类型 |
| `boundary-debt:d4-to-d6-evolution-observer-v1.0` | ✅ RESOLVED | D4 emit 3 字面量 → D6 evolution/guard | D6 增加 fail-fast 维度 |
| `boundary-debt:d4-forbidden-flow-hub-publish-v2.0` | ✅ RESOLVED | D4 禁止 flow.Hub.Publish (D7 v2.0-b lint) | D4 需直接发 FlowEvent 时 |

> 治理常量在 `internal/layers/multiagent/orchtypes/boundary_decision.go`。
> 3 单元测试: `internal/layers/multiagent/orchtypes/boundary_decision_test.go` (Exist + VersionFormat + Unique)。

## §4 Delta — f-registry.md F 路径修正 (18 处)

**CHANGED**:

| F ID | 旧路径 | 新路径 |
|------|--------|--------|
| D4-S11 F01..F05 | `agent/factory.go` + `factory/factory.go` | `provision/factory.go` (含 WorkerEngine inline, PR-1) |
| D4-S12 F01..F04 | `agent/{lifecycle,agent,perm_gate}.go` | `run/{lifecycle,agent,perm_gate}.go` |
| D4-S13 F01..F08 | `agent/{forkjoin,sessionview}.go` + `sessionview/sessionview.go` + `agent/worker_engine.go` | `isolate/{forkjoin,sessionview,worker_engine}.go` (含 sessionview) |
| D4-S14 F01..F10 | `delegate/service.go` | `execute/{service,worker}.go` (Out of Scope, F 路径仅标 D4 owned) |
| D4-S15 F01..F14 | `tool/{registry,cli_adapter,cursor_adapter,stream_json}.go` | `external/{registry,cli_session,cli_execute,cursor_session,cursor_execute,stream_json}.go` (PR-2+PR-3 拆分) |
| D4-S0 F01..F04 | `contracts.go` + `observer/noop.go` | `kernel/{contracts,observer/noop}.go` (PR-1 re-export shim 退役) |

## §5 Delta — t-registry.md Span Evidence 列 (T26)

**ADDED** Span Evidence 列 (31/31 = 100% effective):

- D4-S1 (5): T01/T02/T03/T05 显式 `—` (factory 校验); T04 → `OpD4_S4_Agent_State_Transition`
- D4-S2 (3): T01 → 3 OpD4_S4_* + 3 EventAgent*; T02/T03 → `EventPermissionRequired`
- D4-S3 (8): T01..T08 → 6 OpD4_S4_* + 2 EventAgent* (Fork/Join)
- D4-S4 (2): 显式 `—` (config wiring)
- D4-S5 (1): `EventAgent*` (任意 emit)
- D4-S6 (13): 10 显式 `— (外部 sub-process span)` + 2 显式 `— (parser)` + 1 → `EventPermissionRequired`
- D4-S8 (2): 2 显式 `— (D5 metric，迁 D5)`
- D4-S10 (5): 1 显式 `— (negative test)` + 1 显式 `— (D2 SubQuery)` + 1 显式 `— (D1 bootstrap)` + 2 显式 `— (D7 owns)`
- D4-S12 (1): 1 显式 `— (D7 turn tool_stream)`
- D4-S14 (2): 2 显式 `— (schema validation)`
- D4-S0 (4): 4 → OpD4_S4_Agent_Run/Fork/Join + EventPermissionRequired/Forked/Joined
- D4-FF (4): 4 → OpD4_S4_Agent_Fork/Join + EventAgentForked/Joined

**ADDED** §T-Without-Span Tracker (PR-6, 4 类原因 28 行):
- 外部 sub-process span (10)
- D5/D7 跨域 owns (7)
- config / schema validation (4)
- factory / parser / negative test (7)

**ADDED** §Statistics: 59 total / 59 IMPLEMENTED / 36 P0 / 31 mapped / 28 explicit — / 31/31 = 100% effective。

## §6 Delta — a/f/t-registry + layer-delta.md ValueFlow Alias

**ADDED** ValueFlow Alias block to:
- `a-registry.md` — 5 S section header (D4-S11..S15)
- `f-registry.md` — 5 S 段 (Provision/Run/Isolate/Execute/External)
- `t-registry.md` — §Canonical T 映射块 (5 S + 1 横切 + 1 Cross + 1 Hub-Spoke)
- `layer-delta.md` — §Canonical S → ValueFlow Alias 表

## §7 Delta — span-registry.md

**CHANGED** 6 OpD4_S4_* span ops 登记 (runtime 字面量稳定):
- `OpD4_S4_Agent_Run` (INTERNAL, agent_loop)
- `OpD4_S4_Agent_Tool_Call` (INTERNAL, agent_loop)
- `OpD4_S4_Agent_Fork` (INTERNAL, fork_join)
- `OpD4_S4_Agent_Join` (INTERNAL, fork_join)
- `OpD4_S4_Agent_Terminate` (INTERNAL, agent_loop)
- `OpD4_S4_Agent_State_Transition` (INTERNAL, agent_loop)

**ADDED** 7 EngineEvent 常量 (`orchtypes/events.go`):
- `EventAgentStarted` / `EventAgentError` / `EventAgentTerminated` / `EventAgentIterating`
- `EventAgentForked` / `EventAgentJoined` / `EventPermissionRequired`

**ADDED** T↔Span Evidence 映射表 (与 t-registry 一致性).

## §8 Delta — CI Guard scripts/d4-span-coverage.sh

**ADDED** awk parser 检查 t-registry §2-§8 所有 D4 T 行 Span Evidence 列是否非空非 `—`。守门 ≥80% effective。

实际通过: **31/31 = 100%** PASS (阈值 80%)。

## §9 Delta — legacy S 沉 archive

**MOVED** Historical S 章节 (D4-S1..S10) → `openspec/archive/2026-06-15-devrix-d4-sa-refine/legacy-s1-s10.md` (复用 archive dir)。

t-registry.md §Legacy Archive 表保留追溯映射 (legacy → canonical S)。

## §10 Delta — observability-guide.md (T-Without-Span Tracker)

**ADDED** §T-Without-Span Tracker 章节（与 t-registry §Statistics 联动）+ §Coverage Guard v3.5.0 follow-up。

---

## §11 Delta — orchtypes 包建立 (PR-1)

**ADDED** `internal/layers/multiagent/orchtypes/` 子包:
- `events.go` (NEW): 7 EngineEvent 治理常量
- `boundary_decision.go` (NEW): 3 boundary debt 治理常量 + `AllBoundaryDecisions()`

**RETIRED** `multiagent/contracts.go` re-export shim (47 LOC) — 所有 importer 已迁到 `multiagent/kernel` + `multiagent/orchtypes`。

**REMOVED** 15 死 exported:
- run/metrics.go 整包 (ExecutorMetricsSnapshot + ForkerMetricsSnapshot)
- run/agent.go Creator + run/worker_engine.go WorkerEngine
- provision/factory.go EngineBuilder + provision/builder.go NewAgentFactory
- external/cli_adapter.go CLIAgentTool + CLISession
- external/cursor_adapter.go CursorAgentTool + StreamParseResult
- isolate/fork_sessionview.go ForkSessionViewValue
- isolate/observability.go IncForkSessionViewPolicy + ObservabilitySink + SetObservabilitySink
- run/agent.go CheckFreeForkInvariants

## §12 Delta — god-fn-split (PR-2 + PR-3)

**SPLIT** `external/cli_adapter.go` 466 LOC → `external/cli_session.go` (session 状态/生命周期) + `external/cli_execute.go` (execute 路径)。每个新文件 < 300 LOC。

**SPLIT** `external/cursor_adapter.go` 410 LOC → `external/cursor_session.go` (session) + `external/cursor_execute.go` (execute)。每个新文件 < 300 LOC。

## §13 Delta — 物理路径 (v2.0)

**CHANGED** D4 multiagent/ 目录 v2.0 物理迁移完成:

| Canonical S | scenario-slug | v2.0 实际目录 |
|-------------|---------------|--------------|
| D4-S11 ProvisionAgent | `provision` | `multiagent/provision/`（factory.go, freefork/，含 WorkerEngine inline） |
| D4-S12 RunAgentLoop | `run` | `multiagent/run/`（lifecycle.go, agent.go, state.go, perm_gate.go, forkjoin.go） |
| D4-S13 IsolateAndMerge | `isolate` | `multiagent/isolate/`（sessionview.go, fork_sessionview.go, observer.go） |
| D4-S14 ExecuteWorker | `execute` | `multiagent/execute/`（worker.go, metrics.go, contracts.go） |
| D4-S15 InvokeExternalAgent | `external` | `multiagent/external/`（registry.go, stream_json.go, cli_session.go + cli_execute.go, cursor_session.go + cursor_execute.go） |
| D4-S16 ConfigureAgents | `configure` | `multiagent/configure/`（configure.go） |
| kernel | `kernel` | `multiagent/kernel/`（contracts.go, noop.go） |
| orchtypes | `orchtypes` | `multiagent/orchtypes/`（events.go, boundary_decision.go） |