# D4 Legacy Scenarios — D4-S1 ~ D4-S10（冻结追溯）

**Demand ID:** DM-20260629-004（PR-4 #2 registry-sync）
**Original Change ID:** devrix-d4-sa-refine (DM-20260614-018)
**Archived:** 2026-06-30
**Status:** Frozen (read-only)
**Reason:** D4 域 v2.0 物理路径迁移（factory/agent/tool/sessionview/delegate/observer/ → provision/run/isolate/execute/external/kernel/orchtypes/）完成后，D4-S1~S10 详细 A/F 表已沉到本 archive，仅保留追溯。

---

## §1 冻结索引

| Legacy S | 主题 | Canonical 归属 | 迁移路径 |
|----------|------|---------------|----------|
| D4-S1 | Factory | D4-S11 ProvisionAgent | `factory/factory.go` → `provision/factory.go` |
| D4-S2 | Agent | D4-S12 RunAgentLoop + D4-S13-A03 WrapWorkerEngine | `agent/lifecycle.go` `agent/perm_gate.go` → `run/lifecycle.go` `run/perm_gate.go`；`agent/worker_engine.go` → `provision/factory.go`（PR-1 #0 inline） |
| D4-S3 | ForkJoin | D4-S13 IsolateAndMerge | `agent/forkjoin.go` → `run/forkjoin.go`（NOTE: live in `run/`，非 `isolate/`） |
| D4-S4 | Collaboration | D4-S11-A02 EnhancePrompt | `collaboration/prompt.go` (no rename) |
| D4-S5 | Observer | D4 kernel 横切 | `contracts.go` `observer/noop.go` → `kernel/contracts.go` `kernel/noop.go` |
| D4-S6 | AgentTool | D4-S15 InvokeExternalAgent | `tool/registry.go` → `external/registry.go`；`tool/cli_adapter.go` → `external/cli_session.go` + `cli_execute.go`（PR-2 #1 god-fn pt1）；`tool/cursor_adapter.go` → `external/cursor_session.go` + `cursor_execute.go`（PR-3 #1 god-fn pt2）；`tool/stream_json.go` → `external/stream_json.go` |
| D4-S7 | Builtin | D7-S2 fallback 路由 | `builtin/agents.go` → `orchestration/delegatetools/builtin_agents.go`（D7 owns） |
| D4-S8 | Observability | D5 metric | `observability/instrument/metrics/` 迁 D5 |
| D4-S9 | SessionView | D4-S13-A02 ManageSessionView | `sessionview/sessionview.go` → `isolate/sessionview.go` |
| D4-S10 | Delegate | D4-S14 ExecuteWorker（执行面） + D7-S2/S4（编排面） | `delegate/service.go` → `execute/worker.go`；`delegate/bridge.go` 删（迁 D7） |

---

## §2 Legacy A 层（冻结追溯）— 原 v2.0.0 表

> 来源：`openspec/specs/d4-multi-agent/a-registry.md` v3.0.0/v3.1.0 §Legacy A 层。原表注：迁移 v2.0 时 v1.0 行号保留。

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

## §3 Legacy F 层（冻结追溯）— 原 v2.0.0 表

> 来源：`openspec/specs/d4-multi-agent/f-registry.md` v3.0.0 §Legacy F 层。原表注：保留原始 F-ID 编号供 `// T:` 注释追溯。

### D4-S1-A01 CreateAgent

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D4-S1-A01-F01 | NewAgent | F-BE | config, session, deps | *Impl | `agent/agent.go` (New) |
| D4-S1-A01-F02 | CreateWithView | F-BE | config, session_view | *Impl | `factory/factory.go` (CreateWithView) |

### D4-S2-A01 RunAgent

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D4-S2-A01-F01 | ExecuteRun | F-BE | ctx | *AgentResult | `agent/lifecycle.go` (Run / runLoop) |
| D4-S2-A01-F02 | ApplyStateTransition | F-BE | from, to | — | `agent/agent.go` (setState) |
| D4-S2-A01-F03 | TerminateAgent | F-BE | ctx | — | `agent/lifecycle.go` (Terminate) |
| D4-S2-A01-F04 | WaitAgent | F-BE | ctx | *AgentResult | `agent/lifecycle.go` (Wait) |

### D4-S2-A02 ResolvePermission

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D4-S2-A02-F01 | RequestPermission | F-BE | tool_name, risk | decision_ch | `agent/perm_gate.go` (Request) |
| D4-S2-A02-F02 | ResolveDecision | F-BE | tool_name, granted | — | `agent/perm_gate.go` (resolve) |

### D4-S2-A03 WrapWorkerEngine

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D4-S2-A03-F01 | NewWorkerEngine | F-BE | inner_engine, cfg, agent_id | *WorkerEngine | `agent/worker_engine.go` (NewWorkerEngine) |
| D4-S2-A03-F02 | ProcessOverlay | F-BE | session, message | event_ch | `agent/worker_engine.go` (Process) |

