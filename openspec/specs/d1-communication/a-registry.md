# D1 Communication Domain — A 层活动注册表

**Capability:** architecture-layering
**Status:** Active
**Version:** 3.2.0  
**Last Updated:** 2026-06-30 (DM-20260629-005 PR-5 value-flow-rename)
**Parent:** `openspec/specs/architecture/layering.md`
**Domain SoT:** `openspec/specs/d1-communication/d1-domain.md`
**Change:** DM-20260614-006 — 切法 A 双轨 / DM-20260628-003 (devrix-d1-dsaft-refactor) — DSAFT 边界 + Gateway 拆分 + contracts DTO + lint-d1-imports CI / **devrix-d1-ac-restructuring (DM-20260629-005) PR-5 #3 value-flow-rename — 6 D1_* ValueFlow Alias 加入 a-registry (v3.2.0)**

---

## Overview

D1 通信域 A 层活动注册表。**Canonical SoT：D1-S13–S18**（DM-20260614-006）。  
Legacy D1-S1–S12 活动表保留于下文，供代码位置追溯；新能力只登记 canonical A。

> **流程与时序**（A→F 编排树、IntentKind、跨域契约）见 `terminal-state-guide.md`；本文不重复流程描述。

---

## Canonical — D1-S13–S18（价值流）

### ValueFlow Alias (DM-20260629-005 PR-5 #3 value-flow-rename)

> 价值流别名与跨域 span / signal contract 对齐，避免与 D7 alias 命名冲突。  
> 使用约定：`D1_<Verb>_<Object>`（snake_case 大写），与代码 const / span op / observability tag 同形。

| Canonical S | Name | ValueFlow Alias | 跨域对接 |
|-------------|------|-----------------|----------|
| D1-S13 | CaptureUserIntent | `D1_Capture_User_Intent` | 入站 → D7 `ProcessMessage` (DM-20260629-005 boundary decision 1) |
| D1-S14 | PresentThinking | `D1_Present_Thinking` | 出站 → IM adapter thinking 区 |
| D1-S15 | PresentTaskProgress | `D1_Present_Task_Progress` | 出站 → IM adapter tools/worker 区 |
| D1-S16 | DeliverConclusion | `D1_Deliver_Conclusion` | 出站 → IM adapter conclusion + PublishCritical 必达 |
| D1-S17 | ConnectChannel | `D1_Connect_Channel` | 横切 — 多 IM adapter + 编解码 |
| D1-S18 | GuaranteeDelivery | `D1_Guarantee_Delivery` | 横切 — EventBus Critical 路径 |

### D1-S13: CaptureUserIntent

| A ID | Name | Kind | Input | Output | Legacy 映射 |
|------|------|------|-------|--------|-------------|
| D1-S13-A01 | AcceptInboundMessage | USER | raw IM/CLI | InboundMessage | S2-A01 ParseInbound |
| D1-S13-A02 | PersistUserTurn | SYSTEM | InboundMessage | persisted | S1-A01 ManageSession |
| D1-S13-A03 | DispatchToAgent | USER | session, content | event_chan | S1-A02 RouteMessage；S1-A04 RouteAgent **→ SUPERSEDED** `bootstrap/sessionagents` |
| D1-S13-A04 | ResolvePermissionGate | USER | tool, risk | approved | S1-A03 ResolvePermission |
| D1-S13-A05 | ParseCommand | USER | /new /stop /help | command | S3-A01 ParseCommand |

### D1-S14: PresentThinking

| A ID | Name | Kind | Input | Output | Legacy 映射 |
|------|------|------|-------|--------|-------------|
| D1-S14-A01 | EmitThinkingDelta | SYSTEM | EngineEvent | IMOutboundSignal(Thinking) | S2-A02 SendOutbound (thinking 区) |

### D1-S15: PresentTaskProgress

