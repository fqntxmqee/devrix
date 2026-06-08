# Devrix L5 测试点注册表

**Status:** Active
**Last Updated:** 2026-06-07

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
| L5-COMM-01 | 运行 devrix 创建 CLI 会话 | L4-COMM-CLI | `tests/integration/gateway_session_test.go` | PLANNED |
| L5-COMM-02 | 空消息入站被拒绝 | L4-COMM-GW | `tests/acceptance/p0/comm_gateway_flow_test.go` | PLANNED |
| L5-COMM-03 | 会话空闲超时后拒绝消息 | L4-COMM-STORE | `tests/integration/gateway_session_test.go` | PLANNED |
| L5-COMM-04 | /new 命令解析正确 | L4-COMM-CMD | `tests/acceptance/p0/comm_commands_test.go` | PLANNED |
| L5-COMM-05 | /help 命令解析正确 | L4-COMM-CMD | `tests/acceptance/p0/comm_commands_test.go` | PLANNED |
| L5-COMM-06 | /stop 命令解析正确 | L4-COMM-CMD | `tests/acceptance/p0/comm_commands_test.go` | PLANNED |

### P1

| L5 ID | 描述 | L4 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-COMM-09 | ShortId 唯一且排除歧义字符 | L4-COMM-ID | `internal/shared/types/shortid_test.go` | PLANNED |
| L5-COMM-11 | 飞书消息解析正确 | L4-COMM-FEISHU | `internal/layers/communication/adapters/feishu_test.go` | PLANNED |

---

## Layer 2: Context Engine (CTX)

> V1 已归档：`openspec/archive/2026-06-07-devrix-context-engine/`
> V2 已归档：`openspec/archive/2026-06-07-devrix-context-engine-v2/`
> V3 规划中：`openspec/changes/devrix-context-engine-v3/`

### P0

| L5 ID | 描述 | L4 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-CTX-01 | 新会话初始化上下文与 system prompt | L4-CTX-STATE | `internal/layers/contextengine/memory/manager_test.go` | IMPLEMENTED |
| L5-CTX-02 | 用户消息后历史正确追加 | L4-CTX-STATE | `internal/layers/contextengine/memory/manager_test.go` | IMPLEMENTED |
| L5-CTX-03 | 超 Token 阈值触发七步压缩 | L4-CTX-COMPRESS | `tests/acceptance/p0/ctx_compression_test.go` | IMPLEMENTED |
| L5-CTX-04 | TokenBlock 超限返回 ContextExceeded | L4-CTX-COMPRESS | `internal/layers/contextengine/compression/pipeline_test.go` | IMPLEMENTED |
| L5-CTX-05 | ContextSnapshot 保存与恢复 | L4-CTX-MEMORY | `internal/layers/contextengine/snapshot/store_test.go` | IMPLEMENTED |
| L5-CTX-06 | PEV Execute 调用 LLM 并流式输出 | L4-CTX-PEV | `tests/integration/context_gateway_flow_test.go` | IMPLEMENTED |
| L5-CTX-07 | 工具执行后 Verify basic 模式 | L4-CTX-PEV | `internal/layers/contextengine/pev_engine.go` | IMPLEMENTED |
| L5-CTX-09 | EngineEvent 与通信层四流契约一致 | L4-CTX-STATE | `tests/integration/context_gateway_flow_test.go` | IMPLEMENTED |
| L5-CTX-11 | 权限批准/拒绝后 PEV 行为正确 | L4-CTX-PEV | `tests/integration/context_gateway_flow_test.go` | IMPLEMENTED |
| L5-CTX-12 | Autocompact 触发并降低 token | L4-CTX-COMPRESS | `internal/layers/contextengine/compression/autocompact_test.go` | IMPLEMENTED |
| L5-CTX-13 | Autocompact LLM 失败降级跳过 | L4-CTX-COMPRESS | `internal/layers/contextengine/compression/autocompact_test.go` | IMPLEMENTED |
| L5-CTX-14 | Verify commands 全部通过 | L4-CTX-PEV | `tests/integration/context_verify_commands_test.go` | IMPLEMENTED |
| L5-CTX-15 | Verify 命令失败触发重试 | L4-CTX-PEV | `internal/layers/contextengine/verify_runner_test.go` | IMPLEMENTED |
| L5-CTX-16 | Token 计数共享契约与 Gateway 对齐 | L4-CTX-STATE | `internal/layers/contextengine/token/counter_test.go` | IMPLEMENTED |
| L5-CTX-19 | Plan 生成有效 Milestone DAG | L4-CTX-PLAN | `tests/acceptance/p0/ctx_plan_longterm_test.go` | IMPLEMENTED |
| L5-CTX-20 | Milestone 按依赖拓扑序执行 | L4-CTX-PEV | `tests/integration/context_plan_milestone_test.go` | IMPLEMENTED |
| L5-CTX-21 | milestone_progress 事件正确发射 | L4-CTX-PEV | `tests/acceptance/p0/ctx_plan_longterm_test.go` | IMPLEMENTED |
| L5-CTX-22 | LongTerm Recall 注入上下文 | L4-CTX-MEMORY | `tests/acceptance/p0/ctx_plan_longterm_test.go` | IMPLEMENTED |
| L5-CTX-23 | LongTerm Store 持久化写入 | L4-CTX-MEMORY | `internal/layers/contextengine/memory/longterm_test.go` | IMPLEMENTED |

