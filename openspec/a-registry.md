# Devrix A 层活动注册表

**Capability:** architecture-layering
**Status:** Active
**Version:** 1.0.0
**Last Updated:** 2026-06-12
**Parent:** `openspec/specs/architecture/layering.md`

---

## Overview

本文档定义 Devrix DSAFT 架构的 **A 层（活动）** 注册表。每个 Activity 代表调用方发起的具体业务动作，具有明确的输入、输出和状态变更。

A 层编号格式: `D{X}-S{X}-A{XX}`，归属于对应的 D-S 场景。

> **原则**: 保守起步，每个场景 1-3 个活动。随着项目增长可拆分，不影响已有 ID。

---

## D1: Communication Domain (通信域)

### D1-S1: Gateway

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D1-S1-A01 | ManageSession | A-BE | session_id, action | session_state | session.created / closed / expired | `communication/gateway/store.go` |
| D1-S1-A02 | RouteMessage | A-BE | message, session | routing_result, events | session.last_activity | `communication/gateway/gateway.go` |
| D1-S1-A03 | ResolvePermission | A-BE | permission_request | approved/denied | permission.resolved | `communication/gateway/permission.go` |

### D1-S2: Adapters

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D1-S2-A01 | ParseInbound | A-BE | raw_message (IM/CLI) | parsed_message | — | `communication/adapters/` |
| D1-S2-A02 | SendOutbound | A-BE | message, session | delivery_result | — | `communication/adapters/` |

### D1-S3: Commands

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D1-S3-A01 | ParseCommand | A-BE | raw_text | command, args | — | PLANNED (code in gateway) |

### D1-S4: Auth

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D1-S4-A01 | Authenticate | A-BE | credentials | auth_result | session.authenticated | PLANNED |

### D1-S5: Milestone

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D1-S5-A01 | TrackMilestone | A-BE | task_id, milestones | execution_order | milestone.{created,progress,completed,failed} | `communication/milestone/service.go` |

### D1-S6: RateLimit

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D1-S6-A01 | CheckRateLimit | A-BE | session_id, action | allowed/denied | rate.counter++ | `communication/ratelimit/limiter.go` |

### D1-S7: Metrics

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D1-S7-A01 | CollectCommMetrics | A-BE | metric_event | — | metric.recorded | `communication/metrics/` |

### D1-S8: Renderers

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D1-S8-A01 | RenderMessage | A-BE | message, format | rendered_output | — | `communication/renderers/` |

### D1-S9: EventBus

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D1-S9-A01 | PublishEvent | A-BE | *EngineEvent | — | event.queued | `communication/eventbus/bus.go` |
| D1-S9-A02 | ManageBusLifecycle | A-BE | action (drain/compact/reconnect) | bus_state | bus.{draining,compacting,reconnecting} | `communication/eventbus/` |

### D1-S10: Connection

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D1-S10-A01 | ManageConnection | A-BE | instance_id, action | connection_state | connection.{registered,unregistered} | `communication/connection/` |

### D1-S11: Core

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D1-S11-A01 | ResolveCoreConfig | A-BE | config_source | core_config | — | `communication/core/` |

### D1-S12: Instance

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D1-S12-A01 | RegisterInstance | A-BE | instance_spec | instance_id | instance.registered | `communication/instance/` |

---

## D2: Context Engine Domain (上下文引擎域)

### D2-S1: PEV (RETIRED)

> **2026-06-13**：PEV 引擎已下线。Execute / Verify / Plan 活动由 **D2-S10 QueryLoop** 承接。

| A ID | Name | Type | Status | Successor |
|------|------|------|--------|-----------|
| D2-S1-A01 | ExecutePEV | A-BE | RETIRED | D2-S10-A01 RunQueryLoop |
| D2-S1-A02 | VerifyExecution | A-BE | RETIRED | — |
| D2-S1-A03 | PlanExecution | A-BE | RETIRED | — |

### D2-S2: Compression

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D2-S2-A01 | CompressContext | A-BE | messages, budget | compressed_messages, report | context.compressed | `contextengine/compression/pipeline.go` |

### D2-S3: Memory

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D2-S3-A01 | ManageMemory | A-BE | session, messages | updated_session | memory.{appended,recalled,persisted} | `contextengine/memory/manager.go` |

### D2-S4: Token

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D2-S4-A01 | CountTokens | A-BE | text/messages | token_count, budget | — | `contextengine/token/` |

### D2-S5: Registry

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D2-S5-A01 | RegisterOperation | A-BE | tool_spec | operation_id | operation.registered | `contextengine/registry/builtin.go` |

### D2-S6: Snapshot

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D2-S6-A01 | SnapshotContext | A-BE | session | snapshot_data | snapshot.created | `contextengine/snapshot/` |

### D2-S7: Prompt

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D2-S7-A01 | ManagePromptTemplate | A-BE | template_name, params | assembled_prompt | — | `contextengine/prompt/` (PLANNED — no dedicated directory) |

