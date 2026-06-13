# D1 Communication Domain — A 层活动注册表

**Capability:** architecture-layering
**Status:** Active
**Version:** 2.0.0
**Last Updated:** 2026-06-13
**Parent:** `openspec/specs/architecture/layering.md`

---

## Overview

D1 通信域 A 层活动注册表。每个 Activity 代表调用方发起的具体业务动作。

---

## D1-S1: Gateway

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D1-S1-A01 | ManageSession | A-BE | session_id, action | session_state | session.created / closed / expired | `gateway/store.go` |
| D1-S1-A02 | RouteMessage | A-BE | message, session | routing_result, events | session.last_activity | `gateway/gateway.go` |
| D1-S1-A03 | ResolvePermission | A-BE | permission_request | approved/denied | permission.resolved | `gateway/permission.go` |
| D1-S1-A04 | RouteAgent | A-BE | agent_factory, session | agent_registered | session.agent_bound | `gateway/agent_route.go` |

## D1-S2: Adapters

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D1-S2-A01 | ParseInbound | A-BE | raw_message (IM/CLI) | parsed_message | — | `adapters/feishu.go`, `adapters/dingtalk.go`, `adapters/cli.go` |
| D1-S2-A02 | SendOutbound | A-BE | message, session | delivery_result | — | `adapters/feishu.go`, `adapters/dingtalk_outbound.go`, `adapters/cli.go` |
| D1-S2-A03 | RenderWorkerCard | A-BE | worker_event, session | card_rendered | card.created / updated | `adapters/feishu_worker_card.go` |
| D1-S2-A04 | StreamCardContent | A-BE | card_id, content, sequence | stream_result | card.streaming | `adapters/feishu_cardkit.go` |
| D1-S2-A05 | ResolveOrCreateSession | A-BE | session_key | session | session.cached | `adapters/session_resolve.go` |

## D1-S3: Commands

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D1-S3-A01 | ParseCommand | A-BE | raw_text | command, args | — | `adapters/cli.go` (handleCommand), `adapters/feishu.go` (handleFeishuCommand) |

## D1-S4: Auth

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D1-S4-A01 | Authenticate | A-BE | credentials | auth_result | session.authenticated | PLANNED |

## D1-S5: Milestone

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D1-S5-A01 | TrackMilestone | A-BE | task_id, milestones | execution_order | milestone.{created,progress,completed,failed} | `milestone/service.go` |
| D1-S5-A02 | OrchestrateTaskFlow | A-BE | taskflow_spec | taskflow_state | taskflow.{started,progress,completed,failed} | `milestone/taskflow.go` |

## D1-S6: RateLimit

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D1-S6-A01 | CheckRateLimit | A-BE | adapter_id, action | allowed/denied | rate.counter++ | `ratelimit/limiter.go` |

## D1-S7: Metrics

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D1-S7-A01 | CollectCommMetrics | A-BE | metric_event | — | metric.recorded | [DEPRECATED] `metrics/collector.go` — 已迁移至 D5 `observability.Bridge` (DM-20260607-007) |

## D1-S8: Renderers

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D1-S8-A01 | RenderMessage | A-BE | message, format | rendered_output | — | `renderers/message.go`, `renderers/dingtalk_card.go`, `renderers/components.go` |

## D1-S9: EventBus

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D1-S9-A01 | PublishEvent | A-BE | *Event | — | event.queued | `eventbus/bus.go` |
| D1-S9-A02 | ManageBusLifecycle | A-BE | action (drain/compact/reconnect) | bus_state | bus.{draining,compacting,reconnecting} | `eventbus/drain.go`, `eventbus/compact.go`, `eventbus/reconnect.go` |

## D1-S10: Connection

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D1-S10-A01 | ManageConnection | A-BE | instance_id, action | connection_state | connection.{registered,unregistered,restored,lost} | `connection/manager.go` |

## D1-S11: Core

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D1-S11-A01 | ResolveCoreConfig | A-BE | config_source | core_config | — | `core/card.go` (Card/CardBuilder 模型) |

## D1-S12: Instance

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D1-S12-A01 | RegisterInstance | A-BE | instance_spec | instance_id | instance.registered | `instance/registry.go` |

---

## Statistics

| Scenarios | Activities | IMPLEMENTED | PLANNED | DEPRECATED |
|-----------|------------|-------------|---------|------------|
| 12 | 21 | 19 | 1 | 1 |