### P1

| L5 ID | 描述 | L4 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-CTX-08 | Autocompact 禁用时跳过步骤 6 | L4-CTX-COMPRESS | `internal/layers/contextengine/compression/pipeline_test.go` | IMPLEMENTED |
| L5-CTX-17 | 压缩/Verify 步骤可观测事件 | L4-CTX-OBS | `tests/integration/context_compression_obs_test.go` | IMPLEMENTED |
| L5-CTX-18 | 主路径接入真实 LLM Gateway | L4-CTX-STATE | `tests/integration/context_llm_gateway_test.go` | IMPLEMENTED |
| L5-CTX-24 | plan.enabled=false 时回退 V2 路径 | L4-CTX-PEV | `tests/integration/context_plan_milestone_test.go` | IMPLEMENTED |
| L5-CTX-25 | Plan DAG 环检测拒绝无效图 | L4-CTX-PLAN | `internal/layers/contextengine/pev/plan_test.go` | IMPLEMENTED |

### P2

| L5 ID | 描述 | L4 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-CTX-10 | L3 长期记忆返回 NotImplemented | L4-CTX-MEMORY | `internal/layers/contextengine/memory/longterm_test.go` | IMPLEMENTED |

---

## Layer 3: LLM Gateway (LLM)

> 变更：`openspec/archive/2026-06-07-devrix-llm-gateway/`（Demand: DM-20260607-004，Status: S7 Archived）

### P0

| L5 ID | 描述 | L4 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-LLM-01 | DeepSeek 适配器流式响应 | L4-LLM-ADAPTER | `internal/layers/llmgateway/adapter/deepseek_test.go` | IMPLEMENTED |
| L5-LLM-02 | MiniMax 适配器流式响应 | L4-LLM-ADAPTER | `internal/layers/llmgateway/adapter/minimax_test.go` | IMPLEMENTED |
| L5-LLM-03 | Circuit breaker 正常关闭 | L4-LLM-BREAKER | `internal/layers/llmgateway/breaker/circuit_breaker_test.go` | IMPLEMENTED |
| L5-LLM-04 | Circuit breaker 触发开启 | L4-LLM-BREAKER | `internal/layers/llmgateway/breaker/circuit_breaker_test.go` | IMPLEMENTED |
| L5-LLM-05 | Circuit breaker 半开→关闭 | L4-LLM-BREAKER | `internal/layers/llmgateway/breaker/circuit_breaker_test.go` | IMPLEMENTED |
| L5-LLM-06 | Circuit breaker 半开→开启 | L4-LLM-BREAKER | `internal/layers/llmgateway/breaker/circuit_breaker_test.go` | IMPLEMENTED |
| L5-LLM-07 | Token 计数准确性 (cl100k_base) | L4-LLM-TOKEN | `internal/layers/llmgateway/token/counter_test.go` | IMPLEMENTED |
| L5-LLM-08 | Token 预算检查 | L4-LLM-TOKEN | `internal/layers/llmgateway/token/counter_test.go` | IMPLEMENTED |
| L5-LLM-09 | Provider 配置加载 | L4-LLM-CONFIG | `internal/layers/llmgateway/config/loader_test.go` | IMPLEMENTED |

### P1

| L5 ID | 描述 | L4 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-LLM-10 | DeepSeek Fallback 模型切换 | L4-LLM-RETRY | `tests/integration/llm_fallback_test.go` | IMPLEMENTED |
| L5-LLM-11 | MiniMax Fallback 模型切换 | L4-LLM-RETRY | `tests/integration/llm_fallback_test.go` | IMPLEMENTED |
| L5-LLM-12 | 重试策略执行 | L4-LLM-RETRY | `internal/layers/llmgateway/retry/retry_test.go` | IMPLEMENTED |
| L5-LLM-13 | LLM 调用可观测事件 | L4-LLM-OBS | `tests/integration/llm_observer_test.go` | IMPLEMENTED |
| L5-LLM-16 | 未知 Provider/Model 报错 | L4-LLM-GATEWAY | `internal/layers/llmgateway/gateway/router_test.go` | IMPLEMENTED |