### D2-S8: Sandbox

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D2-S8-A01 | IsolateTool | A-BE | tool_call, workdir | sandboxed_command | — | PLANNED (test only) |

### D2-S9: Harness

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D2-S9-A01 | BootstrapSession | A-BE | session, config | bootstrap_result | session.bootstrapped | `contextengine/harness/bootstrap.go` |
| D2-S9-A02 | AssembleSystemPrompt | A-BE | layers (base/workspace/rules/tool) | assembled_prompt | — | `contextengine/harness/system_prompt_assembler.go` |
| D2-S9-A03 | FilterToolPool | A-BE | all_tools, mode | visible_tools | — | `contextengine/harness/toolpool.go` |

### D2-S10: QueryLoop

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D2-S10-A01 | RunQueryLoop | A-BE | session, params | result (messages, usage) | loop.{iterating,completed,failed} | `contextengine/query/loop.go` |
| D2-S10-A02 | AttachUserContext | A-BE | session | enriched_session | session.user_context_set | `contextengine/usercontext/` |
| D2-S10-A03 | ExecuteBackgroundTask | A-BE | task_spec | task_id | task.{registered,running,completed} | `contextengine/query/background.go` |

### D2-S11: Queue

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D2-S11-A01 | ManageSessionQueue | A-BE | session_id, events | drained_events | queue.{enqueued,drained} | `contextengine/queue/` |

### D2-S12: Worktree

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D2-S12-A01 | EnterWorktree | A-BE | session, spec | worktree_path | worktree.{entered,exited} | `contextengine/worktree/` |

### D2-S13: Conversation

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D2-S13-A01 | ManageConversation | A-BE | session, messages | conversation_state | conversation.updated | `contextengine/conversation/` |

### D2-S14: Mock

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D2-S14-A01 | MockEngine | A-BE | mock_config | mock_engine | — | `contextengine/mock/` |

---

## D3: LLM Gateway Domain (LLM 网关域)

### D3-S1: Adapter

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D3-S1-A01 | CallModel | A-BE | llm_request | <-chan llm_chunk | — | `llmgateway/adapter/` |

### D3-S2: Gateway

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D3-S2-A01 | RouteLLMCall | A-BE | model, messages | streaming_response | — | `llmgateway/gateway/gateway.go` |

### D3-S3: Breaker

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D3-S3-A01 | ManageCircuitBreaker | A-BE | provider, call_result | circuit_state | circuit.{closed,open,half_open} | `llmgateway/breaker/circuit_breaker.go` |

### D3-S4: Retry

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D3-S4-A01 | ExecuteRetry | A-BE | attempt, error | retry_decision | — | `llmgateway/retry/` |

### D3-S5: Token

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D3-S5-A01 | CountLLMTokens | A-BE | text, encoding | token_count | — | `llmgateway/token/` |

### D3-S6: Config

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D3-S6-A01 | LoadLLMConfig | A-BE | config_file | llm_config | — | `llmgateway/config/` |

---

## D4: Multi-Agent Domain (多智能体域)

### D4-S1: Factory

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D4-S1-A01 | CreateAgent | A-BE | config, session | agent_instance | agent.created | `multiagent/factory/factory.go` |

### D4-S2: Agent

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D4-S2-A01 | RunAgent | A-BE | ctx | agent_result | agent.{created→running→iterating→terminated} | `multiagent/agent/agent.go` |
| D4-S2-A02 | ResolvePermission | A-BE | tool_name, decision | — | permission.{granted,denied} | `multiagent/agent/perm_gate.go` |

### D4-S3: ForkJoin

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D4-S3-A01 | ForkAgent | A-BE | child_config | child_agent | agent.forked | `multiagent/agent/forkjoin.go` |
| D4-S3-A02 | JoinAgents | A-BE | child_agent | merged_messages | agent.joined | `multiagent/agent/forkjoin.go` |

### D4-S4: Collaboration

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D4-S4-A01 | EnhancePrompt | A-BE | base_prompt, mode | enhanced_prompt | — | `multiagent/collaboration/prompt.go` |

### D4-S5: Observer

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D4-S5-A01 | BridgeAgentEvents | A-BE | agent_event | — | event.emitted | `multiagent/observer/` |

### D4-S6: AgentTool

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D4-S6-A01 | RegisterAgentTool | A-BE | tool_spec | tool_id | tool.registered | `multiagent/tool/` |
| D4-S6-A02 | ExecuteAgentTool | A-BE | tool_call | tool_result | — | `multiagent/tool/` |

### D4-S7: Builtin

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D4-S7-A01 | LoadBuiltinAgent | A-BE | agent_spec | agent_instance | agent.loaded | `multiagent/builtin/` |

### D4-S8: AgentObservability

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D4-S8-A01 | ObserveAgent | A-BE | agent_metric | — | metric.recorded | `multiagent/observability/` |

