# D3 LLM Gateway Domain — T 层测试点注册表

**Status:** Active
**Version:** 2.1.0
**Last Updated:** 2026-06-14
**Parent:** `openspec/specs/architecture/layering.md`

---

## D3-S1: Adapter Module

| T ID | 描述 | 优先级 | S 映射 | Test 位置 | Status |
|-------|------|--------|--------|-----------|--------|
| D3-S1-A01-T01 | DeepSeek 适配器流式响应 | P0 | Adapter | `internal/layers/llmgateway/adapter/deepseek_test.go` | IMPLEMENTED |
| D3-S1-A01-T02 | MiniMax 适配器流式响应 | P0 | Adapter | `internal/layers/llmgateway/adapter/minimax_test.go` | IMPLEMENTED |
| D3-S1-A01-T03 | SSE parse error handling | P1 | Adapter | `internal/layers/llmgateway/adapter/sse_parser_test.go` | IMPLEMENTED |
| D3-S1-A01-T04 | OpenAI request body construction | P1 | Adapter | `internal/layers/llmgateway/adapter/openai_request_test.go` | IMPLEMENTED |

## D3-S2: Gateway Module

| T ID | 描述 | 优先级 | S 映射 | Test 位置 | Status |
|-------|------|--------|--------|-----------|--------|
| D3-S2-A01-T01 | LLM 调用可观测事件 (spans + metrics) | P1 | Gateway | `tests/integration/llm_observer_test.go` | IMPLEMENTED |
| D3-S2-A01-T02 | 未知 Provider/Model 报错 (router) | P1 | Gateway | `internal/layers/llmgateway/gateway/router_test.go` | IMPLEMENTED |
| D3-S2-A01-T03 | 多 Provider 并发调用 | P1 | Gateway | `internal/layers/llmgateway/gateway/gateway_test.go` | IMPLEMENTED |
| D3-S2-A01-T04 | Retry 与 CB 联动，context 取消不触发 CB | P0 | Gateway | `internal/layers/llmgateway/gateway/gateway_test.go` | IMPLEMENTED |
| D3-S2-A01-T05 | Half-Open 并发探测限制 | P0 | Gateway | `internal/layers/llmgateway/gateway/gateway_test.go` | IMPLEMENTED |
| D3-S2-A01-T06 | LLM 429 rate limit handling | P1 | Gateway | `tests/integration/llm_real_api_test.go` | IMPLEMENTED |

## D3-S3: Breaker Module

| T ID | 描述 | 优先级 | S 映射 | Test 位置 | Status |
|-------|------|--------|--------|-----------|--------|
| D3-S3-A01-T01 | Circuit breaker 正常关闭 (Closed) | P0 | Breaker | `internal/layers/llmgateway/breaker/circuit_breaker_test.go` | IMPLEMENTED |
| D3-S3-A01-T02 | Circuit breaker 触发开启 (Open) | P0 | Breaker | `internal/layers/llmgateway/breaker/circuit_breaker_test.go` | IMPLEMENTED |
| D3-S3-A01-T03 | Circuit breaker 半开→关闭 (HalfOpen→Closed) | P0 | Breaker | `internal/layers/llmgateway/breaker/circuit_breaker_test.go` | IMPLEMENTED |
| D3-S3-A01-T04 | Circuit breaker 半开→开启 (HalfOpen→Open) | P0 | Breaker | `internal/layers/llmgateway/breaker/circuit_breaker_test.go` | IMPLEMENTED |
| D3-S3-A01-T05 | 熔断器状态持久化 | P2 | Breaker | — | PLANNED |

## D3-S4: Retry Module

| T ID | 描述 | 优先级 | S 映射 | Test 位置 | Status |
|-------|------|--------|--------|-----------|--------|
| D3-S4-A01-T01 | 重试策略执行 (Full Jitter 退避) | P0 | Retry | `internal/layers/llmgateway/retry/retry_test.go` | IMPLEMENTED |
| D3-S4-A01-T02 | Full Jitter 随机化验证 | P1 | Retry | `internal/layers/llmgateway/retry/retry_jitter_test.go` | IMPLEMENTED |
| D3-S4-A01-T03 | DeepSeek Fallback 模型切换 | P1 | Retry | `tests/integration/llm_fallback_test.go` | IMPLEMENTED |
| D3-S4-A01-T04 | MiniMax Fallback 模型切换 | P1 | Retry | `tests/integration/llm_fallback_test.go` | IMPLEMENTED |

## D3-S5: Token Module

| T ID | 描述 | 优先级 | S 映射 | Test 位置 | Status |
|-------|------|--------|--------|-----------|--------|
| D3-S5-A01-T01 | Token 计数准确性 (cl100k_base) | P0 | Token | `internal/layers/llmgateway/token/counter_test.go` | IMPLEMENTED |
| D3-S5-A01-T02 | Token 预算检查 (CheckBudget) | P0 | Token | `internal/layers/llmgateway/token/counter_test.go` | IMPLEMENTED |
| D3-S5-A01-T03 | Token counter 中文准确性 (CJK) | P1 | Token | `internal/layers/llmgateway/token/counter_test.go` | IMPLEMENTED |

## D3-S6: Config Module

| T ID | 描述 | 优先级 | S 映射 | Test 位置 | Status |
|-------|------|--------|--------|-----------|--------|
| D3-S6-A01-T01 | Provider 配置加载与验证 | P0 | Config | `internal/layers/llmgateway/config/loader_test.go` | IMPLEMENTED |

## D3-S7: Safety Module

| T ID | 描述 | 优先级 | S 映射 | Test 位置 | Status |
|-------|------|--------|--------|-----------|--------|
| D3-S7-A01-T01 | Safety filter critical 拒绝 (malware/exploit) | P0 | Safety | `internal/layers/llmgateway/safety/filter_test.go` | IMPLEMENTED |
| D3-S7-A01-T02 | Safety filter warning 匹配 (injection/credential) | P1 | Safety | `internal/layers/llmgateway/safety/filter_test.go` | IMPLEMENTED |

## Bridge (D3→D2)

| T ID | 描述 | 优先级 | S 映射 | Test 位置 | Status |
|-------|------|--------|--------|-----------|--------|
| D3-S2-A01-T07 | Bridge 适配 Gateway → ILLMGateway | P1 | Bridge | `internal/bridges/llm/bridge_test.go` | IMPLEMENTED |

---

## Statistics

| Total | IMPLEMENTED | PLANNED |
|-------|-------------|---------|
| 26 | 25 | 1 |
