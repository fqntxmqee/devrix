# D1 Communication Domain — T 层测试点注册表

**Status:** Active
**Version:** 1.0.0
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

## D1-S5: Milestone Module

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| D1-S5-A01-T01 | Milestone 环检测拒绝循环依赖 | Milestone | `internal/layers/communication/milestone/service_test.go` | IMPLEMENTED | P1 |
| D1-S5-A01-T02 | TaskFlow 多里程碑链顺序执行至完成 | Milestone | `internal/layers/communication/milestone/taskflow_test.go` | IMPLEMENTED | P1 |
| D1-S5-A01-T03 | 无 V1 TaskFlow stub 误导日志 | Milestone | — (file removed) | IMPLEMENTED | P2 |

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

---

## Statistics

| Total | IMPLEMENTED | P0 |
|-------|-------------|-----|
| 22 | 22 | 8 |