### D4-S3-A01 ForkAgent

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D4-S3-A01-F01 | CreateFork | F-BE | child_config | child_agent | `agent/forkjoin.go` (Fork) |
| D4-S3-A01-F02 | ForkSessionView | F-BE | parent_session | child_view | `sessionview/sessionview.go` (Fork) |

### D4-S3-A02 JoinAgent

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D4-S3-A02-F01 | JoinResult | F-BE | child | merged_messages | `agent/forkjoin.go` (Join) |
| D4-S3-A02-F02 | DedupToolCalls | F-BE | messages | deduped | `agent/forkjoin.go` (dedupToolCallMessages) |

### D4-S4-A01 EnhancePrompt

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D4-S4-A01-F01 | ValidateMode | F-BE | mode | error | `collaboration/mode.go` (ValidateMode) |
| D4-S4-A01-F02 | BuildPromptForMode | F-BE | base, mode | enhanced | `collaboration/prompt.go` (BuildPromptForMode) |

### D4-S5-A01 BridgeAgentEvents

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D4-S5-A01-F01 | EmitAgentEvent | F-BE | *AgentEvent | — | `contracts.go` (AgentObserverChain) |
| D4-S5-A01-F02 | NoOpObserver | F-BE | *AgentEvent | — | `observer/noop.go` (NoOpAgentObserver) |

### D4-S6-A01 RegisterAgentTool

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D4-S6-A01-F01 | RegisterTool | F-BE | tool_spec | — | `tool/registry.go` (Register) |
| D4-S6-A01-F02 | LookupTool | F-BE | name | Info | `tool/registry.go` (Get / List) |

### D4-S6-A02 ExecuteAgentTool

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D4-S6-A02-F01 | ExecuteCLI | F-BE | ctx, req | event_ch | `tool/cli_adapter.go` (Execute) |
| D4-S6-A02-F02 | ExecuteCursor | F-BE | ctx, req | event_ch | `tool/cursor_adapter.go` (Execute) |
| D4-S6-A02-F03 | ManageSession | F-BE | session_id | — | `tool/cli_adapter.go` (ensureSession / CloseSession) |

### D4-S6-A03 ParseStreamOutput

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D4-S6-A03-F01 | ParseStreamJSONLine | F-BE | stdout_line | StreamParseResult | `tool/stream_json.go` (ParseStreamJSONLine) |

### D4-S7-A01 RunBuiltinAgent

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D4-S7-A01-F01 | RunExplore | F-BE | deps, prompt, tools | *SubQueryResult | `builtin/agents.go` (RunExplore) |
| D4-S7-A01-F02 | RunPlan | F-BE | deps, prompt, tools | *SubQueryResult | `builtin/agents.go` (RunPlan) |
| D4-S7-A01-F03 | RunImplement | F-BE | deps, prompt, tools | *SubQueryResult | `builtin/agents.go` (RunImplement) |

### D4-S8-A01 RecordForkPolicyMetrics

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D4-S8-A01-F01 | IncForkSessionView | F-BE | policy_label | — | `observability/instrument/metrics/` (IncForkSessionView) |
| D4-S8-A01-F02 | SetObservabilitySink | F-BE | sink | — | `observability/instrument/metrics/` (SetObservabilitySink) |

### D4-S9-A01 ManageSessionView

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D4-S9-A01-F01 | SetMetadata | F-BE | key, value | — | `sessionview/sessionview.go` (SetMetadata) |
| D4-S9-A01-F02 | SetSnapshot | F-BE | snap_bytes | — | `sessionview/sessionview.go` (SetSnapshot) |
| D4-S9-A01-F03 | MergeToParent | F-BE | parent_session | — | `sessionview/sessionview.go` (MergeToParent) |

### D4-S10-A01 DelegateTask

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D4-S10-A01-F01 | DelegateSync | F-BE | leader, spec | *DelegateResult | `delegate/service.go` (DelegateSync) |
| D4-S10-A01-F02 | DelegateAsync | F-BE | leader, spec | task_id | `delegate/service.go` (DelegateAsync) |
| D4-S10-A01-F03 | DelegateOrFallback | F-BE | leader, spec | DelegateResult | `delegate/service.go` (DelegateOrFallback) |

### D4-S10-A02 BridgeFlowEvents

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D4-S10-A02-F01 | EmitAgentEvent | F-BE | *AgentEvent | — | `delegate/bridge.go` (EmitAgentEvent) |
| D4-S10-A02-F02 | EngineEventSink | F-BE | *EngineEvent | — | `delegate/bridge.go` (EngineEventSink) |

---

## §4 备注

- 本文件**只读**，不接受 PR 修改
- 新需求请走 D4 Canonical S11-S16 (见 `a-registry.md` / `f-registry.md`)
- 历史追溯定位：搜 `D4-S1~S10` 关键字 → 本 archive
