# D1 Communication Domain — T 层测试点注册表

**Status:** Active
**Version:** 3.2.0
**Last Updated:** 2026-06-28
**Parent:** `openspec/specs/architecture/layering.md`
**Domain SoT:** `openspec/specs/d1-communication/d1-domain.md`
**Change:** DM-20260614-006 — 切法 A 双轨 / … / **DM-20260628-003 (devrix-d1-dsaft-refactor) — DSAFT 边界 + Gateway 拆分 + contracts DTO + lint-d1-imports CI**

---

## Overview

- **Canonical SoT：** 价值流 `D1-S13–S18`（见 `layering.md` v3.4+）
- **Legacy T ID：** 44 条 IMPLEMENTED 测试注释 **v1.0 不改**；本表 **Canonical** 列指向终态 S/A/F
- **按 S 分组摘要 / P0 Runbook / 必达演练：** 见 `observability-guide.md` §4–§6（本文保留全表登记）

---

## Canonical T（v2.0 — 全部 IMPLEMENTED）

| T ID | 描述 | Canonical S/A | Test 位置 | Status | Priority |
|------|------|---------------|-----------|--------|----------|
| D1-S13-A02-T01 | 入站用户 turn 持久化成功 | S13-A02 PersistUserTurn | `tests/acceptance/p0/d1_signal_journey_test.go` | IMPLEMENTED | P0 |
| D1-S13-A03-T01 | Dispatch F02 走 D7 路径 | S13-A03-F02 routeD7 | `capture/coordinator_matrix_test.go` | IMPLEMENTED | P0 |
| D1-S13-A03-T02 | 未配置 orchestration entry 时 RouteInbound 失败 | S13-A03 DispatchToAgent | `capture/coordinator_integration_test.go` | IMPLEMENTED | P0 |
| D1-S13-A04-T01 | YOLO 模式 CRITICAL 风险永不自动审批 | S13-A04 ResolvePermissionGate | `capture/permission_test.go` | IMPLEMENTED | P0 |
| D1-S14-A01-F01-T01 | thinking 首 chunk（信号映射 + 锚点） | S14-A01-F01 map→Signal(Thinking) | `tests/acceptance/p0/d1_signal_journey_test.go` | IMPLEMENTED | P0 |
| D1-S15-A01-F01-T01 | tool_call/result 映射为 Task 信号 | S15-A01 EmitToolProgress | `tests/acceptance/p0/d1_signal_journey_test.go` | IMPLEMENTED | P1 |
| D1-S15-A02-F01-T01 | Worker 卡双块流式隔离 | S15-A02-F02 worker card encode | `channel/adapters/feishu_worker_card_test.go` | IMPLEMENTED | P0 |
| D1-S16-A02-T01 | complete costly 信号必达用户 | S16-A02 FinalizeReply | `tests/acceptance/p0/d1_signal_journey_test.go` | IMPLEMENTED | P0 |
| D1-S16-A02-T02 | error costly 信号必达用户 | S16-A02 FinalizeReply | `tests/acceptance/p0/d1_signal_journey_test.go` | IMPLEMENTED | P0 |
| D1-S17-A01-T01 | 飞书入站 Parse 与 canonical 信号链兼容 | S17-A01 ParseFeishuInbound | `channel/adapters/feishu_test.go` | IMPLEMENTED | P1 |
| D1-S18-A01-F02-T01 | PublishCritical complete/error 不被 Drain | S18-A01-F02 PublishCritical | `internal/layers/communication/delivery/eventbus/bus_test.go` | IMPLEMENTED | P0 |
| D1-S18-A01-F03-T01 | Drain 排空非 Critical 且不丢 Critical | S18-A01-F03 Drain | `internal/layers/communication/delivery/eventbus/drain_test.go` | IMPLEMENTED | P0 |
| D1-S19-A01-T01 | Transcript Writer 追加 + 读回 (NDJSON) | S19-A01 PersistTranscript | `internal/layers/communication/capture/transcript/writer_test.go` | IMPLEMENTED | P0 |
| D1-S19-A01-T02 | Transcript ListSessions 按 mtime 倒序 | S19-A01 ListSessions | `internal/layers/communication/capture/transcript/writer_test.go` | IMPLEMENTED | P0 |
| D1-S19-A01-T03 | Transcript path traversal 防御 | S19-A01 SanitizeFilename | `internal/layers/communication/capture/transcript/writer_test.go` | IMPLEMENTED | P0 |
| D1-S19-A01-T04 | Transcript 并发 100 goroutine 追加无丢行 | S19-A01 ConcurrentAppend | `internal/layers/communication/capture/transcript/writer_test.go` | IMPLEMENTED | P1 |
| **D1-S5-A07-T05** | **AC5 feishu table-count precheck: `countTableTags` 命中 `<table>` 标签后 SendCard fall back to plain text (ErrCode 11310 防护)** | **S5-A07 SendCard (extended)** | **`internal/layers/communication/channel/adapters/{card_precheck,feishu_card_precheck,feishu}_test.go`** | **IMPLEMENTED (DM-20260620-001)** | **P0** |
| **D1-S5-A07-T06** | **AC5 feishu size precheck: 序列化 JSON 超 28KB 触发 fall back to truncated plain text (Feishu 30KB hard limit 防护)** | **S5-A07 SendCard (extended)** | **`card_precheck_test.go::TestFeishuTableCountPrecheck_SizeOverLimit`, `feishu_test.go::TestSendCard_SizeFallback`** | **IMPLEMENTED (DM-20260620-001)** | **P0** |
| **D1-S5-A07-T07** | **AC5 default precheck wired in NewFeishuAdapter: `cardPrecheck = NewFeishuTableCountPrecheck(DefaultCardPrecheckConfig())` (MaxTables=5, MaxChars=28000)** | **S5-A07 NewFeishuAdapter (extended)** | **`feishu_test.go::TestNewFeishuAdapter_DefaultPrecheckWired`** | **IMPLEMENTED (DM-20260620-001)** | **P0** |
| **D1-S5-A07-T08** | **AC5 plain-text fallback preserves header + markdown: `cardFallbackText` returns title + content + "[card auto-flattened]" trailer** | **S5-A07 cardFallbackText (new)** | **`feishu_test.go::TestCardFallbackText_{HeaderAndMarkdown,FlattensPipeTable,TrailerMarker}`** | **IMPLEMENTED (DM-20260620-001)** | **P0** |
| **D1-S3-A08-T01** | **feishu / cli IM 适配器基于 `Event.Metadata["error_code"]` 走差异化文案（5 类 code 各自独立文案 + 兜底 Unknown）：RateLimit → "模型繁忙，请稍候重试" / AuthenticationFailed → "API key 失效，请检查 ~/.devrix/config.yaml" / PromptTooLong → "会话过长，已尝试压缩" / MediaSize + ImageSize → "附件过大" / ServerError → "模型服务异常" / Unknown → 现有通用文案** | **D1-S3-A08 EmitError (error_code → 文案映射)** | **`internal/layers/communication/{feishu,cli}_test.go::TestEmitError_{RateLimit,AuthenticationFailed,PromptTooLong,MediaSize,ImageSize,ServerError,Unknown}_MessageMapping` (7 sub-test)** | **IMPLEMENTED (DM-20260628-001 S5 验收 PR #265，feishu + cli 单测 7 sub-case 全 PASS)** | **P0** |
| **D1-RF-T01** | **D1 capture 生产代码禁止 import multiagent / orchestration/** | **S13 边界 (DM-20260628-003)** | **`capture/import_boundary_test.go`** | **IMPLEMENTED** | **P0** |
| **D1-RF-T02** | **beforeDispatch + D7：leader Created 且 ProcessMessage 仅 1 次** | **S13-A03 + bootstrap hook** | **`coordinator_integration_test.go`, `tests/acceptance/p0/d1_dsaft_refactor_test.go`** | **IMPLEMENTED** | **P0** |
| **D1-RF-T03** | **permission_required → RoutePermission → ResolveAgentPermission** | **S13-A04 + sessionagents** | **`bootstrap/sessionagents/manager_test.go`** | **IMPLEMENTED** | **P0** |
| **D1-RF-T04** | **无 active turn 时 orphan EngineEvent 到达 DeliverOrphanEngineEvent** | **bootstrap sessionagents sink** | **`bootstrap/sessionagents/manager_test.go`** | **IMPLEMENTED** | **P1** |
| **D1-RF-T05** | **orchestrationEntry nil 时 RouteInbound 失败（hook 不 bypass）** | **S13-A03** | **`capture/coordinator_integration_test.go`** | **IMPLEMENTED** | **P0** |
| **D1-RF-T06** | **text delta → Conclusion 非终态（独立 signal journey）** | **S16-A01 EmitSummaryChunk** | **`tests/acceptance/p0/d1_signal_journey_test.go::TestL5_D1_SignalJourney_TextDeltaConclusion`** | **IMPLEMENTED** | **P1** |
| **D1-RF-T07** | **milestone_progress → Task presenter 单测** | **S15-A01 EmitToolProgress** | **`capture/signal_router_test.go::TestSignalRouter_Dispatch_MilestoneProgress`** | **IMPLEMENTED** | **P1** |
| **D1-RF-T08** | **Gateway 拆分后 ingress/outbound 包级测试锚点不变** | **S13 + S14–S16** | **`capture/gateway_test.go`, `capture/coordinator_*_test.go`** | **IMPLEMENTED** | **P1** |
| **D1-RF-T09** | **channel/adapters 生产代码禁止 import orchestration/** | **S17 边界** | **`channel/adapters/import_boundary_test.go`** | **IMPLEMENTED** | **P0** |

---

## Legacy T（ARCHIVED — 追溯用，v2.0 测试注释已迁移 canonical ID）

### D1-S1: Gateway Module

| T ID | 描述 | Legacy S | Canonical S/A/F | Test 位置 | Status | Priority |
|------|------|----------|-----------------|-----------|--------|----------|
| D1-S1-A01-T01 | 新会话创建被拒绝 | Gateway | S13-A02-F01 CreateSession | `tests/acceptance/p0/comm_gateway_flow_test.go` | IMPLEMENTED | P0 |
| D1-S1-A01-T02 | IM 入口实例 Register/Unregister | Gateway | S17-A05 RegisterInstance | `internal/layers/communication/channel/instance/registry_test.go` | IMPLEMENTED | P2 |
| D1-S1-A01-T03 | `buildCompletionSummary` 注入 ctx_pct 段 | Gateway | S16-A02-F buildCompletionSummary | `internal/layers/communication/capture/summary_test.go` | IMPLEMENTED | P1 |
| D1-S1-A01-T04 | `ComputeCtxPct` 边界 clamp | Gateway | S16-A02-F ComputeCtxPct | `internal/layers/communication/capture/summary_test.go` | IMPLEMENTED | P1 |
| D1-S1-A01-T05 | Session 完整生命周期 | Gateway | S13-A02-F sessionStore | `internal/layers/communication/capture/gateway_test.go` | IMPLEMENTED | P0 |
| D1-S1-A02-T01 | Inbound/Outbound 消息路由全链路 | Gateway | S13-A03 + S14/S15/S16 | `internal/layers/communication/capture/gateway_test.go` | IMPLEMENTED | P0 |
| D1-S1-A03-T01 | YOLO 模式 CRITICAL 风险永不自动审批 | Permission | S13-A04 ResolvePermissionGate | `internal/layers/communication/capture/permission_test.go` | IMPLEMENTED | P0 |
| D1-S1-A03-T02 | Permission Request/Resolve/ListPending 生命周期 | Permission | S13-A04 ResolvePermissionGate | `internal/layers/communication/capture/permission_test.go` | IMPLEMENTED | P1 |
| D1-S1-A04-T01 | AgentFactory 注入后路由走 Agent 路径 | Agent | S13-A03-F03 routeAgent | `internal/layers/communication/capture/gateway_test.go` | **SUPERSEDED** → D1-RF-T02 (`d1_dsaft_refactor_test.go`, `sessionagents`) | P1 |

### D1-S5: Milestone Module

| T ID | 描述 | Legacy S | Canonical S/A/F | Test 位置 | Status | Priority |
|------|------|----------|-----------------|-----------|--------|----------|
| D1-S5-A01-T01 | Milestone 环检测拒绝循环依赖 | Milestone | S15-A01-F + D7（v2.0 迁 D7） | `internal/layers/orchestration/milestone/service_test.go` | IMPLEMENTED | P1 |
| D1-S5-A01-T02 | TaskFlow 多里程碑链顺序执行至完成 | Milestone | S15-A01-F + D7 TaskFlow | `internal/layers/orchestration/milestone/taskflow_test.go` | IMPLEMENTED | P1 |
| D1-S5-A01-T03 | Milestone CRUD 完整生命周期 | Milestone | S15-A01-F EmitMilestoneProgress | `internal/layers/orchestration/milestone/service_test.go` | IMPLEMENTED | P1 |

### D1-S3: Commands Module

| T ID | 描述 | Legacy S | Canonical S/A/F | Test 位置 | Status | Priority |
|------|------|----------|-----------------|-----------|--------|----------|
| D1-S3-A01-T01 | /new 命令解析正确 | Commands | S13-A05 ParseCommand | `tests/acceptance/p0/comm_commands_test.go` | IMPLEMENTED | P0 |
| D1-S3-A01-T02 | /help 命令解析正确 | Commands | S13-A05 ParseCommand | `tests/acceptance/p0/comm_commands_test.go` | IMPLEMENTED | P1 |
| D1-S3-A01-T03 | /stop 命令解析正确 | Commands | S13-A05 ParseCommand | `tests/acceptance/p0/comm_commands_test.go` | IMPLEMENTED | P1 |

### D1-S8: Renderers Module

| T ID | 描述 | Legacy S | Canonical S/A/F | Test 位置 | Status | Priority |
|------|------|----------|-----------------|-----------|--------|----------|
| D1-S8-A01-T01 | ShortId 唯一且排除异议字符 | Renderers | S17 Encode F | `internal/shared/types/shortid_test.go` | IMPLEMENTED | P1 |
| D1-S8-A01-T02 | ProgressBar / StatusBadge 渲染输出合法 | Renderers | S14–S16 Encode F | `internal/layers/communication/channel/renderers/components_test.go` | IMPLEMENTED | P2 |
| D1-S8-A01-T03 | CLIRenderer 覆盖全消息类型 | Renderers | S17-A03 + Encode CLI | `internal/layers/communication/channel/renderers/message_test.go` | IMPLEMENTED | P1 |

### D1-S2: Adapters Module

| T ID | 描述 | Legacy S | Canonical S/A/F | Test 位置 | Status | Priority |
|------|------|----------|-----------------|-----------|--------|----------|
| D1-S2-A01-T01 | 飞书消息解析正确 | Adapters | S17-A01 / S13-A01-F01 | `internal/layers/communication/channel/adapters/feishu_test.go` | IMPLEMENTED | P1 |
| D1-S2-A01-T02 | 钉钉 Webhook 入站路由 + Session 出站 | Adapters | S17-A02 ParseDingTalkInbound | `internal/layers/communication/channel/adapters/dingtalk_test.go` | IMPLEMENTED | P1 |
| D1-S2-A02-T03 | 钉钉 milestone 出站走 CardRenderer | Adapters | S15-A02 + S17 Encode | `internal/layers/communication/channel/adapters/dingtalk_test.go` | IMPLEMENTED | P1 |
| D1-S2-A02-T04 | Cardkit 双步发卡成功 | Adapters | S16-A01 + S17 EncodeFeishuCardKit | `internal/layers/communication/channel/adapters/feishu_streaming_test.go` | IMPLEMENTED | P0 |
| D1-S2-A02-T05 | 元素级流式 PUT sequence 递增 | Adapters | S16-A01 + S17 EncodeFeishuCardKit | `internal/layers/communication/channel/adapters/feishu_streaming_test.go` | IMPLEMENTED | P0 |
| D1-S2-A02-T06 | cardkit 失败降级 Patch | Adapters | S16-A01 + S17 EncodeFeishuCardKit | `internal/layers/communication/channel/adapters/feishu_streaming_test.go` | IMPLEMENTED | P0 |
| D1-S2-A02-T07 | complete 关闭 streaming_mode | Adapters | S16-A02 + S17 EncodeFeishuCardKit | `internal/layers/communication/channel/adapters/feishu_streaming_test.go` | IMPLEMENTED | P0 |
| D1-S2-A02-T08 | 流式节流配置生效 | Adapters | S16-A01 + S17 Encode | `internal/layers/communication/channel/adapters/feishu_streaming_test.go` | IMPLEMENTED | P1 |
| D1-S2-A02-T09 | /stop 保留 session 映射、清理流状态 | Adapters | S13-A02 + S16-A02 | `internal/layers/communication/channel/adapters/stop_session_flow_test.go` | IMPLEMENTED | P0 |
| D1-S2-A03-T01 | WorkerCard 双块流式创建/更新/隔离 | Adapters | S15-A02 + S17 EncodeFeishuWorkerCard | `internal/layers/communication/channel/adapters/feishu_worker_card_test.go` | IMPLEMENTED | P0 |
| D1-S2-A03-T02 | WorkerCard 终结更新内容完整、关闭幂等 | Adapters | S15-A02 + S17 EncodeFeishuWorkerCard | `internal/layers/communication/channel/adapters/feishu_worker_card_test.go` | IMPLEMENTED | P1 |
| D1-S2-A04-T01 | Cardkit CreateCard/Stream/Update 正常路径 | Adapters | S17 Encode F | `internal/layers/communication/channel/adapters/feishu_cardkit_test.go` | IMPLEMENTED | P0 |
| D1-S2-A04-T02 | Cardkit 流关闭/限速错误 sentinel | Adapters | S17 Encode F | `internal/layers/communication/channel/adapters/feishu_cardkit_test.go` | IMPLEMENTED | P1 |
| D1-S2-A05-T01 | Session 内存缓存/磁盘恢复/新建三级回退 | Adapters | S13-A02-F ResolveSession | `internal/layers/communication/channel/adapters/session_resolve_test.go` | IMPLEMENTED | P1 |

### D1-S9: EventBus Module

| T ID | 描述 | Legacy S | Canonical S/A/F | Test 位置 | Status | Priority |
|------|------|----------|-----------------|-----------|--------|----------|
| D1-S9-A01-T01 | BackpressureEventBus 正常事件流不丢 | EventBus | S18-A01-F01 Publish | `internal/layers/communication/delivery/eventbus/bus_test.go` | IMPLEMENTED | P0 |
| D1-S9-A02-T02 | 背压触发 Drain 排空非关键事件 | EventBus | S18-A01-F03 Drain | `internal/layers/communication/delivery/eventbus/drain_test.go` | IMPLEMENTED | P0 |
| D1-S9-A02-T03 | Compact 同类事件合并 | EventBus | S18-A01-F04 Compact | `internal/layers/communication/delivery/eventbus/compact_test.go` | IMPLEMENTED | P0 |
| D1-S9-A02-T04 | Reconnect 重建通道后继续处理 | EventBus | S18-A01-F05 Reconnect | `internal/layers/communication/delivery/eventbus/reconnect_test.go` | IMPLEMENTED | P0 |
| D1-S9-A01-T05 | Critical 事件（complete）必达不被丢弃 | EventBus | S18-A01-F02 PublishCritical | `internal/layers/communication/delivery/eventbus/bus_test.go` | IMPLEMENTED | P0 |
| D1-S9-A01-T06 | Error 事件必达不被丢弃 | EventBus | S18-A01-F02 PublishCritical | `internal/layers/communication/delivery/eventbus/bus_test.go` | IMPLEMENTED | P0 |
| D1-S9-A01-T07 | Publish 在高水位阻塞回压到上游 | EventBus | S18-A01-F01 Publish | `internal/layers/communication/delivery/eventbus/bus_test.go` | IMPLEMENTED | P0 |

### D1-S10: Connection Module

| T ID | 描述 | Legacy S | Canonical S/A/F | Test 位置 | Status | Priority |
|------|------|----------|-----------------|-----------|--------|----------|
| D1-S10-A01-T01 | 连接心跳超时触发指数退避重连 | Connection | S17-A04 ManageConnection | `internal/layers/communication/channel/connection/manager_lifecycle_test.go` | IMPLEMENTED | P1 |
| D1-S10-A01-T02 | Register/Unregister/Count/Heartbeat 生命周期 | Connection | S17-A04 ManageConnection | `internal/layers/communication/channel/connection/manager_lifecycle_test.go` | IMPLEMENTED | P1 |

### D1-S6: RateLimit Module

| T ID | 描述 | Legacy S | Canonical S/A/F | Test 位置 | Status | Priority |
|------|------|----------|-----------------|-----------|--------|----------|
| D1-S6-A01-T01 | 令牌桶 Allow/Deny/Remaining 正确计数 | RateLimit | S17-A06 CheckRateLimit | `internal/layers/communication/channel/ratelimit/limiter_test.go` | IMPLEMENTED | P1 |
| D1-S6-A01-T02 | 不同 adapter 独立限流桶互不影响 | RateLimit | S17-A06 CheckRateLimit | `internal/layers/communication/channel/ratelimit/limiter_test.go` | IMPLEMENTED | P1 |
| D1-S6-A01-T03 | HTTP Middleware 返回正确限流头 | RateLimit | S17-A06 CheckRateLimit | `internal/layers/communication/channel/ratelimit/limiter_test.go` | IMPLEMENTED | P1 |

---

## Statistics

| Category | Total | IMPLEMENTED | PLANNED | P0 |
|----------|-------|-------------|---------|-----|
| Legacy T | 44 | 44 | 0 | 19 |
| Canonical T（新增） | 16 | 16 | 0 | 11 |
| DM-20260628-001（api-error-classification, +1 P0 T01） | 1 | 1 | 0 | 1 |
| **合计** | **61** | **61** | **0** | **31** |

> **DM-20260620-001 (devrix-context-budget-and-isolation, Phase A) — AC5 feishu precheck 新增 4 项 P0 T 点 T05-T08（16 - 12 = 4）**，全部 IMPLEMENTED。
> **DM-20260628-001 (devrix-api-error-classification) — AC5 飞书/cli IM 适配器 error_code 差异化文案 +1 P0 T (D1-S3-A08-T01)**，S5 验收 PR #265 IMPLEMENTED。
