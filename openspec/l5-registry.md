# Devrix L5 测试点注册表

**Status:** Active
**Last Updated:** 2026-06-08
**Layering Spec:** `openspec/specs/architecture/layering.md`

> L5 测试点是 OpenSpec S5 验收的确定性锚点。新增 L4/L3 能力时 MUST 先在此记录或复用现有 L5。
>
> **编号格式**: `L5-{D}-{S}-{NN}`
> - D = 域编号 Domain (1-6)
> - S = 场景编号 Scenario (1-8)
> - NN = 序号 (01-99)

---

## 注册规则

| 字段 | 说明 |
|------|------|
| ID | `L5-{D}-{S}-{NN}` |
| Priority | P0（阻断交付）/ P1（需执行，失败记例外）/ P2（尽力） |
| S 映射 | 关联的 Scenario 模块 ID |
| Test 位置 | 测试文件路径 |
| Status | PLANNED / IMPLEMENTED / DEPRECATED |

---

## D1: Communication Domain (COMM)

> **Spec Reference:** `openspec/archive/2026-06-08-devrix-d1-d6-testing/demand.md`

### D1-S1: Gateway Module

| L5 ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| L5-1-1-01 | 新会话创建被拒绝 | Gateway | `tests/acceptance/p0/comm_gateway_flow_test.go` | IMPLEMENTED | P0 |
| L5-1-1-02 | IM 入口实例 Register/Unregister | Gateway | `internal/layers/communication/instance/registry_test.go` | IMPLEMENTED | P2 |

### D1-S5: Milestone Module

> **Change:** `devrix-v3-integration` (DM-20260608-010)

| L5 ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| L5-1-5-01 | Milestone 环检测拒绝循环依赖 | Milestone | `internal/layers/communication/milestone/service_test.go` | IMPLEMENTED | P1 |
| L5-1-5-02 | TaskFlow 多里程碑链顺序执行至完成 | Milestone | `internal/layers/communication/milestone/taskflow_test.go` | IMPLEMENTED | P1 |
| L5-1-5-03 | 无 V1 TaskFlow stub 误导日志 | Milestone | — (file removed) | IMPLEMENTED | P2 |

### D1-S3: Commands Module

| L5 ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| L5-1-3-01 | /new 命令解析正确 | Commands | `tests/acceptance/p0/comm_commands_test.go` | IMPLEMENTED | P0 |
| L5-1-3-02 | /help 命令解析正确 | Commands | `tests/acceptance/p0/comm_commands_test.go` | IMPLEMENTED | P1 |
| L5-1-3-03 | /stop 命令解析正确 | Commands | `tests/acceptance/p0/comm_commands_test.go` | IMPLEMENTED | P1 |

### D1-S8: Renderers Module

| L5 ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| L5-1-8-01 | ShortId 唯一且排除异议字符 | Renderers | `internal/shared/types/shortid_test.go` | IMPLEMENTED | P1 |
| L5-1-8-02 | ProgressBar / StatusBadge 渲染输出合法 | Renderers | `internal/layers/communication/renderers/components_test.go` | IMPLEMENTED | P2 |

### D1-S2: Adapters Module

| L5 ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| L5-1-2-01 | 飞书消息解析正确 | Adapters | `internal/layers/communication/adapters/feishu_test.go` | IMPLEMENTED | P1 |
| L5-1-2-02 | 钉钉 Webhook 入站路由 + Session 出站 | Adapters | `internal/layers/communication/adapters/dingtalk_test.go` | IMPLEMENTED | P1 |
| L5-1-2-03 | 钉钉 milestone 出站走 CardRenderer | Adapters | `internal/layers/communication/adapters/dingtalk_test.go` | IMPLEMENTED | P1 |

---

## D2: Context Engine Domain (CTX)

> V1 已归档：`openspec/archive/2026-06-07-devrix-context-engine/`
> V2 已归档：`openspec/archive/2026-06-07-devrix-context-engine-v2/`
> V3 已归档：`openspec/archive/2026-06-07-devrix-context-engine-v3/`
> V4 已归档：`openspec/archive/2026-06-08-devrix-context-engine-v4/`

### D2-S3: Memory Module

