# D1 Communication Domain — F 层功能点注册表

**Capability:** architecture-layering
**Status:** Active
**Version:** 2.0.0
**Last Updated:** 2026-06-13
**Parent:** `openspec/specs/architecture/layering.md`
**Depends On:** `openspec/specs/d1-communication/a-registry.md`

---

## Overview

D1 通信域 F 层功能点注册表。

---

## D1-S1-A01 ManageSession

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D1-S1-A01-F01 | CreateSession | F-BE | chat_id, work_dir | session | `gateway/store.go` (FileSessionStore.Create) |
| D1-S1-A01-F02 | GetSession | F-BE | session_id | session | `gateway/store.go` (FileSessionStore.Get) |
| D1-S1-A01-F03 | ExpireSession | F-BE | session_id | — | `gateway/store.go` (FileSessionStore.Delete) |

## D1-S1-A02 RouteMessage

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D1-S1-A02-F01 | RouteInbound | F-BE | ctx, message | — | `gateway/gateway.go` (CommunicationGateway.RouteInbound) |
| D1-S1-A02-F02 | RouteOutbound | F-BE | message | — | `gateway/gateway.go` (CommunicationGateway.RouteOutbound) |
| D1-S1-A02-F03 | PublishEngineEvent | F-BE | *EngineEvent | — | `gateway/gateway.go` (CommunicationGateway.PublishEngineEvent) |

## D1-S1-A03 ResolvePermission

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D1-S1-A03-F01 | RequestPermission | F-BE | session_id, tool, input, risk | request_id | `gateway/permission.go` (PermissionManager.Request) |
| D1-S1-A03-F02 | ResolveRequest | F-BE | request_id, approved | — | `gateway/permission.go` (PermissionManager.Resolve) |

## D1-S1-A04 RouteAgent

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D1-S1-A04-F01 | SetAgentFactory | F-BE | factory, observer_factory | — | `gateway/agent_route.go` (CommunicationGateway.SetAgentFactory) |
| D1-S1-A04-F02 | RegisterSessionAgent | F-BE | session_id, agent | — | `gateway/agent_route.go` (CommunicationGateway.RegisterSessionAgent) |
| D1-S1-A04-F03 | ResolveAgentPermission | F-BE | session_id, tool_name, granted | — | `gateway/agent_route.go` (CommunicationGateway.ResolveAgentPermission) |

## D1-S2-A01 ParseInbound

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D1-S2-A01-F01 | ParseFeishuMessage | F-BE | raw_card | message | `adapters/feishu.go` (onMessage, webhookHandler) |
| D1-S2-A01-F02 | ParseCLIInput | F-BE | stdin_line | message | `adapters/cli.go` (readLine, handleCommand) |
| D1-S2-A01-F03 | ParseDingTalkMessage | F-BE | webhook_body | message | `adapters/dingtalk.go` (webhookHandler) |

## D1-S2-A02 SendOutbound

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D1-S2-A02-F01 | SendFeishuReply | F-BE | message, session | — | `adapters/feishu.go` (OnMessage handler) |
| D1-S2-A02-F02 | SendCLIOutput | F-BE | message | — | `adapters/cli.go` (OnMessage handler) |
| D1-S2-A02-F03 | SendDingTalkReply | F-BE | message, session | — | `adapters/dingtalk.go` (OnMessage handler), `adapters/dingtalk_outbound.go` |

## D1-S2-A03 RenderWorkerCard

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D1-S2-A03-F01 | EmitWorkerEvent | F-BE | opts, worker_event | — | `adapters/feishu_worker_card.go` (WorkerCardRenderer.EmitWorkerEvent) |
| D1-S2-A03-F02 | SnapshotWorkerCard | F-BE | session_id, task_id | thinking, output, status | `adapters/feishu_worker_card.go` (WorkerCardRenderer.Snapshot) |

