# D1 Communication Domain — F 层功能点注册表

**Capability:** architecture-layering
**Status:** Active
**Version:** 1.0.0
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
| D1-S1-A01-F01 | CreateSession | F-BE | chat_id, work_dir | session | `gateway/store.go` |
| D1-S1-A01-F02 | GetSession | F-BE | session_id | session | `gateway/store.go` |
| D1-S1-A01-F03 | ExpireSession | F-BE | session_id | — | `gateway/store.go` |

## D1-S1-A02 RouteMessage

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D1-S1-A02-F01 | RouteInbound | F-BE | ctx, message | — | `gateway/gateway.go` |
| D1-S1-A02-F02 | RouteOutbound | F-BE | message | — | `gateway/gateway.go` |
| D1-S1-A02-F03 | PublishEngineEvent | F-BE | *EngineEvent | — | `gateway/gateway.go` |

## D1-S1-A03 ResolvePermission

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D1-S1-A03-F01 | RequestPermission | F-BE | session_id, tool, input, risk | request_id | `gateway/permission.go` |
| D1-S1-A03-F02 | ResolveRequest | F-BE | request_id, approved | — | `gateway/permission.go` |

## D1-S2-A01 ParseInbound

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D1-S2-A01-F01 | ParseFeishuMessage | F-BE | raw_card | message | `adapters/feishu.go` |
| D1-S2-A01-F02 | ParseCLIInput | F-BE | stdin_line | message | `adapters/cli.go` |

## D1-S2-A02 SendOutbound

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D1-S2-A02-F01 | SendFeishuReply | F-BE | message, session | — | `adapters/feishu.go` |
| D1-S2-A02-F02 | SendCLIOutput | F-BE | message | — | `adapters/cli.go` |

## D1-S5-A01 TrackMilestone

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D1-S5-A01-F01 | CreateBatch | F-BE | task_id, milestones | — | `contracts/milestone.go` |
| D1-S5-A01-F02 | GetExecutionOrder | F-BE | task_id | []*Milestone | `contracts/milestone.go` |
| D1-S5-A01-F03 | UpdateProgress | F-BE | id, progress | — | `contracts/milestone.go` |
| D1-S5-A01-F04 | CompleteMilestone | F-BE | id | — | `contracts/milestone.go` |
| D1-S5-A01-F05 | FailMilestone | F-BE | id, reason | — | `contracts/milestone.go` |

## D1-S8-A01 RenderMessage

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D1-S8-A01-F01 | RenderCLI | F-BE | message | formatted_text | `renderers/message.go` |
| D1-S8-A01-F02 | RenderCard | F-BE | milestone/progress | card_json | `renderers/components.go` |

## D1-S9-A01 PublishEvent

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D1-S9-A01-F01 | Publish | F-BE | *EngineEvent | — | `eventbus/bus.go` |
| D1-S9-A01-F02 | Drain | F-BE | threshold | drained_count | `eventbus/drain.go` |
| D1-S9-A01-F03 | Compact | F-BE | events | compacted_events | `eventbus/compact.go` |

## D1-S9-A02 ManageBusLifecycle

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D1-S9-A02-F01 | Reconnect | F-BE | — | new_bus | `eventbus/reconnect.go` |
| D1-S9-A02-F02 | Close | F-BE | — | — | `eventbus/bus.go` |

## Milestone Bridge (D1 → D2)

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D1-S5-A01-F06 | AdaptToPlanner | F-BE | milestone_service | planner_interface | `bridges/milestone/wire.go` |

---

## Statistics

| Activities with F | Total F Points |
|-------------------|----------------|
| 8 | 21 |