| L5 ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-2-3-01 | 新会话历史正确追加 | Memory | `internal/layers/contextengine/memory/manager_test.go` | IMPLEMENTED |
| L5-2-3-02 | ContextSnapshot 备份 | Memory | `internal/layers/contextengine/snapshot/store_test.go` | IMPLEMENTED |
| L5-2-3-03 | LongTerm Recall 注入上下文 | Memory | `tests/acceptance/p0/ctx_plan_longterm_test.go` | IMPLEMENTED |
| L5-2-3-04 | LongTerm Store 持久化写入 | Memory | `internal/layers/contextengine/memory/longterm_test.go` | IMPLEMENTED |
| L5-2-3-05 | L3 长期记忆返回 NotImplemented | Memory | `internal/layers/contextengine/memory/longterm_test.go` | IMPLEMENTED |
| L5-2-3-06 | 快照使用 snappy 压缩体积显著缩减 | Memory | `internal/layers/contextengine/snapshot/store_test.go` | IMPLEMENTED |

### D2-S2: Compression Module

| L5 ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-2-2-01 | 超 Token 阈值触发七步压缩 | Compression | `tests/acceptance/p0/ctx_compression_test.go` | IMPLEMENTED |
| L5-2-2-02 | TokenBlock 超限返回 ContextExceeded | Compression | `internal/layers/contextengine/compression/pipeline_test.go` | IMPLEMENTED |
| L5-2-2-03 | Autocompact 触发并降低 token | Compression | `internal/layers/contextengine/compression/autocompact_test.go` | IMPLEMENTED |
| L5-2-2-04 | Autocompact LLM 失败降级跳过 | Compression | `internal/layers/contextengine/compression/autocompact_test.go` | IMPLEMENTED |
| L5-2-2-05 | Autocompact 禁用时跳过步骤 6 | Compression | `internal/layers/contextengine/compression/pipeline_test.go` | IMPLEMENTED |
| L5-2-2-06 | 异步压缩占位不阻塞主路径 | Compression | `internal/layers/contextengine/compression/async_compact_test.go` | IMPLEMENTED |
| L5-2-2-07 | 异步压缩失败降级不丢失数据 | Compression | `internal/layers/contextengine/compression/async_compact_test.go` | IMPLEMENTED |
| L5-2-2-08 | Autocompact timeout fallback | Compression | `internal/layers/contextengine/compression/autocompact_test.go` | IMPLEMENTED |

### D2-S1: PEV Module

| L5 ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-2-1-01 | PEV Execute 调用 LLM 并流输出 | PEV | `tests/integration/context_gateway_flow_test.go` | IMPLEMENTED |
| L5-2-1-02 | 工具执行 Verify basic 模式 | PEV | `internal/layers/contextengine/verify_runner_test.go` | IMPLEMENTED |
| L5-2-1-03 | EngineEvent 与通信层四握约定一致 | PEV | `tests/integration/context_gateway_flow_test.go` | IMPLEMENTED |
| L5-2-1-04 | 批准/拒绝 PEV 行为正确 | PEV | `tests/integration/context_gateway_flow_test.go` | IMPLEMENTED |
| L5-2-1-05 | Verify commands 全部通过 | PEV | `tests/integration/context_verify_commands_test.go` | IMPLEMENTED |
| L5-2-1-06 | Verify 命令失败触发重试 | PEV | `internal/layers/contextengine/verify_runner_test.go` | IMPLEMENTED |
| L5-2-1-07 | Milestone 按序执行 | PEV | `tests/integration/context_plan_milestone_test.go` | IMPLEMENTED |
| L5-2-1-08 | milestone_progress 事件正确投射 | PEV | `tests/acceptance/p0/ctx_plan_longterm_test.go` | IMPLEMENTED |
| L5-2-1-09 | Verify timeout kills command (DeadlineExceeded) | PEV | `tests/integration/context_verify_commands_test.go` | IMPLEMENTED |
| L5-2-1-10 | Shell injection attack prevention | PEV | `tests/security/shell_injection_test.go` | IMPLEMENTED |
| L5-2-1-11 | PEV concurrent session isolation | PEV | `internal/layers/contextengine/pev_engine_test.go` | IMPLEMENTED |
| L5-2-1-12 | PEV context cancellation cleanup | PEV | `internal/layers/contextengine/pev_engine_test.go` | IMPLEMENTED |

### D2-S4: Token Module

| L5 ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-2-4-01 | Token 计数共享约定与 Gateway 对齐 | Token | `tests/acceptance/p0/ctx_plan_longterm_test.go` | IMPLEMENTED |

### D2-S8: Sandbox Module

| L5 ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-2-8-01 | bash 困居工作目录 + 命令白名单 | Sandbox | `internal/layers/contextengine/sandbox_test.go` | IMPLEMENTED |

### D2: Cross-Scenario Tests