| A ID | Name | Kind | Input | Output | Legacy 映射 |
|------|------|------|-------|--------|-------------|
| D1-S15-A01 | EmitToolProgress | SYSTEM | tool_call/result | IMOutboundSignal(Task) | S2-A02 SendOutbound (tools) |
| D1-S15-A02 | EmitWorkerProgress | SYSTEM | WorkerEvent | IMOutboundSignal(Task) | S2-A03 RenderWorkerCard, S5-A01 TrackMilestone |

### D1-S16: DeliverConclusion

| A ID | Name | Kind | Input | Output | Legacy 映射 |
|------|------|------|-------|--------|-------------|
| D1-S16-A01 | EmitSummaryChunk | SYSTEM | text delta | IMOutboundSignal(Conclusion) | S2-A02/S2-A04 StreamCardContent |
| D1-S16-A02 | FinalizeReply | SYSTEM | complete/error | IMOutboundSignal(Conclusion) | S1-A01 summary, S2-A02 complete |

### D1-S17: ConnectChannel

| A ID | Name | Kind | Input | Output | Legacy 映射 |
|------|------|------|-------|--------|-------------|
| D1-S17-A01 | ParseFeishuInbound | USER | raw_card | InboundMessage | S2-A01-F01 |
| D1-S17-A02 | ParseDingTalkInbound | USER | webhook_body | InboundMessage | S2-A01-F03 |
| D1-S17-A03 | ParseCLIInbound | USER | stdin_line | InboundMessage | S2-A01-F02 |
| D1-S17-A04 | ManageConnection | INTERNAL | instance_id | connection_state | S10-A01 ManageConnection |
| D1-S17-A05 | RegisterInstance | INTERNAL | instance_spec | instance_id | S12-A01 RegisterInstance |
| D1-S17-A06 | CheckRateLimit | INTERNAL | adapter_id | allowed/denied | S6-A01 CheckRateLimit |

### D1-S18: GuaranteeDelivery

| A ID | Name | Kind | Input | Output | Legacy 映射 |
|------|------|------|-------|--------|-------------|
| D1-S18-A01 | DeliverOutboundSignal | SYSTEM | *Event | queued/delivered | S9-A01 PublishEvent, S9-A02 ManageBusLifecycle |

---

## Legacy Module Index — D1-S1–S12（ARCHIVED — 仅追溯）

> **完整 Legacy 索引与路径映射** 见 `layer-delta.md` §Legacy Archive（DM-20260628-003）。  
> 新能力 **禁止** 登记 Legacy S1–S12；仅 Canonical S13–S18。

## D1-S1: Gateway

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D1-S1-A01 | ManageSession | A-BE | session_id, action | session_state | session.created / closed / expired | `capture/store.go`, `capture/session.go` |
| D1-S1-A02 | RouteMessage | A-BE | message, session | routing_result, events | session.last_activity | `capture/ingress.go`, `capture/gateway.go` |
| D1-S1-A03 | ResolvePermission | A-BE | permission_request | approved/denied | permission.resolved | `capture/permission.go` |
| D1-S1-A04 | RouteAgent | A-BE | agent_factory, session | agent_registered | session.agent_bound | **SUPERSEDED** → `bootstrap/sessionagents/manager.go`（原 `capture/agent_route.go` 已删除，DM-20260628-003；D1-RF-T02） |

## D1-S2: Adapters

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D1-S2-A01 | ParseInbound | A-BE | raw_message (IM/CLI) | parsed_message | — | `adapters/feishu.go`, `adapters/dingtalk.go`, `adapters/cli.go` |
| D1-S2-A02 | SendOutbound | A-BE | message, session | delivery_result | — | `adapters/feishu.go`, `adapters/dingtalk_outbound.go`, `adapters/cli.go` |
| D1-S2-A03 | RenderWorkerCard | A-BE | worker_event, session | card_rendered | card.created / updated | `adapters/feishu_worker_card.go`（`contracts.WorkerStreamEvent`，零 D7 import） |
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

| Track | Scenarios | Activities |
|-------|-----------|------------|
| Canonical S13–S18 | 6 | 16 |
| Legacy S1–S12 | 12 | 21 |