### P2

| L5 ID | 描述 | L4 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-LLM-14 | 多 Provider 并发调用 | L4-LLM-GATEWAY | `internal/layers/llmgateway/gateway/gateway_test.go` | IMPLEMENTED |
| L5-LLM-15 | 熔断器状态持久化 | L4-LLM-BREAKER | — | PLANNED |

---

## Layer 4: Multi-Agent (AGENT)

### P0

| L5 ID | 描述 | L4 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-AGENT-01 | AgentFactory 创建 Agent 实例 | L4-AGENT-FACTORY | — | PLANNED |
| L5-AGENT-02 | Agent 生命周期状态转换 | L4-AGENT-LIFECYCLE | — | PLANNED |
| L5-AGENT-03 | 工具注册与风险等级 | L4-AGENT-REGISTRY | — | PLANNED |
| L5-AGENT-04 | Permission Pipeline 授权流程 | L4-AGENT-PERMISSION | — | PLANNED |

---

## Layer 5: Observability (OBS)

### P0

| L5 ID | 描述 | L4 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-OBS-01 | Tracing Span 创建与传播 | L4-OBS-TRACING | — | PLANNED |
| L5-OBS-02 | Metrics Counter 计数 | L4-OBS-METRICS | — | PLANNED |
| L5-OBS-03 | 日志级别过滤 | L4-OBS-LOGGING | — | PLANNED |
| L5-OBS-FIX-01 | Gauge Set/Inc/Dec/Add/Sub 数值正确 | L4-OBS-METRICS | `internal/layers/observability/metrics/gauge_test.go` | IMPLEMENTED |
| L5-OBS-FIX-02 | Histogram Prometheus 输出与 golden 一致 | L4-OBS-METRICS | `internal/layers/observability/metrics/histogram_test.go` | IMPLEMENTED |
| L5-OBS-FIX-03 | Shutdown 刷写所有 pending spans | L4-OBS-TRACING | `internal/layers/observability/tracer/tracer_test.go` | IMPLEMENTED |
| L5-OBS-FIX-04 | Shutdown 覆盖 Tracer + Logger | L4-OBS-LOGGING | `internal/layers/observability/logger/logger_test.go` | IMPLEMENTED |
| L5-OBS-FIX-05 | Int64UpDownCounter 返回 Gauge | L4-OBS-METRICS | `internal/layers/observability/metrics/meter_test.go` | IMPLEMENTED |
| L5-OBS-FIX-06 | Error 日志包含 stacktrace 字段 | L4-OBS-LOGGING | `internal/layers/observability/logger/logger_test.go` | IMPLEMENTED |
| L5-OBS-FIX-07 | 日志采样 max_entries_per_span 生效 | L4-OBS-LOGGING | `internal/layers/observability/logger/sampling_test.go` | IMPLEMENTED |
| L5-OBS-FIX-08 | ConsoleExporter 可直接作为 SpanExporter | L4-OBS-TRACING | `internal/layers/observability/exporter/console_test.go` | IMPLEMENTED |
| L5-OBS-13 | LongTerm recall/store 产生 canonical Operation span | L4-OBS-INSTRUMENT | `internal/layers/contextengine/engine.go` | IMPLEMENTED |
| L5-OBS-14 | Plan 生成与 Milestone Run 产生 canonical Operation span | L4-OBS-INSTRUMENT | `internal/layers/contextengine/pev_engine.go` | IMPLEMENTED |
| L5-OBS-15 | Feishu 入站产生 adapter.message.receive span | L4-OBS-INSTRUMENT | `tests/integration/obs_trace_propagation_test.go` | IMPLEMENTED |
| L5-OBS-16 | Operation Registry 与 names.go 常量全集一致 | L4-OBS-REGISTRY | `internal/layers/observability/coverage/registry_test.go` | IMPLEMENTED |
| L5-OBS-17 | Coverage 报告正确列出 zero_hit operations | L4-OBS-COVERAGE | `tests/integration/obs_coverage_test.go` | IMPLEMENTED |

### P1

| L5 ID | 描述 | L4 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-OBS-18 | Gateway 会话 Gauge 使用 SessionBridge | L4-OBS-METRICS | `tests/integration/obs_session_bridge_test.go` | IMPLEMENTED |

> 变更：`openspec/changes/devrix-observability-coverage/`（Demand: DM-20260607-007，Target: observability v1.3.0）

---

## Layer 6: Evolution (EVO)

### P1

| L5 ID | 描述 | L4 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-EVO-01 | 版本检测与记录 | L4-EVO-VERSION | — | PLANNED |
| L5-EVO-02 | 配置热更新 | L4-EVO-CONFIG | — | PLANNED |