| L5 ID | 描述 | Test 位置 | Status |
|-------|------|-----------|--------|
| L5-2-0-01 | 压缩/Verify 步骤可观事务 | `tests/integration/context_compression_obs_test.go` | IMPLEMENTED |
| L5-2-0-02 | 主路径接入真实 LLM Gateway | `tests/integration/context_llm_gateway_test.go` | IMPLEMENTED |
| L5-2-0-03 | plan.enabled=false 时回退 V2 路径 | `tests/integration/context_plan_milestone_test.go` | IMPLEMENTED |

---

## D3: LLM Gateway Domain (LLM)

### D3-S1: Adapter Module

| L5 ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-3-1-01 | DeepSeek 适配器流响应 | Adapter | `internal/layers/llmgateway/adapter/deepseek_test.go` | IMPLEMENTED |
| L5-3-1-02 | MiniMax 适配器流响应 | Adapter | `internal/layers/llmgateway/adapter/minimax_test.go` | IMPLEMENTED |
| L5-3-1-03 | SSE parse error handling | Adapter | `tests/integration/llm_real_api_test.go` | IMPLEMENTED |

### D3-S3: Breaker Module

| L5 ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-3-3-01 | Circuit breaker 正常关闭 | Breaker | `internal/layers/llmgateway/breaker/circuit_breaker_test.go` | IMPLEMENTED |
| L5-3-3-02 | Circuit breaker 触发放开 | Breaker | `internal/layers/llmgateway/breaker/circuit_breaker_test.go` | IMPLEMENTED |
| L5-3-3-03 | Circuit breaker 半开→关闭 | Breaker | `internal/layers/llmgateway/breaker/circuit_breaker_test.go` | IMPLEMENTED |
| L5-3-3-04 | Circuit breaker 半开→放开 | Breaker | `internal/layers/llmgateway/breaker/circuit_breaker_test.go` | IMPLEMENTED |
| L5-3-3-05 | 熔断器状态长久化 | Breaker | - | PLANNED |

### D3-S5: Token Module

| L5 ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-3-5-01 | Token 计数准确性 (cl100k_base) | Token | `internal/layers/llmgateway/token/counter_test.go` | IMPLEMENTED |
| L5-3-5-02 | Token 预算检查 | Token | `internal/layers/llmgateway/token/counter_test.go` | IMPLEMENTED |
| L5-3-5-03 | Token counter 中文准确性 | Token | `internal/layers/llmgateway/token/counter_test.go` | IMPLEMENTED |

### D3-S6: Config Module

| L5 ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-3-6-01 | Provider 配置加载 | Config | `internal/layers/llmgateway/config/loader_test.go` | IMPLEMENTED |

### D3-S4: Retry Module

| L5 ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-3-4-01 | 重试策略执行 | Retry | `internal/layers/llmgateway/retry/retry_test.go` | IMPLEMENTED |
| L5-3-4-02 | DeepSeek Fallback 模型切换 | Retry | `tests/integration/llm_fallback_test.go` | IMPLEMENTED |
| L5-3-4-03 | MiniMax Fallback 模型切换 | Retry | `tests/integration/llm_fallback_test.go` | IMPLEMENTED |

### D3-S2: Gateway Module

| L5 ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-3-2-01 | LLM 调用可观测事件 | Gateway | `tests/integration/llm_observer_test.go` | IMPLEMENTED |
| L5-3-2-02 | 未知 Provider/Model 报错 | Gateway | `internal/layers/llmgateway/gateway/router_test.go` | IMPLEMENTED |
| L5-3-2-03 | 多 Provider 并发调用 | Gateway | `internal/layers/llmgateway/gateway/gateway_test.go` | IMPLEMENTED |
| L5-3-2-04 | Retry 与 CB 联动，context 取消不触发 CB | Gateway | `internal/layers/llmgateway/gateway/gateway_test.go` | IMPLEMENTED |
| L5-3-2-05 | Half-Open 并发探测上游 | Gateway | `internal/layers/llmgateway/gateway/gateway_test.go` | IMPLEMENTED |
| L5-3-2-06 | LLM 429 rate limit handling | Gateway | `tests/integration/llm_real_api_test.go` | IMPLEMENTED |

---

## D4: Multi-Agent Domain (AGENT)

> 已归档：`openspec/archive/2026-06-08-devrix-multi-agent/`（Demand: DM-20260608-005）

### D4-S1: Factory Module

| L5 ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-4-1-01 | AgentFactory 创建 Agent 实例 | Factory | `internal/layers/multiagent/factory/factory_test.go` | IMPLEMENTED |

### D4-S2: Agent Module

