# Devrix F 层功能点注册表

**Capability:** architecture-layering
**Status:** Active
**Version:** 1.0.0
**Last Updated:** 2026-06-12
**Parent:** `openspec/specs/architecture/layering.md`
**Depends On:** `openspec/a-registry.md`

---

## Overview

本文档定义 Devrix DSAFT 架构的 **F 层（功能点）** 注册表。每个 Function Point 代表可被 A 层活动编排的最小业务/技术逻辑单元。

F 层编号格式: `D{X}-S{X}-A{XX}-F{NN}`，归属于对应的 D-S-A 活动。

> **原则**: 保守起步，每个活动 1-5 个功能点。功能点应是可独立测试和验证的最小单元。

---

## D1: Communication Domain

### D1-S1-A01 ManageSession

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D1-S1-A01-F01 | CreateSession | F-BE | chat_id, work_dir | session | `gateway/store.go` |
| D1-S1-A01-F02 | GetSession | F-BE | session_id | session | `gateway/store.go` |
| D1-S1-A01-F03 | ExpireSession | F-BE | session_id | — | `gateway/store.go` |

### D1-S1-A02 RouteMessage

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D1-S1-A02-F01 | RouteInbound | F-BE | ctx, message | — | `gateway/gateway.go` |
| D1-S1-A02-F02 | RouteOutbound | F-BE | message | — | `gateway/gateway.go` |
| D1-S1-A02-F03 | PublishEngineEvent | F-BE | *EngineEvent | — | `gateway/gateway.go` |

### D1-S1-A03 ResolvePermission

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D1-S1-A03-F01 | RequestPermission | F-BE | session_id, tool, input, risk | request_id | `gateway/permission.go` |
| D1-S1-A03-F02 | ResolveRequest | F-BE | request_id, approved | — | `gateway/permission.go` |

### D1-S2-A01 ParseInbound

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D1-S2-A01-F01 | ParseFeishuMessage | F-BE | raw_card | message | `adapters/feishu.go` |
| D1-S2-A01-F02 | ParseCLIInput | F-BE | stdin_line | message | `adapters/cli.go` |

### D1-S2-A02 SendOutbound

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D1-S2-A02-F01 | SendFeishuReply | F-BE | message, session | — | `adapters/feishu.go` |
| D1-S2-A02-F02 | SendCLIOutput | F-BE | message | — | `adapters/cli.go` |

### D1-S5-A01 TrackMilestone

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D1-S5-A01-F01 | CreateBatch | F-BE | task_id, milestones | — | `contracts/milestone.go` |
| D1-S5-A01-F02 | GetExecutionOrder | F-BE | task_id | []*Milestone | `contracts/milestone.go` |
| D1-S5-A01-F03 | UpdateProgress | F-BE | id, progress | — | `contracts/milestone.go` |
| D1-S5-A01-F04 | CompleteMilestone | F-BE | id | — | `contracts/milestone.go` |
| D1-S5-A01-F05 | FailMilestone | F-BE | id, reason | — | `contracts/milestone.go` |

### D1-S8-A01 RenderMessage

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D1-S8-A01-F01 | RenderCLI | F-BE | message | formatted_text | `renderers/message.go` |
| D1-S8-A01-F02 | RenderCard | F-BE | milestone/progress | card_json | `renderers/components.go` |

### D1-S9-A01 PublishEvent

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D1-S9-A01-F01 | Publish | F-BE | *EngineEvent | — | `eventbus/bus.go` |
| D1-S9-A01-F02 | Drain | F-BE | threshold | drained_count | `eventbus/drain.go` |
| D1-S9-A01-F03 | Compact | F-BE | events | compacted_events | `eventbus/compact.go` |

### D1-S9-A02 ManageBusLifecycle

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D1-S9-A02-F01 | Reconnect | F-BE | — | new_bus | `eventbus/reconnect.go` |
| D1-S9-A02-F02 | Close | F-BE | — | — | `eventbus/bus.go` |

---

## D2: Context Engine Domain

### D2-S1 (RETIRED): ExecutePEV / Verify / Plan

> **2026-06-13**：PEV 功能点已移除。QueryLoop 见 D2-S10。