### D4-S9: SessionView

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D4-S9-A01 | ViewSession | A-BE | session_id | session_view | — | `multiagent/sessionview/` |

### D4-S10: Delegate

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D4-S10-A01 | DelegateTask | A-BE | leader, worker_spec | delegate_result | task.{delegated,completed,failed} | `multiagent/delegate/service.go` |
| D4-S10-A02 | TrackProgress | A-BE | task_id | progress_event | — | `multiagent/delegate/bridge.go` |

---

## D5: Observability Domain (可观测性域)

### D5-S1: Tracer

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D5-S1-A01 | CreateSpan | A-BE | span_name, parent_ctx | span, ctx | span.{started,ended} | `observability/tracer/tracer.go` |

### D5-S2: Metrics

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D5-S2-A01 | RecordMetric | A-BE | metric_name, value, labels | — | metric.recorded | `observability/metrics/` |

### D5-S3: Logger

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D5-S3-A01 | LogRecord | A-BE | level, message, fields | — | log.emitted | `observability/logger/` |

### D5-S4: Exporter

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D5-S4-A01 | ExportData | A-BE | telemetry_data | — | data.exported | `observability/exporter/` |

### D5-S5: Coverage

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D5-S5-A01 | AssessCoverage | A-BE | target_path | coverage_report | — | `observability/coverage/` |

### D5-S6: Telemetry

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D5-S6-A01 | CollectTelemetry | A-BE | telemetry_event | — | telemetry.collected | `observability/telemetry/` |

### D5-S7: Settings

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D5-S7-A01 | ManageObsSettings | A-BE | config_source | obs_config | — | `observability/settings/` |

### D5-S8: Incident

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D5-S8-A01 | DeclareIncident | A-BE | incident_spec | incident_id | incident.{declared,resolved} | `observability/incident/` |

### D5-S9: Runtime

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D5-S9-A01 | MonitorRuntime | A-BE | runtime_metric | — | metric.recorded | `observability/runtime/` |

---

## D6: Evolution Domain (演化域)

### D6-S1: Version

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D6-S1-A01 | DetectVersion | A-BE | build_info | version_report | — | PLANNED |

### D6-S2: Config

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D6-S2-A01 | HotReload | A-BE | config_watch | updated_config | config.reloaded | PLANNED |

### D6-S3: Eval

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D6-S3-A01 | RunEval | A-BE | dataset, probes | eval_report | eval.{started,completed} | `evolution/eval/engine.go` |
| D6-S3-A02 | JudgeResult | A-BE | eval_item, rubric | score | — | `evolution/eval/judge.go` |

### D6-S4: Orchestration

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D6-S4-A01 | ValidateOrchestration | A-BE | orchestration_event | validation_result | validation.{passed,failed} | `evolution/orchestration/` |

---

## ORCH: Orchestration Domain (编排域 — 跨域)

### ORCH-S1: WorkPlan

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| ORCH-S1-A01 | AggregateWorkPlan | A-BE | task_nodes, dependencies | work_plan | plan.aggregated | `orchestration/workplan/` |

### ORCH-S2: ExecutionFlowHub

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| ORCH-S2-A01 | PublishFlow | A-BE | flow_event | — | flow.event_published | `orchestration/flow/hub.go` |
| ORCH-S2-A02 | SnapshotFlow | A-BE | session_id | flow_snapshot | — | `orchestration/flow/hub.go` |

### ORCH-S3: WaveScheduler

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| ORCH-S3-A01 | ScheduleWave | A-BE | session_id, task_graph | artifact_list | wave.{started,completed,failed} | `orchestration/wave/scheduler.go` |
| ORCH-S3-A02 | ResolveContext | A-BE | task_node, session | resolved_context | — | `orchestration/wave/context.go` |
| ORCH-S3-A03 | GuardConflict | A-BE | candidate, running_tasks | allowed/blocked | — | `orchestration/wave/conflict.go` |

---

## CROSS: 跨域活动

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| CROSS-A01 | ValidateLayers | A-BE | import_graph | violation_report | — | `internal/lint/layer/` |
| CROSS-A02 | CheckContracts | A-BE | contract_catalog | check_result | — | `internal/shared/contracts/registry.go` |

---

## Statistics

| Domain | Scenarios | Activities | PLANNED |
|--------|-----------|------------|---------|
| D1 Communication | 12 | 17 | 2 |
| D2 Context Engine | 14 | 18 | 2 |
| D3 LLM Gateway | 6 | 6 | 0 |
| D4 Multi-Agent | 10 | 14 | 0 |
| D5 Observability | 9 | 9 | 0 |
| D6 Evolution | 4 | 5 | 2 |
| ORCH Orchestration | 3 | 6 | 0 |
| CROSS | — | 2 | 0 |
| **Total** | **58** | **77** | **6** |

---

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-12 | Initial A-layer registry: 77 activities across 7 domains + CROSS |
