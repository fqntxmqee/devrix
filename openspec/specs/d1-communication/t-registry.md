# D1 Communication Domain — T 层测试点注册表

**Status:** Active
**Version:** 2.0.0
**Last Updated:** 2026-06-13
**Parent:** `openspec/specs/architecture/layering.md`

---

## D1-S1: Gateway Module

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| D1-S1-A01-T01 | 新会话创建被拒绝 | Gateway | `tests/acceptance/p0/comm_gateway_flow_test.go` | IMPLEMENTED | P0 |
| D1-S1-A01-T02 | IM 入口实例 Register/Unregister | Gateway | `internal/layers/communication/instance/registry_test.go` | IMPLEMENTED | P2 |
| D1-S1-A01-T03 | `buildCompletionSummary` 注入 ctx_pct 段（含/省略/clamp/异常） | Gateway | `internal/layers/communication/gateway/summary_test.go` | IMPLEMENTED | P1 |
| D1-S1-A01-T04 | `ComputeCtxPct` 边界：0 prompt / 0 max / 负数 / 超限 clamp | Gateway | `internal/layers/communication/gateway/summary_test.go` | IMPLEMENTED | P1 |
| D1-S1-A01-T05 | Session 完整生命周期：创建/恢复/过期/CleanupRoutine | Gateway | `internal/layers/communication/gateway/gateway_test.go` | IMPLEMENTED | P0 |
| D1-S1-A02-T01 | Inbound/Outbound 消息路由全链路 | Gateway | `internal/layers/communication/gateway/gateway_test.go` | IMPLEMENTED | P0 |
| D1-S1-A03-T01 | YOLO 模式 CRITICAL 风险永不自动审批 | Permission | `internal/layers/communication/gateway/permission_test.go` | IMPLEMENTED | P0 |
| D1-S1-A03-T02 | Permission Request/Resolve/ListPending 生命周期 | Permission | `internal/layers/communication/gateway/permission_test.go` | IMPLEMENTED | P1 |
| D1-S1-A04-T01 | AgentFactory 注入后路由走 Agent 路径 | Agent | `internal/layers/communication/gateway/gateway_test.go` | IMPLEMENTED | P1 |

## D1-S5: Milestone Module

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| D1-S5-A01-T01 | Milestone 环检测拒绝循环依赖 | Milestone | `internal/layers/communication/milestone/service_test.go` | IMPLEMENTED | P1 |
| D1-S5-A01-T02 | TaskFlow 多里程碑链顺序执行至完成 | Milestone | `internal/layers/communication/milestone/taskflow_test.go` | IMPLEMENTED | P1 |
| D1-S5-A01-T03 | Milestone CRUD 完整生命周期（create/duplicate/progress/complete/fail） | Milestone | `internal/layers/communication/milestone/service_test.go` | IMPLEMENTED | P1 |

## D1-S3: Commands Module

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| D1-S3-A01-T01 | /new 命令解析正确 | Commands | `tests/acceptance/p0/comm_commands_test.go` | IMPLEMENTED | P0 |
| D1-S3-A01-T02 | /help 命令解析正确 | Commands | `tests/acceptance/p0/comm_commands_test.go` | IMPLEMENTED | P1 |
| D1-S3-A01-T03 | /stop 命令解析正确 | Commands | `tests/acceptance/p0/comm_commands_test.go` | IMPLEMENTED | P1 |

## D1-S8: Renderers Module

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| D1-S8-A01-T01 | ShortId 唯一且排除异议字符 | Renderers | `internal/shared/types/shortid_test.go` | IMPLEMENTED | P1 |
| D1-S8-A01-T02 | ProgressBar / StatusBadge 渲染输出合法 | Renderers | `internal/layers/communication/renderers/components_test.go` | IMPLEMENTED | P2 |
| D1-S8-A01-T03 | CLIRenderer 覆盖全消息类型（text/streaming/error/tool/progress/complete） | Renderers | `internal/layers/communication/renderers/message_test.go` | IMPLEMENTED | P1 |