| F ID | Name | Status | Successor |
|------|------|--------|-----------|
| D2-S1-A01-F03 | RunPEVCycle | RETIRED | D2-S10 QueryLoop |
| D2-S1-A02-F01 | VerifyCommands | RETIRED | — |
| D2-S1-A03-F01–F03 | Plan/Milestone | RETIRED | — |

### D2-S2-A01 CompressContext

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D2-S2-A01-F01 | RunPipeline | F-BE | messages, budget | compressed, report | `compression/pipeline.go` |
| D2-S2-A01-F02 | AutoCompact | F-BE | messages, llm | compacted | `compression/autocompact.go` |
| D2-S2-A01-F03 | TruncateFallback | F-BE | messages, max_tokens | truncated | `compression/pipeline.go` |

### D2-S3-A01 ManageMemory

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D2-S3-A01-F01 | LoadOrInit | F-BE | session_id | *SessionContext | `memory/manager.go` |
| D2-S3-A01-F02 | AppendMessages | F-BE | session, messages | — | `memory/manager.go` |
| D2-S3-A01-F03 | RecallLongTerm | F-BE | session_id | []Memory | `memory/longterm.go` |

### D2-S4-A01 CountTokens

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D2-S4-A01-F01 | CountText | F-BE | text | int | `contracts/tokencounter.go` |
| D2-S4-A01-F02 | CountMessages | F-BE | []Message | int | `contracts/tokencounter.go` |
| D2-S4-A01-F03 | CountWithSystemPrompt | F-BE | prompt, messages | int | `contracts/tokencounter.go` |
| D2-S4-A01-F04 | TruncateToTokens | F-BE | text, max_tokens | string | `contracts/tokencounter.go` |
| D2-S4-A01-F05 | EncodingForModel | F-BE | model | encoding_name | `contracts/tokencounter.go` |

### D2-S5-A01 RegisterOperation

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D2-S5-A01-F01 | RegisterTool | F-BE | tool_spec | — | `registry/builtin.go` |
| D2-S5-A01-F02 | ListTools | F-BE | — | []ToolSpec | `registry/builtin.go` |

### D2-S9-A01 BootstrapSession

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D2-S9-A01-F01 | ScanWorkspace | F-BE | workdir | *WorkspaceContext | `harness/workspace.go` |
| D2-S9-A01-F02 | EvaluatePreflight | F-BE | ctx, messages, budget | *PreflightResult | `harness/preflight.go` |
| D2-S9-A01-F03 | RoutePrompt | F-BE | user_message, tools | routing_hints | `harness/router.go` |

### D2-S9-A02 AssembleSystemPrompt

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D2-S9-A02-F01 | BuildBasePrompt | F-BE | agent_config | base_prompt | `harness/system_prompt_assembler.go` |
| D2-S9-A02-F02 | MergeRulesLayers | F-BE | workspace, user, project | merged_rules | `harness/system_prompt_assembler.go` |

### D2-S9-A03 FilterToolPool

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D2-S9-A03-F01 | FilterByMode | F-BE | all_tools, mode | visible_tools | `harness/toolpool.go` |
| D2-S9-A03-F02 | FilterByConfig | F-BE | tools, deny_list | filtered_tools | `harness/toolpool.go` |

### D2-S10-A01 RunQueryLoop

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D2-S10-A01-F01 | RunLoop | F-BE | ctx, session, params | *Result | `query/loop.go` |
| D2-S10-A01-F02 | CallLLM | F-BE | request | <-chan LLMChunk | `query/loop.go` |
| D2-S10-A01-F03 | ExecuteTools | F-BE | []ToolCall | []ToolResult | `query/streaming_executor.go` |

### D2-S10-A03 ExecuteBackgroundTask

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D2-S10-A03-F01 | RegisterTask | F-BE | session_id, agent_id | task_id | `query/background.go` |
| D2-S10-A03-F02 | WaitForTask | F-BE | task_id, timeout | terminal_state | `query/background.go` |
| D2-S10-A03-F03 | CancelTask | F-BE | task_id | — | `query/background.go` |

---

## D3: LLM Gateway Domain