| L5 ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-4-2-01 | Agent 生命周期状态转换 | Agent | `internal/layers/multiagent/agent/agent_test.go` | IMPLEMENTED |
| L5-4-2-02 | AgentPermissionGate 批准/拒绝/超时 | Agent | `internal/layers/multiagent/agent/perm_gate_test.go` | IMPLEMENTED |
| L5-4-2-03 | CRITICAL 工具权限异步流程 | Agent | `internal/layers/multiagent/agent/perm_gate_test.go` | IMPLEMENTED |

### D4-S3: ForkJoin Module

| L5 ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-4-3-01 | Fork/Join 消息隔离模型 | ForkJoin | `internal/layers/multiagent/agent/agent_test.go` | IMPLEMENTED |
| L5-4-3-02 | Fork 双层限额 MaxChildren+MaxTotalAgents | ForkJoin | `internal/layers/multiagent/factory/factory_test.go` | IMPLEMENTED |
| L5-4-3-03 | Agent 超时自动终止 | Agent | `internal/layers/multiagent/agent/agent_test.go` | IMPLEMENTED |
| L5-4-3-04 | Context 取消传播到子 Agent | Agent | `internal/layers/multiagent/agent/agent_test.go` | IMPLEMENTED |

### D4-S4: Collaboration Module

| L5 ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-4-4-01 | CoT prompt 增强 | Collaboration | `internal/layers/multiagent/collaboration/mode_test.go` | IMPLEMENTED |
| L5-4-4-02 | Iterative-Refinement prompt 增强 | Collaboration | `internal/layers/multiagent/collaboration/mode_test.go` | IMPLEMENTED |

### D4-S5: Observer Module

| L5 ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-4-5-01 | ObserverAdapter 桥接 AgentEvent → IObserver | Observer | `internal/layers/multiagent/observer/adapter.go` | IMPLEMENTED |

### D4: Cross-Scenario Tests

| L5 ID | 描述 | Test 位置 | Status |
|-------|------|-----------|--------|
| L5-4-0-01 | Agent 并发安全 (-race) | `internal/layers/multiagent/agent/agent_test.go` | IMPLEMENTED |
| L5-4-0-02 | Fork 消息隔离并发安全 | `internal/layers/multiagent/agent/agent_test.go` | IMPLEMENTED |
| L5-4-0-03 | Gateway → ResolvePermission 集成全流程 | `tests/integration/agent_integration_test.go` | IMPLEMENTED |
| L5-4-0-04 | E2E Fork 端到端 | `tests/e2e/agent_fork_e2e_test.go` | IMPLEMENTED |

---

## D5: Observability Domain (OBS)

### D5-S2: Metrics Module

| L5 ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-5-2-01 | Tracing Span 创建与传播 | Metrics | - | PLANNED |
| L5-5-2-02 | Metrics Counter 计数 | Metrics | - | PLANNED |
| L5-5-2-03 | Gauge Set/Inc/Dec/Add/Sub 数值正确 | Metrics | `internal/layers/observability/metrics/gauge_test.go` | IMPLEMENTED |
| L5-5-2-04 | Histogram Prometheus 输出与 golden 一致 | Metrics | `internal/layers/observability/metrics/histogram_test.go` | IMPLEMENTED |
| L5-5-2-05 | Int64UpDownCounter 返回 Gauge | Metrics | `internal/layers/observability/metrics/meter_test.go` | IMPLEMENTED |
| L5-5-2-06 | Compression P99 latency < 500ms | Metrics | `tests/performance/compression_test.go` | IMPLEMENTED |
| L5-5-2-07 | Concurrent session memory bounded | Metrics | `tests/performance/memory_test.go` | IMPLEMENTED |

### D5-S3: Logger Module

| L5 ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-5-3-01 | 日志级别过滤 | Logger | - | PLANNED |
| L5-5-3-02 | Shutdown 覆盖 Tracer + Logger | Logger | `internal/layers/observability/logger/logger_test.go` | IMPLEMENTED |
| L5-5-3-03 | Error 日志包含 stacktrace 字段 | Logger | `internal/layers/observability/logger/logger_test.go` | IMPLEMENTED |
| L5-5-3-04 | 日志采样 max_entries_per_span 生效 | Logger | `internal/layers/observability/logger/sampling_test.go` | IMPLEMENTED |

### D5-S1: Tracer Module

| L5 ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-5-1-01 | Shutdown 刷写所有 pending spans | Tracer | `internal/layers/observability/tracer/tracer_test.go` | IMPLEMENTED |
| L5-5-1-02 | ConsoleExporter 可直接作为 SpanExporter | Tracer | `internal/layers/observability/exporter/console_test.go` | IMPLEMENTED |

### D5-S4: Exporter Module