## D1-S2-A04 StreamCardContent

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D1-S2-A04-F01 | CreateStreamCard | F-BE | card_json | card_id | `adapters/feishu_cardkit.go` (CardkitClient.CreateCard) |
| D1-S2-A04-F02 | StreamElement | F-BE | card_id, element_id, content, seq | — | `adapters/feishu_cardkit.go` (CardkitClient.StreamElementContent) |
| D1-S2-A04-F03 | UpdateStreamCard | F-BE | card_id, card_json, seq | — | `adapters/feishu_cardkit.go` (CardkitClient.UpdateCard) |

## D1-S2-A05 ResolveOrCreateSession

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D1-S2-A05-F01 | ResolveSession | F-BE | session_key, session_map, gateway | session | `adapters/session_resolve.go` (resolveOrCreateSession) |

## D1-S5-A01 TrackMilestone

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D1-S5-A01-F01 | CreateMilestone | F-BE | milestone | — | `milestone/service.go` (MilestoneService.Create) |
| D1-S5-A01-F02 | GetExecutionOrder | F-BE | — | []*Milestone | `milestone/service.go` (MilestoneService.GetExecutionOrder) |
| D1-S5-A01-F03 | UpdateProgress | F-BE | id, progress | — | `milestone/service.go` (MilestoneService.UpdateProgress) |
| D1-S5-A01-F04 | CompleteMilestone | F-BE | id | — | `milestone/service.go` (MilestoneService.Complete) |
| D1-S5-A01-F05 | FailMilestone | F-BE | id, reason | — | `milestone/service.go` (MilestoneService.Fail) |

## D1-S5-A02 OrchestrateTaskFlow

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D1-S5-A02-F01 | CreateTaskFlow | F-BE | name, dag | taskflow | `milestone/taskflow.go` (TaskFlowService.Create) |
| D1-S5-A02-F02 | StartTaskFlow | F-BE | id | — | `milestone/taskflow.go` (TaskFlowService.Start) |
| D1-S5-A02-F03 | AdvanceMilestone | F-BE | id, milestone_id | — | `milestone/taskflow.go` (TaskFlowService.CompleteMilestone) |
| D1-S5-A02-F04 | FailTaskFlowMilestone | F-BE | id, milestone_id, reason | — | `milestone/taskflow.go` (TaskFlowService.FailMilestone) |
| D1-S5-A02-F05 | GetTaskFlowProgress | F-BE | id | progress | `milestone/taskflow.go` (TaskFlowService.GetProgress) |

## D1-S8-A01 RenderMessage

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D1-S8-A01-F01 | RenderCLI | F-BE | message | formatted_text | `renderers/message.go` (CLIRenderer.RenderMessage) |
| D1-S8-A01-F02 | RenderCard | F-BE | milestone/progress | card_json/markdown | `renderers/components.go`, `renderers/dingtalk_card.go` |

## D1-S9-A01 PublishEvent

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D1-S9-A01-F01 | Publish | F-BE | *Event | — | `eventbus/bus.go` (Bus.Publish) |
| D1-S9-A01-F02 | PublishCritical | F-BE | *Event | — | `eventbus/bus.go` (Bus.PublishCritical) |
| D1-S9-A01-F03 | Drain | F-BE | threshold | drained_count | `eventbus/drain.go` (Bus.Drain) |
| D1-S9-A01-F04 | Compact | F-BE | events | compacted_events | `eventbus/compact.go` (Bus.Compact) |

## D1-S9-A02 ManageBusLifecycle

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D1-S9-A02-F01 | Reconnect | F-BE | — | new_bus | `eventbus/reconnect.go` (Bus.Reconnect) |
| D1-S9-A02-F02 | Close | F-BE | — | — | `eventbus/bus.go` (Bus.Close) |

## Cross-Domain Bridges

### Milestone Bridge (D1 → D2)

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D1-S5-A01-F06 | AdaptToPlanner | F-BE | milestone_service | planner_interface | `bridges/milestone/wire.go` |

### Permission Bridge (D1 → D4)

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D1-S1-A03-F03 | AdaptToPermissionGate | F-BE | permission_mgr | ipg_interface | `gateway/permission_adapter.go` (PermissionGateAdapter) |

---

## Statistics

| Activities with F | Total F Points | Cross-Domain Bridges |
|-------------------|----------------|---------------------|
| 12 | 43 | 2 |