## D1-S2: Adapters Module

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| D1-S2-A01-T01 | 飞书消息解析正确 | Adapters | `internal/layers/communication/adapters/feishu_test.go` | IMPLEMENTED | P1 |
| D1-S2-A01-T02 | 钉钉 Webhook 入站路由 + Session 出站 | Adapters | `internal/layers/communication/adapters/dingtalk_test.go` | IMPLEMENTED | P1 |
| D1-S2-A02-T03 | 钉钉 milestone 出站走 CardRenderer | Adapters | `internal/layers/communication/adapters/dingtalk_test.go` | IMPLEMENTED | P1 |
| D1-S2-A02-T04 | Cardkit 双步发卡成功 | Adapters | `internal/layers/communication/adapters/feishu_streaming_test.go` | IMPLEMENTED | P0 |
| D1-S2-A02-T05 | 元素级流式 PUT sequence 递增 | Adapters | `internal/layers/communication/adapters/feishu_streaming_test.go` | IMPLEMENTED | P0 |
| D1-S2-A02-T06 | cardkit 失败降级 Patch | Adapters | `internal/layers/communication/adapters/feishu_streaming_test.go` | IMPLEMENTED | P0 |
| D1-S2-A02-T07 | complete 关闭 streaming_mode | Adapters | `internal/layers/communication/adapters/feishu_streaming_test.go` | IMPLEMENTED | P0 |
| D1-S2-A02-T08 | 流式节流配置生效 | Adapters | `internal/layers/communication/adapters/feishu_streaming_test.go` | IMPLEMENTED | P1 |
| D1-S2-A02-T09 | /stop 保留 session 映射、清理流状态、新入站复用 | Adapters | `internal/layers/communication/adapters/stop_session_flow_test.go` | IMPLEMENTED | P0 |
| D1-S2-A03-T01 | WorkerCard 双块流式创建/更新/独立卡片隔离 | Adapters | `internal/layers/communication/adapters/feishu_worker_card_test.go` | IMPLEMENTED | P0 |
| D1-S2-A03-T02 | WorkerCard 终结更新内容完整、关闭幂等 | Adapters | `internal/layers/communication/adapters/feishu_worker_card_test.go` | IMPLEMENTED | P1 |
| D1-S2-A04-T01 | Cardkit CreateCard/StreamElementContent/UpdateCard 正常路径 | Adapters | `internal/layers/communication/adapters/feishu_cardkit_test.go` | IMPLEMENTED | P0 |
| D1-S2-A04-T02 | Cardkit 流关闭/限速错误 sentinel 正确返回 | Adapters | `internal/layers/communication/adapters/feishu_cardkit_test.go` | IMPLEMENTED | P1 |
| D1-S2-A05-T01 | Session 内存缓存/磁盘恢复/新建三级回退 | Adapters | `internal/layers/communication/adapters/session_resolve_test.go` | IMPLEMENTED | P1 |

## D1-S9: EventBus Module

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| D1-S9-A01-T01 | BackpressureEventBus 正常事件流不丢 | EventBus | `internal/layers/communication/eventbus/bus_test.go` | IMPLEMENTED | P0 |
| D1-S9-A02-T02 | 背压触发 Drain 排空非关键事件 | EventBus | `internal/layers/communication/eventbus/drain_test.go` | IMPLEMENTED | P0 |
| D1-S9-A02-T03 | Compact 同类事件合并 | EventBus | `internal/layers/communication/eventbus/compact_test.go` | IMPLEMENTED | P0 |
| D1-S9-A02-T04 | Reconnect 重建通道后继续处理 | EventBus | `internal/layers/communication/eventbus/reconnect_test.go` | IMPLEMENTED | P0 |
| D1-S9-A01-T05 | Critical 事件（complete）必达不被丢弃 | EventBus | `internal/layers/communication/eventbus/bus_test.go` | IMPLEMENTED | P0 |
| D1-S9-A01-T06 | Error 事件必达不被丢弃 | EventBus | `internal/layers/communication/eventbus/bus_test.go` | IMPLEMENTED | P0 |
| D1-S9-A01-T07 | Publish 在高水位阻塞回压到上游 | EventBus | `internal/layers/communication/eventbus/bus_test.go` | IMPLEMENTED | P0 |

## D1-S10: Connection Module

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| D1-S10-A01-T01 | 连接心跳超时触发指数退避重连（1s → 60s，最多 10 次） | Connection | `internal/layers/communication/connection/manager_lifecycle_test.go` | IMPLEMENTED | P1 |
| D1-S10-A01-T02 | Register/Unregister/Count/Heartbeat 生命周期 | Connection | `internal/layers/communication/connection/manager_lifecycle_test.go` | IMPLEMENTED | P1 |

## D1-S6: RateLimit Module

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| D1-S6-A01-T01 | 令牌桶 Allow/Deny/Remaining 正确计数 | RateLimit | `internal/layers/communication/ratelimit/limiter_test.go` | IMPLEMENTED | P1 |
| D1-S6-A01-T02 | 不同 adapter 独立限流桶互不影响 | RateLimit | `internal/layers/communication/ratelimit/limiter_test.go` | IMPLEMENTED | P1 |
| D1-S6-A01-T03 | HTTP Middleware 返回正确限流头（Retry-After/X-RateLimit-*） | RateLimit | `internal/layers/communication/ratelimit/limiter_test.go` | IMPLEMENTED | P1 |

---

## Statistics

| Total | IMPLEMENTED | P0 | P1 | P2 |
|-------|-------------|----|----|-----|
| 44 | 44 | 19 | 23 | 2 |