### D3-S1-A01 CallModel

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D3-S1-A01-F01 | StreamChat | F-BE | openai_request | <-chan chunk | `adapter/deepseek.go` / `adapter/minimax.go` |
| D3-S1-A01-F02 | ParseSSE | F-BE | raw_bytes | parsed_chunk | `adapter/sse_parser.go` |

### D3-S2-A01 RouteLLMCall

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D3-S2-A01-F01 | ResolveModel | F-BE | model_name | provider, resolved_model | `gateway/router.go` |
| D3-S2-A01-F02 | StreamWithBreaker | F-BE | ctx, request | <-chan chunk | `gateway/gateway.go` |

### D3-S3-A01 ManageCircuitBreaker

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D3-S3-A01-F01 | BeforeCall | F-BE | provider | allowed/blocked | `breaker/circuit_breaker.go` |
| D3-S3-A01-F02 | AfterCall | F-BE | provider, success | — | `breaker/circuit_breaker.go` |
| D3-S3-A01-F03 | TransitionState | F-BE | provider, state | — | `breaker/state.go` |

### D3-S5-A01 CountLLMTokens

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D3-S5-A01-F01 | EncodeText | F-BE | text, encoding | []int | `token/` |
| D3-S5-A01-F02 | DecodeTokens | F-BE | []int, encoding | string | `token/` |

---

## D4: Multi-Agent Domain

### D4-S1-A01 CreateAgent

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D4-S1-A01-F01 | NewAgent | F-BE | config, session, deps | *Impl | `factory/factory.go` |
| D4-S1-A01-F02 | CreateWithView | F-BE | config, session_view | *Impl | `factory/factory.go` |

### D4-S2-A01 RunAgent

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D4-S2-A01-F01 | ExecuteRun | F-BE | ctx | *AgentResult | `agent/agent.go` |
| D4-S2-A01-F02 | ApplyStateTransition | F-BE | from, to | — | `agent/lifecycle.go` |
| D4-S2-A01-F03 | TerminateAgent | F-BE | ctx | — | `agent/agent.go` |

### D4-S3-A01 ForkAgent

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D4-S3-A01-F01 | CreateFork | F-BE | child_config | child_agent | `agent/forkjoin.go` |
| D4-S3-A01-F02 | BuildForkedMessages | F-BE | parent_messages | child_prefix | `query/fork.go` |

### D4-S3-A02 JoinAgents

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D4-S3-A02-F01 | JoinResult | F-BE | child | merged_messages | `agent/forkjoin.go` |
| D4-S3-A02-F02 | DedupToolCalls | F-BE | messages | deduped | `agent/forkjoin.go` |

### D4-S4-A01 EnhancePrompt

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D4-S4-A01-F01 | ValidateMode | F-BE | mode | error | `collaboration/mode.go` |
| D4-S4-A01-F02 | BuildPromptForMode | F-BE | base, mode | enhanced | `collaboration/prompt.go` |

### D4-S10-A01 DelegateTask

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D4-S10-A01-F01 | DelegateSync | F-BE | leader, spec | *DelegateResult | `delegate/service.go` |
| D4-S10-A01-F02 | DelegateAsync | F-BE | leader, spec | task_id | `delegate/service.go` |

### D4-S10-A02 TrackProgress

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D4-S10-A02-F01 | EmitAgentEvent | F-BE | *AgentEvent | — | `delegate/bridge.go` |
| D4-S10-A02-F02 | PublishFlowEvent | F-BE | *FlowEvent | — | `delegate/bridge.go` |

---

## D5: Observability Domain

### D5-S1-A01 CreateSpan

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D5-S1-A01-F01 | StartSpan | F-BE | name, parent | span, ctx | `tracer/tracer.go` |
| D5-S1-A01-F02 | EndSpan | F-BE | span | — | `tracer/span.go` |
| D5-S1-A01-F03 | PropagateContext | F-BE | ctx | carrier | `tracer/propagation.go` |

### D5-S2-A01 RecordMetric

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D5-S2-A01-F01 | IncCounter | F-BE | name, labels | — | `metrics/counter.go` |
| D5-S2-A01-F02 | RecordHistogram | F-BE | name, value, labels | — | `metrics/histogram.go` |

