# D4 Multi-Agent Domain — F 层功能点注册表

**Capability:** architecture-layering
**Status:** Active
**Version:** 1.0.0
**Last Updated:** 2026-06-13
**Parent:** `openspec/specs/architecture/layering.md`
**Depends On:** `openspec/specs/d4-multi-agent/a-registry.md`

---

## Overview

D4 多智能体域 F 层功能点注册表。

---

## D4-S1-A01 CreateAgent

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D4-S1-A01-F01 | NewAgent | F-BE | config, session, deps | *Impl | `factory/factory.go` |
| D4-S1-A01-F02 | CreateWithView | F-BE | config, session_view | *Impl | `factory/factory.go` |

## D4-S2-A01 RunAgent

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D4-S2-A01-F01 | ExecuteRun | F-BE | ctx | *AgentResult | `agent/agent.go` |
| D4-S2-A01-F02 | ApplyStateTransition | F-BE | from, to | — | `agent/lifecycle.go` |
| D4-S2-A01-F03 | TerminateAgent | F-BE | ctx | — | `agent/agent.go` |

## D4-S3-A01 ForkAgent

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D4-S3-A01-F01 | CreateFork | F-BE | child_config | child_agent | `agent/forkjoin.go` |
| D4-S3-A01-F02 | BuildForkedMessages | F-BE | parent_messages | child_prefix | `query/fork.go` |

## D4-S3-A02 JoinAgents

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D4-S3-A02-F01 | JoinResult | F-BE | child | merged_messages | `agent/forkjoin.go` |
| D4-S3-A02-F02 | DedupToolCalls | F-BE | messages | deduped | `agent/forkjoin.go` |

## D4-S4-A01 EnhancePrompt

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D4-S4-A01-F01 | ValidateMode | F-BE | mode | error | `collaboration/mode.go` |
| D4-S4-A01-F02 | BuildPromptForMode | F-BE | base, mode | enhanced | `collaboration/prompt.go` |

## D4-S10-A01 DelegateTask

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D4-S10-A01-F01 | DelegateSync | F-BE | leader, spec | *DelegateResult | `delegate/service.go` |
| D4-S10-A01-F02 | DelegateAsync | F-BE | leader, spec | task_id | `delegate/service.go` |

## D4-S10-A02 TrackProgress

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D4-S10-A02-F01 | EmitAgentEvent | F-BE | *AgentEvent | — | `delegate/bridge.go` |
| D4-S10-A02-F02 | PublishFlowEvent | F-BE | *FlowEvent | — | `delegate/bridge.go` |

---

## Statistics

| Activities with F | Total F Points |
|-------------------|----------------|
| 6 | 14 |
