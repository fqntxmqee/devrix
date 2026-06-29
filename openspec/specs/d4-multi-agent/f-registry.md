# D4 Multi-Agent Domain — F 层功能点注册表

**Capability:** architecture-layering
**Status:** Active
**Version:** 3.2.0
**Last Updated:** 2026-06-30
**Change ID:** devrix-d4-dsaft-restructuring
**Demand ID:** DM-20260629-004
**Parent:** `openspec/specs/architecture/layering.md`
**Depends On:** `openspec/specs/d4-multi-agent/a-registry.md`

---

## Overview

D4 多智能体域 F 层功能点注册表。Canonical S11–S16 为 SoT；Legacy S1–S10 冻结追溯。

---

## Canonical F 层（SoT）— D4-S11–S16

> F 编号与 Canonical A 1:1 或按机制拆分；Hub-Spoke 编排 F 迁 D7（Out of Scope）。
>
> **ValueFlow Alias (用户感知)**（DM-20260629-004 PR-5 value-flow-rename）：每个 S 段 header 加 `> **ValueFlow Alias (用户感知):** D4_*` 行；与 a-registry.md / d4-domain.md 同步。

### D4-S11 ProvisionAgent

> **ValueFlow Alias (用户感知):** `D4_Provision_Agent`

| F ID | Name | Type | Input | Output | Legacy 映射 | Code Location |
|------|------|------|-------|--------|-------------|---------------|
| D4-S11-A01-F01 | NewAgent | F-BE | config, session, deps | *Impl | S1-F01 | `provision/factory.go` (NewAgentFactory / Create / CreateWithView) |
| D4-S11-A01-F02 | CreateWithView | F-BE | config, session_view | *Impl | S1-F02 | `provision/factory.go` (CreateWithView) |
| D4-S11-A02-F01 | ValidateMode | F-BE | mode | error | S4-F01 | `collaboration/mode.go` (ValidateMode) |
| D4-S11-A02-F02 | BuildPromptForMode | F-BE | base, mode | enhanced | S4-F02 | `collaboration/prompt.go` (BuildPromptForMode) |
| D4-S11-A03-F01 | RunExplore | F-BE | deps, prompt, tools | *SubQueryResult | S7-F01 | `orchestration/delegatetools/builtin_agents.go` (RunExplore) |
| D4-S11-A03-F02 | RunPlan | F-BE | deps, prompt, tools | *SubQueryResult | S7-F02 | `orchestration/delegatetools/builtin_agents.go` (RunPlan) |
| D4-S11-A03-F03 | RunImplement | F-BE | deps, prompt, tools | *SubQueryResult | S7-F03 | `orchestration/delegatetools/builtin_agents.go` (RunImplement) |

### D4-S12 RunAgentLoop

> **ValueFlow Alias (用户感知):** `D4_Run_Agent_Loop`

| F ID | Name | Type | Input | Output | Legacy 映射 | Code Location |
|------|------|------|-------|--------|-------------|---------------|
| D4-S12-A01-F01 | ExecuteRun | F-BE | ctx | *AgentResult | S2-F01 | `run/lifecycle.go` (Run / runLoop) |
| D4-S12-A01-F02 | ApplyStateTransition | F-BE | from, to | — | S2-F02 | `run/agent.go` (setState) |
| D4-S12-A01-F03 | TerminateAgent | F-BE | ctx | — | S2-F03 | `run/lifecycle.go` (Terminate) |
| D4-S12-A01-F04 | WaitAgent | F-BE | ctx | *AgentResult | S2-F04 | `run/lifecycle.go` (Wait) |
| D4-S12-A02-F01 | RequestPermission | F-BE | tool_name, risk | decision_ch | S2-F05 | `run/perm_gate.go` (Request) |
| D4-S12-A02-F02 | ResolveDecision | F-BE | tool_name, granted | — | S2-F06 | `run/perm_gate.go` (resolve) |

### D4-S13 IsolateAndMerge

> **ValueFlow Alias (用户感知):** `D4_Isolate_Merge`

| F ID | Name | Type | Input | Output | Legacy 映射 | Code Location |
|------|------|------|-------|--------|-------------|---------------|
| D4-S13-A01-F01 | CreateFork | F-BE | child_config | child_agent | S3-F01 | `run/forkjoin.go` (Fork) |
| D4-S13-A01-F02 | JoinResult | F-BE | child | merged_messages | S3-F03 | `run/forkjoin.go` (Join) |
| D4-S13-A01-F03 | DedupToolCalls | F-BE | messages | deduped | S3-F04 | `run/forkjoin.go` (dedupToolCallMessages) |
| D4-S13-A02-F01 | ForkSessionView | F-BE | parent_session | child_view | S3-F02 | `isolate/sessionview.go` (Fork) |
| D4-S13-A02-F02 | SetMetadata | F-BE | key, value | — | S9-F01 | `isolate/sessionview.go` (SetMetadata) |
| D4-S13-A02-F03 | SetSnapshot | F-BE | snap_bytes | — | S9-F02 | `isolate/sessionview.go` (SetSnapshot) |
| D4-S13-A02-F04 | MergeToParent | F-BE | parent_session | — | S9-F03 | `isolate/sessionview.go` (MergeToParent) |
| D4-S13-A03-F01 | NewWorkerEngine | F-BE | inner_engine, cfg, agent_id | *WorkerEngine | S2-F07 | `provision/factory.go`（PR-1 #0 已 inline 为 `newWorkerEngine`） |
| D4-S13-A03-F02 | ProcessOverlay | F-BE | session, message | event_ch | S2-F08 | `provision/factory.go`（PR-1 #0 已 inline 为 `workerEngine.Process`） |

