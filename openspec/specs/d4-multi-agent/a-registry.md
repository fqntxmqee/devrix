# D4 Multi-Agent Domain — A 层活动注册表

**Capability:** architecture-layering
**Status:** Active
**Version:** 3.0.0
**Last Updated:** 2026-06-14
**Change ID:** devrix-d4-sa-refine
**Parent:** `openspec/specs/architecture/layering.md`

---

## Canonical A 层（SoT）— D4-S11–S16

| A ID | Name | Canonical S | Legacy 映射 | Code（v1.0） |
|------|------|-------------|-------------|-------------|
| D4-S11-A01 | CreateAgent | S11 | S1-A01 | `factory/factory.go` |
| D4-S11-A02 | EnhancePrompt | S11 | S4-A01 | `collaboration/prompt.go` |
| D4-S11-A03 | RegisterBuiltin | S11 | S7-A01 | `builtin/agents.go` |
| D4-S12-A01 | RunAgentLoop | S12 | S2-A01 | `agent/lifecycle.go` |
| D4-S12-A02 | ResolvePermission | S12 | S2-A02 | `agent/perm_gate.go` |
| D4-S13-A01 | ForkAndJoin | S13 | S3-A01/A02 | `agent/forkjoin.go` |
| D4-S13-A02 | ManageSessionView | S13 | S9-A01 | `sessionview/sessionview.go` |
| D4-S13-A03 | WrapWorkerEngine | S13 | S2-A03 | `agent/worker_engine.go` |
| D4-S14-A01 | ExecuteWorker | S14 | S10-A01（执行） | `delegate/service.go` |
| D4-S15-A01 | RegisterExternalTool | S15 | S6-A01 | `tool/registry.go` |
| D4-S15-A02 | ExecuteExternalTool | S15 | S6-A02 | `tool/cli_adapter.go`, `cursor_adapter.go` |
| D4-S15-A03 | ParseStreamOutput | S15 | S6-A03 | `tool/stream_json.go` |
| D4-S16-A01 | LoadAgentConfig | S16 | config | `shared/config/multiagent.go` |

**迁 D7（Out of Scope D4）：**

| Legacy A | 目标 |
|----------|------|
| S10-A01 DelegateOrFallback | D7-S2-A04 DispatchWorker |
| S10-A02 BridgeFlowEvents | D7-S4-A04/A05 SpokeBridge |

---

## Legacy A 层（冻结追溯）— 原 v2.0.0 表

## D4-S1: Factory

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D4-S1-A01 | CreateAgent | A-BE | config, session | agent_instance | agent.created | `factory/factory.go` (AgentFactory.Create / CreateWithView) |

## D4-S2: Agent

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D4-S2-A01 | RunAgent | A-BE | ctx | agent_result | agent.{created→running→iterating→terminated} | `agent/lifecycle.go` (Impl.Run / runLoop) |
| D4-S2-A02 | ResolvePermission | A-BE | tool_name, decision | — | permission.{granted,denied} | `agent/perm_gate.go` (agentPermissionGate.Request/resolve) |
| D4-S2-A03 | WrapWorkerEngine | A-BE | inner_engine, cfg, agent_id | worker_engine | — | `agent/worker_engine.go` (NewWorkerEngine) |

## D4-S3: ForkJoin

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D4-S3-A01 | ForkAgent | A-BE | child_config | child_agent | agent.forked | `agent/forkjoin.go` (Impl.Fork) |
| D4-S3-A02 | JoinAgent | A-BE | child_agent | merged_messages | agent.joined | `agent/forkjoin.go` (Impl.Join) |
| D4-S3-A03 | CreateSessionView | A-BE | parent_session | child_view | view.forked | `sessionview/sessionview.go` (Fork) |

## D4-S4: Collaboration

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D4-S4-A01 | EnhancePrompt | A-BE | base_prompt, mode | enhanced_prompt | — | `collaboration/prompt.go` (BuildPromptForMode) |

## D4-S5: Observer

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D4-S5-A01 | BridgeAgentEvents | A-BE | agent_event | — | event.emitted | `contracts.go` (AgentObserverChain), `observer/noop.go` |

## D4-S6: AgentTool

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D4-S6-A01 | RegisterAgentTool | A-BE | tool_spec | tool_id | tool.registered | `tool/registry.go` (Registry.Register) |
| D4-S6-A02 | ExecuteAgentTool | A-BE | tool_call | tool_result | — | `tool/cli_adapter.go` (CLIAgentTool.Execute), `tool/cursor_adapter.go` (CursorAgentTool.Execute) |
| D4-S6-A03 | ParseStreamOutput | A-BE | stdout_line | stream_event | — | `tool/stream_json.go` (ParseStreamJSONLine) |

## D4-S7: Builtin

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D4-S7-A01 | RunBuiltinAgent | A-BE | deps, parent, prompt, tools | subquery_result | — | `builtin/agents.go` (RunExplore / RunPlan / RunImplement) |

## D4-S8: Observability

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D4-S8-A01 | RecordForkPolicyMetrics | A-BE | policy_label | — | counter.inc | `observability/instrument/metrics/` (IncForkSessionView / SetObservabilitySink) |

## D4-S9: SessionView

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D4-S9-A01 | ManageSessionView | A-BE | session_id, action | view_state | view.{created,merged,discarded} | `sessionview/sessionview.go` (View.SetMetadata / SetSnapshot / MergeToParent) |

## D4-S10: Delegate

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D4-S10-A01 | DelegateTask | A-BE | leader, worker_spec | delegate_result | task.{delegated,completed,failed} | `delegate/service.go` (Service.DelegateSync / DelegateAsync / DelegateOrFallback) |
| D4-S10-A02 | BridgeFlowEvents | A-BE | agent_event, engine_event | — | flow.published | `delegate/bridge.go` (FlowBridge.EmitAgentEvent / EngineEventSink) |

---

## Statistics

| Scenarios | Activities |
|-----------|------------|
| 10 | 17 |
