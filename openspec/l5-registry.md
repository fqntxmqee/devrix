# Devrix L5 测试点注册表

**Status:** Active
**Last Updated:** 2026-06-07 (CTX 草案登记)

> L5 测试点是 OpenSpec S5 验收的确定性锚点。新增 L4 能力前 MUST 先在此登记或复用已有 L5。

---

## 登记规则

| 字段 | 说明 |
|------|------|
| ID | `L5-{LAYER}-{NN}`，LAYER = COMM / CTX / LLM / AGENT / OBS / EVO |
| Priority | P0（阻断交付）/ P1（需执行，失败记例外）/ P2（尽力） |
| L4 映射 | 关联的功能点 ID |
| Test 位置 | 测试文件路径 |
| Status | PLANNED / IMPLEMENTED / DEPRECATED |

---

## Layer 1: Communication (COMM)

### P0

| L5 ID | 描述 | L4 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-COMM-01 | 运行 devrix 创建 CLI 会话 | L4-COMM-CLI | `tests/integration/gateway_session_test.go` | IMPLEMENTED |
| L5-COMM-02 | 空消息入站被拒绝 | L4-COMM-GW | `tests/acceptance/p0/comm_gateway_flow_test.go` | IMPLEMENTED |
| L5-COMM-03 | 会话空闲超时后拒绝消息 | L4-COMM-STORE | `tests/integration/gateway_session_test.go` | IMPLEMENTED |
| L5-COMM-04 | `/new` 命令解析正确 | L4-COMM-CMD | `tests/acceptance/p0/comm_commands_test.go` | IMPLEMENTED |
| L5-COMM-05 | `/help` 命令解析正确 | L4-COMM-CMD | `tests/acceptance/p0/comm_commands_test.go` | IMPLEMENTED |
| L5-COMM-06 | `/stop` 命令解析正确 | L4-COMM-CMD | `tests/acceptance/p0/comm_commands_test.go` | IMPLEMENTED |
| L5-COMM-07 | 权限请求超时自动拒绝 | L4-COMM-PERM | `tests/integration/permission_flow_test.go` | IMPLEMENTED |
| L5-COMM-08 | 入站消息经 Engine 产生出站响应 | L4-COMM-GW | `tests/acceptance/p0/comm_gateway_flow_test.go` | IMPLEMENTED |
| L5-COMM-17 | Milestone 循环依赖被拒绝 | L4-COMM-MS | `internal/layers/communication/milestone/service_test.go` | IMPLEMENTED |
| L5-COMM-18 | TaskFlow 按依赖顺序执行 | L4-COMM-TF | — | PLANNED |

### P1

| L5 ID | 描述 | L4 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-COMM-09 | ShortId 唯一且排除歧义字符 | L4-COMM-ID | `internal/shared/types/shortid_test.go` | IMPLEMENTED |
| L5-COMM-10 | 无效 Token 返回 401 | L4-COMM-AUTH | — | PLANNED |
| L5-COMM-11 | 飞书消息解析正确 | L4-COMM-FEISHU | `internal/layers/communication/adapters/feishu_test.go` | IMPLEMENTED |
| L5-COMM-12 | 限流超限拒绝请求 | L4-COMM-RL | `internal/layers/communication/ratelimit/limiter_test.go` | IMPLEMENTED |
| L5-COMM-19 | 飞书消息去重 | L4-COMM-FEISHU | `internal/layers/communication/adapters/feishu_test.go` | IMPLEMENTED |
| L5-COMM-20 | UI 组件跨平台渲染 | L4-COMM-UI | — | PLANNED |
| L5-COMM-24 | 飞书卡片 12 种颜色支持 | L4-COMM-FEISHU | `internal/layers/communication/adapters/feishu_card_test.go` | PLANNED |
| L5-COMM-25 | CardListItem 双列布局渲染 | L4-COMM-FEISHU | `internal/layers/communication/adapters/feishu_card_test.go` | PLANNED |
| L5-COMM-26 | CardSelect 下拉选择器渲染 | L4-COMM-FEISHU | `internal/layers/communication/adapters/feishu_card_test.go` | PLANNED |
| L5-COMM-27 | RenderText 降级回退 | L4-COMM-FEISHU | `internal/layers/communication/core/card_test.go` | PLANNED |
| L5-COMM-28 | 收到飞书消息立即返回 OK 确认 | L4-COMM-FEISHU | `internal/layers/communication/adapters/feishu_test.go` | PLANNED |
| L5-COMM-29 | done_emoji 在完成时添加到用户消息 | L4-COMM-FEISHU | `internal/layers/communication/adapters/feishu_test.go` | PLANNED |

### P2

| L5 ID | 描述 | L4 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-COMM-21 | Prometheus `/metrics` 可访问 | L4-COMM-METRICS | — | PLANNED |
| L5-COMM-22 | 多实例注册与健康检查 | L4-COMM-INST | — | PLANNED |
| L5-COMM-23 | 钉钉 Adapter 消息收发（Mock） | L4-COMM-DING | — | PLANNED |

### Live Only（不阻断 PR）