### D4-S14 ExecuteWorker

> **ValueFlow Alias (用户感知):** `D4_Execute_Worker`

| F ID | Name | Type | Input | Output | Legacy 映射 | Code Location |
|------|------|------|-------|--------|-------------|---------------|
| D4-S14-A01-F01 | ExecuteWorkerSync | F-BE | leader, spec | *DelegateResult | S10-F01 | `execute/worker.go` (WorkerExecutor.Execute) |
| D4-S14-A01-F02 | ExecuteWorkerAsync | F-BE | leader, spec | task_id | S10-F02 | `execute/worker.go` (WorkerExecutor.ExecuteAsync) |

> **迁 D7（Out of Scope）：** `DelegateOrFallback` → D7-S2-A04-F01；`EmitAgentEvent` / `EngineEventSink` → D7-S4-A04/A05。

### D4-S15 InvokeExternalAgent

> **ValueFlow Alias (用户感知):** `D4_External_Agent_Tool`

| F ID | Name | Type | Input | Output | Legacy 映射 | Code Location |
|------|------|------|-------|--------|-------------|---------------|
| D4-S15-A01-F01 | RegisterTool | F-BE | tool_spec | — | S6-F01 | `external/registry.go` (Register) |
| D4-S15-A01-F02 | LookupTool | F-BE | name | Info | S6-F02 | `external/registry.go` (Get / List) |
| D4-S15-A02-F01 | ExecuteCLI | F-BE | ctx, req | event_ch | S6-F03 | `external/cli_execute.go` (Execute) |
| D4-S15-A02-F02 | ExecuteCursor | F-BE | ctx, req | event_ch | S6-F04 | `external/cursor_execute.go` (Execute) |
| D4-S15-A02-F03 | ManageSession | F-BE | session_id | — | S6-F05 | `external/cli_session.go` (ensureSession / CloseSession) + `cursor_session.go` (chatID tracking) |
| D4-S15-A03-F01 | ParseStreamJSONLine | F-BE | stdout_line | StreamParseResult | S6-F06 | `external/stream_json.go` (ParseStreamJSONLine) |

### D4-S16 ConfigureAgents

> **ValueFlow Alias (用户感知):** `D4_Configure_Agents`

| F ID | Name | Type | Input | Output | Legacy 映射 | Code Location |
|------|------|------|-------|--------|-------------|---------------|
| D4-S16-A01-F01 | LoadMultiAgentConfig | F-BE | yaml | MultiAgentConfig | config | `shared/config/multiagent.go` |

### D4-S5 BridgeAgentEvents（kernel，横切）

| F ID | Name | Type | Input | Output | Legacy 映射 | Code Location |
|------|------|------|-------|--------|-------------|---------------|
| D4-S5-A01-F01 | EmitAgentEvent | F-BE | *AgentEvent | — | S5-F01 | `kernel/contracts.go` (AgentObserverChain) |
| D4-S5-A01-F02 | NoOpObserver | F-BE | *AgentEvent | — | S5-F02 | `kernel/noop.go` (NoOpAgentObserver) |

### D4-S8 RecordForkPolicyMetrics（v1.1 迁 D5）

| F ID | Name | Type | Input | Output | Legacy 映射 | Code Location |
|------|------|------|-------|--------|-------------|---------------|
| D4-S8-A01-F01 | IncForkSessionView | F-BE | policy_label | — | S8-F01 | `internal/layers/observability/instrument/metrics/` (IncForkSessionView) |
| D4-S8-A01-F02 | SetObservabilitySink | F-BE | sink | — | S8-F02 | `internal/layers/observability/instrument/metrics/` (SetObservabilitySink) |

---

## Legacy F 层（冻结追溯）

> DM-20260629-004 PR-4 #2 registry-sync：D4-S1~S10 详细 F 表已下沉到 `openspec/archive/2026-06-15-devrix-d4-sa-refine/legacy-s1-s10.md`。本域概览见 `d4-domain.md §Legacy Module Index`。

---

## Statistics

| Track | Activities | F Points |
|-------|------------|----------|
| Canonical S11–S16 | 13 A | 22 F |
| Migrated to D7（Out of Scope） | 2 A | 3 F |
| Legacy S1–S10（frozen） | 17 A | 39 F (archived) |

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| 3.2.0 | 2026-06-30 | DM-20260629-004 PR-5 #3 value-flow-rename：§S→A→F 索引每个 S header 加 `> **ValueFlow Alias (用户感知):**` 行（5 S + 1 横切 = 6 alias：`D4_Provision_Agent` / `D4_Run_Agent_Loop` / `D4_Isolate_Merge` / `D4_Execute_Worker` / `D4_External_Agent_Tool` / `D4_Configure_Agents`）；与 d4-domain.md §North Star + a-registry.md §ValueFlow Alias 总览对齐 |
| 3.0.0 | 2026-06-14 | Canonical S11–S16 F 表 + Legacy 冻结；Hub-Spoke F 标 Out of Scope（DM-20260614-018） |
| 2.0.0 | 2026-06-14 | Fixed D4-S3-A01-F02 (BuildForkedMessages→ForkSessionView), added D4-S2-A02/A03, D4-S5, D4-S6-A01/A02/A03, D4-S7, D4-S8, D4-S9 F points; added WaitAgent, DelegateOrFallback; 16 activities, 37 F points |
| 1.0.0 | 2026-06-13 | Initial F registry (6 activities, 14 F points) |
