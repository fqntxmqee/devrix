# D4 Multi-Agent Domain — F 层功能点注册表

**Capability:** architecture-layering
**Status:** Active
**Version:** 2.0.0
**Last Updated:** 2026-06-14
**Parent:** `openspec/specs/architecture/layering.md`
**Depends On:** `openspec/specs/d4-multi-agent/a-registry.md`

---

## Overview

D4 多智能体域 F 层功能点注册表。

---

## D4-S1-A01 CreateAgent

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D4-S1-A01-F01 | NewAgent | F-BE | config, session, deps | *Impl | `agent/agent.go` (New) |
| D4-S1-A01-F02 | CreateWithView | F-BE | config, session_view | *Impl | `factory/factory.go` (CreateWithView) |

## D4-S2-A01 RunAgent

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D4-S2-A01-F01 | ExecuteRun | F-BE | ctx | *AgentResult | `agent/lifecycle.go` (Run / runLoop) |
| D4-S2-A01-F02 | ApplyStateTransition | F-BE | from, to | — | `agent/agent.go` (setState) |
| D4-S2-A01-F03 | TerminateAgent | F-BE | ctx | — | `agent/lifecycle.go` (Terminate) |
| D4-S2-A01-F04 | WaitAgent | F-BE | ctx | *AgentResult | `agent/lifecycle.go` (Wait) |

## D4-S2-A02 ResolvePermission

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D4-S2-A02-F01 | RequestPermission | F-BE | tool_name, risk | decision_ch | `agent/perm_gate.go` (Request) |
| D4-S2-A02-F02 | ResolveDecision | F-BE | tool_name, granted | — | `agent/perm_gate.go` (resolve) |

## D4-S2-A03 WrapWorkerEngine

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D4-S2-A03-F01 | NewWorkerEngine | F-BE | inner_engine, cfg, agent_id | *WorkerEngine | `agent/worker_engine.go` (NewWorkerEngine) |
| D4-S2-A03-F02 | ProcessOverlay | F-BE | session, message | event_ch | `agent/worker_engine.go` (Process) |

## D4-S3-A01 ForkAgent

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D4-S3-A01-F01 | CreateFork | F-BE | child_config | child_agent | `agent/forkjoin.go` (Fork) |
| D4-S3-A01-F02 | ForkSessionView | F-BE | parent_session | child_view | `sessionview/sessionview.go` (Fork) |

## D4-S3-A02 JoinAgent

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D4-S3-A02-F01 | JoinResult | F-BE | child | merged_messages | `agent/forkjoin.go` (Join) |
| D4-S3-A02-F02 | DedupToolCalls | F-BE | messages | deduped | `agent/forkjoin.go` (dedupToolCallMessages) |

## D4-S4-A01 EnhancePrompt

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D4-S4-A01-F01 | ValidateMode | F-BE | mode | error | `collaboration/mode.go` (ValidateMode) |
| D4-S4-A01-F02 | BuildPromptForMode | F-BE | base, mode | enhanced | `collaboration/prompt.go` (BuildPromptForMode) |

## D4-S5-A01 BridgeAgentEvents

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D4-S5-A01-F01 | EmitAgentEvent | F-BE | *AgentEvent | — | `contracts.go` (AgentObserverChain) |
| D4-S5-A01-F02 | NoOpObserver | F-BE | *AgentEvent | — | `observer/noop.go` (NoOpAgentObserver) |

## D4-S6-A01 RegisterAgentTool

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D4-S6-A01-F01 | RegisterTool | F-BE | tool_spec | — | `tool/registry.go` (Register) |
| D4-S6-A01-F02 | LookupTool | F-BE | name | Info | `tool/registry.go` (Get / List) |

## D4-S6-A02 ExecuteAgentTool

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D4-S6-A02-F01 | ExecuteCLI | F-BE | ctx, req | event_ch | `tool/cli_adapter.go` (Execute) |
| D4-S6-A02-F02 | ExecuteCursor | F-BE | ctx, req | event_ch | `tool/cursor_adapter.go` (Execute) |
| D4-S6-A02-F03 | ManageSession | F-BE | session_id | — | `tool/cli_adapter.go` (ensureSession / CloseSession) |

## D4-S6-A03 ParseStreamOutput

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D4-S6-A03-F01 | ParseStreamJSONLine | F-BE | stdout_line | StreamParseResult | `tool/stream_json.go` (ParseStreamJSONLine) |

## D4-S7-A01 RunBuiltinAgent

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D4-S7-A01-F01 | RunExplore | F-BE | deps, prompt, tools | *SubQueryResult | `builtin/agents.go` (RunExplore) |
| D4-S7-A01-F02 | RunPlan | F-BE | deps, prompt, tools | *SubQueryResult | `builtin/agents.go` (RunPlan) |
| D4-S7-A01-F03 | RunImplement | F-BE | deps, prompt, tools | *SubQueryResult | `builtin/agents.go` (RunImplement) |

## D4-S8-A01 RecordForkPolicyMetrics

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D4-S8-A01-F01 | IncForkSessionView | F-BE | policy_label | — | `observability/metrics.go` (IncForkSessionView) |
| D4-S8-A01-F02 | SetD5Sink | F-BE | sink | — | `observability/metrics.go` (SetD5Sink) |

## D4-S9-A01 ManageSessionView

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D4-S9-A01-F01 | SetMetadata | F-BE | key, value | — | `sessionview/sessionview.go` (SetMetadata) |
| D4-S9-A01-F02 | SetSnapshot | F-BE | snap_bytes | — | `sessionview/sessionview.go` (SetSnapshot) |
| D4-S9-A01-F03 | MergeToParent | F-BE | parent_session | — | `sessionview/sessionview.go` (MergeToParent) |

## D4-S10-A01 DelegateTask

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D4-S10-A01-F01 | DelegateSync | F-BE | leader, spec | *DelegateResult | `delegate/service.go` (DelegateSync) |
| D4-S10-A01-F02 | DelegateAsync | F-BE | leader, spec | task_id | `delegate/service.go` (DelegateAsync) |
| D4-S10-A01-F03 | DelegateOrFallback | F-BE | leader, spec | DelegateResult | `delegate/service.go` (DelegateOrFallback) |

## D4-S10-A02 BridgeFlowEvents

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D4-S10-A02-F01 | EmitAgentEvent | F-BE | *AgentEvent | — | `delegate/bridge.go` (EmitAgentEvent) |
| D4-S10-A02-F02 | EngineEventSink | F-BE | *EngineEvent | — | `delegate/bridge.go` (EngineEventSink) |

---

## Statistics

| Activities with F | Total F Points |
|-------------------|----------------|
| 16 | 37 |

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-13 | Initial F registry (6 activities, 14 F points) |
| 2.0.0 | 2026-06-14 | Fixed D4-S3-A01-F02 (BuildForkedMessages→ForkSessionView), added D4-S2-A02/A03, D4-S5, D4-S6-A01/A02/A03, D4-S7, D4-S8, D4-S9 F points; added WaitAgent, DelegateOrFallback; 16 activities, 37 F points |
