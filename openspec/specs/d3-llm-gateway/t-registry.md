# D3 LLM Gateway Domain — T 层测试点注册表

**Status:** Active
**Version:** 1.0.0
**Last Updated:** 2026-06-13
**Parent:** `openspec/specs/architecture/layering.md`

---

## D3-S1: Adapter Module

| T ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| D3-S1-A01-T01 | DeepSeek 适配器流响应 | Adapter | `internal/layers/llmgateway/adapter/deepseek_test.go` | IMPLEMENTED |
| D3-S1-A01-T02 | MiniMax 适配器流响应 | Adapter | `internal/layers/llmgateway/adapter/minimax_test.go` | IMPLEMENTED |
| D3-S1-A01-T03 | SSE parse error handling | Adapter | `tests/integration/llm_real_api_test.go` | IMPLEMENTED |

## D3-S3: Breaker Module

| T ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| D3-S3-A01-T01 | Circuit breaker 正常关闭 | Breaker | `internal/layers/llmgateway/breaker/circuit_breaker_test.go` | IMPLEMENTED |
| D3-S3-A01-T02 | Circuit breaker 触发放开 | Breaker | `internal/layers/llmgateway/breaker/circuit_breaker_test.go` | IMPLEMENTED |
| D3-S3-A01-T03 | Circuit breaker 半开→关闭 | Breaker | `internal/layers/llmgateway/breaker/circuit_breaker_test.go` | IMPLEMENTED |
| D3-S3-A01-T04 | Circuit breaker 半开→放开 | Breaker | `internal/layers/llmgateway/breaker/circuit_breaker_test.go` | IMPLEMENTED |
| D3-S3-A01-T05 | 熔断器状态长久化 | Breaker | - | PLANNED |

## D3-S5: Token Module

| T ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| D3-S5-A01-T01 | Token 计数准确性 (cl100k_base) | Token | `internal/layers/llmgateway/token/counter_test.go` | IMPLEMENTED |
| D3-S5-A01-T02 | Token 预算检查 | Token | `internal/layers/llmgateway/token/counter_test.go` | IMPLEMENTED |
| D3-S5-A01-T03 | Token counter 中文准确性 | Token | `internal/layers/llmgateway/token/counter_test.go` | IMPLEMENTED |

## D3-S6: Config Module

| T ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| D3-S6-A01-T01 | Provider 配置加载 | Config | `internal/layers/llmgateway/config/loader_test.go` | IMPLEMENTED |

## D3-S4: Retry Module

| T ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| D3-S4-A01-T01 | 重试策略执行 | Retry | `internal/layers/llmgateway/retry/retry_test.go` | IMPLEMENTED |
| D3-S4-A01-T02 | DeepSeek Fallback 模型切换 | Retry | `tests/integration/llm_fallback_test.go` | IMPLEMENTED |
| D3-S4-A01-T03 | MiniMax Fallback 模型切换 | Retry | `tests/integration/llm_fallback_test.go` | IMPLEMENTED |

## D3-S2: Gateway Module

| T ID | 描述 | S 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| D3-S2-A01-T01 | LLM 调用可观测事件 | Gateway | `tests/integration/llm_observer_test.go` | IMPLEMENTED |
| D3-S2-A01-T02 | 未知 Provider/Model 报错 | Gateway | `internal/layers/llmgateway/gateway/router_test.go` | IMPLEMENTED |
| D3-S2-A01-T03 | 多 Provider 并发调用 | Gateway | `internal/layers/llmgateway/gateway/gateway_test.go` | IMPLEMENTED |
| D3-S2-A01-T04 | Retry 与 CB 联动，context 取消不触发 CB | Gateway | `internal/layers/llmgateway/gateway/gateway_test.go` | IMPLEMENTED |
| D3-S2-A01-T05 | Half-Open 并发探测上游 | Gateway | `internal/layers/llmgateway/gateway/gateway_test.go` | IMPLEMENTED |
| D3-S2-A01-T06 | LLM 429 rate limit handling | Gateway | `tests/integration/llm_real_api_test.go` | IMPLEMENTED |

---

## Statistics

| Total | IMPLEMENTED | PLANNED |
|-------|-------------|---------|
| 21 | 20 | 1 |
