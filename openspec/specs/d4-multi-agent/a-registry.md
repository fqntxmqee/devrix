# D4 Multi-Agent Domain — A 层活动注册表

**Capability:** architecture-layering
**Status:** Active
**Version:** 1.0.0
**Last Updated:** 2026-06-13
**Parent:** `openspec/specs/architecture/layering.md`

---

## Overview

D4 多智能体域 A 层活动注册表。

---

## D4-S1: Factory

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D4-S1-A01 | CreateAgent | A-BE | config, session | agent_instance | agent.created | `multiagent/factory/factory.go` |

## D4-S2: Agent

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D4-S2-A01 | RunAgent | A-BE | ctx | agent_result | agent.{created→running→iterating→terminated} | `multiagent/agent/agent.go` |
| D4-S2-A02 | ResolvePermission | A-BE | tool_name, decision | — | permission.{granted,denied} | `multiagent/agent/perm_gate.go` |

## D4-S3: ForkJoin

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D4-S3-A01 | ForkAgent | A-BE | child_config | child_agent | agent.forked | `multiagent/agent/forkjoin.go` |
| D4-S3-A02 | JoinAgents | A-BE | child_agent | merged_messages | agent.joined | `multiagent/agent/forkjoin.go` |

## D4-S4: Collaboration

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D4-S4-A01 | EnhancePrompt | A-BE | base_prompt, mode | enhanced_prompt | — | `multiagent/collaboration/prompt.go` |

## D4-S5: Observer

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D4-S5-A01 | BridgeAgentEvents | A-BE | agent_event | — | event.emitted | `multiagent/observer/` |

## D4-S6: AgentTool

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D4-S6-A01 | RegisterAgentTool | A-BE | tool_spec | tool_id | tool.registered | `multiagent/tool/` |
| D4-S6-A02 | ExecuteAgentTool | A-BE | tool_call | tool_result | — | `multiagent/tool/` |

## D4-S7: Builtin

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D4-S7-A01 | LoadBuiltinAgent | A-BE | agent_spec | agent_instance | agent.loaded | `multiagent/builtin/` |

## D4-S8: AgentObservability

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D4-S8-A01 | ObserveAgent | A-BE | agent_metric | — | metric.recorded | `multiagent/observability/` |

## D4-S9: SessionView

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D4-S9-A01 | ViewSession | A-BE | session_id | session_view | — | `multiagent/sessionview/` |

## D4-S10: Delegate

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D4-S10-A01 | DelegateTask | A-BE | leader, worker_spec | delegate_result | task.{delegated,completed,failed} | `multiagent/delegate/service.go` |
| D4-S10-A02 | TrackProgress | A-BE | task_id | progress_event | — | `multiagent/delegate/bridge.go` |

---

## Statistics

| Scenarios | Activities |
|-----------|------------|
| 10 | 14 |