| L5 ID | 描述 | 环境变量 | Status |
|-------|------|----------|--------|
| L5-COMM-L01 | 飞书 WebSocket 真连收发 | `DEVRIX_FEISHU_APP_ID`, `DEVRIX_FEISHU_APP_SECRET` | PLANNED |
| L5-COMM-L02 | 端到端 Stub Engine 回复 | — | PLANNED |

---

## Layer 2: Context Engine (CTX)

> 设计变更：`openspec/changes/devrix-context-engine/`

### P0

| L5 ID | 描述 | L4 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-CTX-01 | 新会话初始化上下文与 system prompt | L4-CTX-STATE | `internal/layers/contextengine/memory/*_test.go` | PLANNED |
| L5-CTX-02 | 用户消息后历史正确追加 | L4-CTX-STATE | 同上 | PLANNED |
| L5-CTX-03 | 超 Token 阈值触发七步压缩 | L4-CTX-COMPRESS | `tests/acceptance/p0/ctx_compression_test.go` | PLANNED |
| L5-CTX-04 | TokenBlock 超限返回 ContextExceeded | L4-CTX-COMPRESS | `internal/layers/contextengine/compression/*_test.go` | PLANNED |
| L5-CTX-05 | ContextSnapshot 保存与恢复 | L4-CTX-MEMORY | `tests/integration/context_gateway_flow_test.go` | PLANNED |
| L5-CTX-06 | PEV Execute 调用 LLM 并流式输出 | L4-CTX-PEV | `tests/integration/context_gateway_flow_test.go` | PLANNED |
| L5-CTX-09 | EngineEvent 与通信层四流契约一致 | L4-CTX-STATE | `tests/integration/context_gateway_flow_test.go` | PLANNED |

### P1

| L5 ID | 描述 | L4 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-CTX-07 | 工具执行后 Verify basic 模式 | L4-CTX-PEV | `internal/layers/contextengine/pev/*_test.go` | PLANNED |
| L5-CTX-08 | V1 跳过 Autocompact 步骤 6 | L4-CTX-COMPRESS | `internal/layers/contextengine/compression/*_test.go` | PLANNED |

### P2

| L5 ID | 描述 | L4 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-CTX-10 | L3 长期记忆返回 NotImplemented | L4-CTX-MEMORY | `internal/layers/contextengine/memory/*_test.go` | PLANNED |

---

## Layer 3–6: 预留

| 前缀 | 层 | 状态 |
|------|-----|------|
| `L5-LLM-*` | LLM Gateway | 未登记 |
| `L5-LLM-*` | LLM Gateway | 未登记 |
| `L5-AGENT-*` | Multi-Agent | 未登记 |
| `L5-OBS-*` | Observability | 未登记 |
| `L5-EVO-*` | Evolution | 未登记 |

新增层时 MUST 在本表追加对应章节。

---

## Layer 5: Observability (OBS)

> 新增层，规格定义：`openspec/specs/observability_layer_delta.md`

### P0

| L5 ID | 描述 | L4 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-OBS-01 | Trace ID 在消息入口生成 | L4-OBS-TRACE | `internal/layers/observability/tracer/*_test.go` | PLANNED |
| L5-OBS-02 | Trace ID 传播至 LLM 调用 | L4-OBS-TRACE | `tests/acceptance/p0/obs_trace_propagation_test.go` | PLANNED |
| L5-OBS-03 | LLM 调用记录 latency/token metrics | L4-OBS-METRICS | `internal/layers/observability/metrics/*_test.go` | PLANNED |
| L5-OBS-04 | 结构化日志包含 traceId | L4-OBS-LOG | `internal/layers/observability/logger/*_test.go` | PLANNED |
| L5-OBS-05 | Graceful shutdown 刷写 traces | L4-OBS-SHUTDOWN | `internal/layers/observability/shutdown_test.go` | PLANNED |

### P1

| L5 ID | 描述 | L4 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-OBS-06 | Prometheus `/metrics` 端点可访问 | L4-OBS-EXPORTER | `tests/acceptance/p1/obs_prometheus_test.go` | PLANNED |
| L5-OBS-07 | Health endpoint 返回 observability 状态 | L4-OBS-HEALTH | `tests/acceptance/p1/obs_health_test.go` | PLANNED |
| L5-OBS-08 | OTLP exporter 导出到收集器 | L4-OBS-EXPORTER | `internal/layers/observability/exporter/otlp_test.go` | PLANNED |
| L5-OBS-09 | Label cardinality 被正确控制 | L4-OBS-METRICS | `internal/layers/observability/metrics/registry_test.go` | PLANNED |

### P2

| L5 ID | 描述 | L4 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-OBS-10 | Sampling 策略按配置生效 | L4-OBS-SAMPLING | `internal/layers/observability/tracer/sampler_test.go` | PLANNED |
| L5-OBS-11 | Secret redaction 在日志中生效 | L4-OBS-LOG | `internal/layers/observability/logger/redactor_test.go` | PLANNED |
| L5-OBS-12 | W3C traceparent 头部注入/提取 | L4-OBS-PROPAGATION | `internal/layers/observability/tracer/propagation_test.go` | PLANNED |

---

## Layer 6: Evolution (EVO)

| 前缀 | 层 | 状态 |
|------|-----|------|
| `L5-EVO-*` | Evolution | 未登记 |