| L5 ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-5-4-01 | LongTerm recall/store 产生 canonical Operation span | Exporter | `internal/layers/contextengine/engine.go` | IMPLEMENTED |
| L5-5-4-02 | Plan 产生 Milestone Run 产生 canonical Operation span | Exporter | `internal/layers/contextengine/pev_engine.go` | IMPLEMENTED |
| L5-5-4-03 | Feishu 入站产生 adapter.message.receive span | Exporter | `tests/integration/obs_trace_propagation_test.go` | IMPLEMENTED |

### D5-S5: Coverage Module

| L5 ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-5-5-01 | Operation Registry 与 names.go 常驻全集一致 | Coverage | `internal/layers/observability/coverage/registry_test.go` | IMPLEMENTED |
| L5-5-5-02 | Coverage 报告正确列出 zero_hit operations | Coverage | `tests/integration/obs_coverage_test.go` | IMPLEMENTED |

### D5: Cross-Scenario Tests

| L5 ID | 描述 | Test 位置 | Status |
|-------|------|-----------|--------|
| L5-5-0-01 | Gateway 会话 | - | PLANNED |

---

## D6: Evolution Domain (EVO)

> **Spec Reference:** `openspec/archive/2026-06-08-devrix-d1-d6-testing/demand.md`

### D6-S1: Version Module

| L5 ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| L5-6-1-01 | 版本检测与记录（PlannedVersion: v2.1.0） | Version | `internal/layers/evolution/version/version_test.go` | PLANNED | P2 |

### D6-S2: Config Module

| L5 ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| L5-6-2-01 | 配置热更新（PlannedVersion: v2.2.0） | Config | `internal/layers/evolution/config/hotreload_test.go` | PLANNED | P2 |

---

## D0: Code Integrity Domain (INTEGRITY)

> 跨域代码健康规范。属架构治理层，非业务域。
> **Spec Reference:** `openspec/changes/devrix-code-integrity/`

### D0-S1: Specification Module

| L5 ID | 描述 | Test 位置 | Status | Priority |
|-------|------|-----------|--------|----------|
| L5-0-1-01 | `coding.md §9` 包含不可变性分层规范 | `openspec/specs/project/coding.md` §9 | IMPLEMENTED | P0 |
| L5-0-1-02 | CLAUDE.md 引用新规范 | `CLAUDE.md` | IMPLEMENTED | P0 |
| L5-0-1-03 | emitEvent 处理 EventConnectionLostData 不 panic | `internal/layers/communication/connection/manager_test.go` | IMPLEMENTED | P1 |
| L5-0-1-04 | emitEvent 处理 EventConnectionRestoredData 不 panic | `internal/layers/communication/connection/manager_test.go` | IMPLEMENTED | P1 |
| L5-0-1-05 | emitEvent 处理未知类型不 panic | `internal/layers/communication/connection/manager_test.go` | IMPLEMENTED | P1 |
| L5-0-1-06 | 新会话创建被拒绝 | `tests/acceptance/p0/comm_gateway_flow_test.go` | IMPLEMENTED | P0 |
| L5-0-1-07 | /new /help /stop 命令解析正确 | `tests/acceptance/p0/comm_commands_test.go` | IMPLEMENTED | P0 |
| L5-0-1-08 | 飞书消息解析正确 | `internal/layers/communication/adapters/feishu_test.go` | IMPLEMENTED | P1 |
| L5-0-1-09 | D6 L5 注册表条目包含目标版本 | `openspec/l5-registry.md` | IMPLEMENTED | P1 |

---

## 状态汇总

| Domain | Domain Name | Total | IMPLEMENTED | PLANNED | P0 |
|--------|------------|-------|-------------|---------|-----|
| D0 | Code Integrity | 9 | 9 | 0 | 3 |
| D1 | Communication | 13 | 13 | 0 | 2 |
| D2 | Context Engine | 31 | 31 | 0 | 0 |
| D3 | LLM Gateway | 21 | 20 | 1 | 0 |
| D4 | Multi-Agent | 15 | 15 | 0 | 0 |
| D5 | Observability | 19 | 15 | 4 | 0 |
| D6 | Evolution | 2 | 0 | 2 | 0 |
| **Total** | | **110** | **103** | **7** | **5** |

---

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-08 | Complete rewrite with D-S numbering system |
| 1.1.0 | 2026-06-08 | Section headers migrated L1-L2 → D-S (DM-20260608-007) |
| 1.2.0 | 2026-06-08 | D1/D6 testing specs added; Priority column added (devrix-d1-d6-testing) |
| 1.3.0 | 2026-06-08 | D1 L5 IMPLEMENTED; registry summary reconciled (DM-20260608-011) |