### D5-S3-A01 LogRecord

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D5-S3-A01-F01 | LogAtLevel | F-BE | level, msg, fields | — | `logger/` |
| D5-S3-A01-F02 | ShutdownLogger | F-BE | — | — | `logger/` |

---

## D6: Evolution Domain

### D6-S3-A01 RunEval

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D6-S3-A01-F01 | LoadDataset | F-BE | path | *EvalDataset | `eval/dataset.go` |
| D6-S3-A01-F02 | RunProbe | F-BE | item, judge | *DomainScore | `eval/probe.go` |
| D6-S3-A01-F03 | AggregateReport | F-BE | []DomainScore | *EvalReport | `eval/engine.go` |
| D6-S3-A01-F04 | CheckDeltaGate | F-BE | delta | GateResult | `eval/delta.go` |

### D6-S3-A02 JudgeResult

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D6-S3-A02-F01 | ScoreWithRubric | F-BE | item, rubric | score | `eval/judge.go` |
| D6-S3-A02-F02 | ResolveDispute | F-BE | dispute | resolved_score | `eval/judge.go` |

---

## ORCH: Orchestration Domain

### ORCH-S2-A01 PublishFlow

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| ORCH-S2-A01-F01 | Publish | F-BE | ctx, FlowEvent | — | `contracts/execution_flow.go` |
| ORCH-S2-A01-F02 | DispatchToIM | F-BE | FlowEvent | — | `orchestration/flow/hub.go` |

### ORCH-S3-A01 ScheduleWave

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| ORCH-S3-A01-F01 | StartWave | F-BE | session_id, task_graph | — | `wave/scheduler.go` |
| ORCH-S3-A01-F02 | DispatchWorker | F-BE | task_node, slot | — | `wave/scheduler.go` |
| ORCH-S3-A01-F03 | WaitForCompletion | F-BE | session_id | []Artifact | `wave/scheduler.go` |

### ORCH-S3-A02 ResolveContext

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| ORCH-S3-A02-F01 | ResolveFreshContext | F-BE | task_node, session | *ResolvedContext | `wave/context.go` |
| ORCH-S3-A02-F02 | ResolveUpstreamContext | F-BE | task_node, artifacts | *ResolvedContext | `wave/context.go` |

### ORCH-S3-A03 GuardConflict

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| ORCH-S3-A03-F01 | CheckConflict | F-BE | candidate, running | allowed/blocked | `wave/conflict.go` |
| ORCH-S3-A03-F02 | RegisterTask | F-BE | task_node | slot_id | `wave/conflict.go` |

---

## CROSS: 跨域功能点

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| CROSS-A01-F01 | ScanImports | F-BE | packages | import_graph | `lint/layer/scanner.go` |
| CROSS-A01-F02 | CheckViolation | F-BE | import_graph, rules | []Violation | `lint/layer/scanner.go` |
| CROSS-A02-F01 | ComputeCtxPct | F-BE | prompt_tokens, max_tokens | pct (0-100) | `contracts/ctxutil.go` |
| CROSS-A02-F02 | SelfCheck | F-BE | — | []ContractStatus | `contracts/registry.go` |

---

## Bridges (桥接功能点)

桥接层功能点属于其桥接的目标域活动。

### LLM Bridge (D3 → D2)

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D3-S1-A01-F03 | AdaptToContextEngine | F-BE | llmgateway_request | contextengine_response | `bridges/llm/bridge.go` |

### Milestone Bridge (D1 → D2)

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D1-S5-A01-F06 | AdaptToPlanner | F-BE | milestone_service | planner_interface | `bridges/milestone/wire.go` |

---

## Statistics

| Domain | Activities with F | Total F Points |
|--------|-------------------|----------------|
| D1 Communication | 8 | 20 |
| D2 Context Engine | 10 | 27 |
| D3 LLM Gateway | 4 | 9 |
| D4 Multi-Agent | 6 | 14 |
| D5 Observability | 3 | 7 |
| D6 Evolution | 2 | 6 |
| ORCH Orchestration | 3 | 9 |
| CROSS + Bridges | 2 + 2 | 6 |
| **Total** | **38** | **98** |

---

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-12 | Initial F-layer registry: 98 function points across 7 domains + CROSS + Bridges |
