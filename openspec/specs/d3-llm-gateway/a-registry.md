# D3 LLM Gateway Domain — A 层活动注册表

**Capability:** architecture-layering
**Status:** Active
**Version:** 2.1.0
**Last Updated:** 2026-06-14
**Parent:** `openspec/specs/architecture/layering.md`

---

## Overview

D3 LLM 网关域 A 层活动注册表。每个 S（场景）下包含若干 A（活动），通过 A 暴露 D3 的能力给其他域。

---

## D3-S1: Adapter（适配器）

多 Provider 统一流式适配。

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D3-S1-A01 | CallModel | A-BE | llmgateway.Request | <-chan AdapterChunk | — | `llmgateway/adapter/deepseek.go`, `minimax.go`, `openai_stream.go`, `openai_request.go`, `sse_parser.go` |

---

## D3-S2: Gateway（网关）

模型路由、流式编排与可观测性。

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D3-S2-A01 | RouteLLMCall | A-BE | model, messages | streaming_response, spans, metrics | — | `llmgateway/gateway/gateway.go`, `router.go` |

---

## D3-S3: Breaker（熔断器）

Provider 级熔断保护。

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D3-S3-A01 | ManageCircuitBreaker | A-BE | provider, call_result | — | circuit.{closed,open,half_open} | `llmgateway/breaker/circuit_breaker.go`, `state.go` |

---

## D3-S4: Retry（重试）

Full Jitter 退避重试与 fallback 模型切换。

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D3-S4-A01 | ExecuteRetry | A-BE | ctx, stream_call, primary_model, fallback_model, retry_config | <-chan AdapterChunk | — | `llmgateway/retry/retry.go` |

---

## D3-S5: Token（令牌计数）

cl100k_base 令牌计数与预算检查。

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D3-S5-A01 | CountLLMTokens | A-BE | text, messages, system_prompt | token_count | — | `llmgateway/token/counter.go`, `bpe_loader.go` |

---

## D3-S6: Config（配置）

LLM 网关配置加载与验证。

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D3-S6-A01 | LoadLLMConfig | A-BE | config_file | validated_llm_config | — | `llmgateway/config/loader.go`, `shared/config/llmgateway.go` |

---

## D3-S7: Safety（内容安全）

LLM 请求内容安全过滤。

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D3-S7-A01 | FilterContent | A-BE | system_prompt, messages | safety_result | — | `llmgateway/safety/filter.go`, `patterns.go` |

---

## Statistics

| Scenarios | Activities |
|-----------|------------|
| 7 | 7 |
