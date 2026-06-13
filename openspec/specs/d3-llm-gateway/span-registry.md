# D3 LLM Gateway Span 注册表

**Domain:** D3 LLM Gateway
**Version:** 2.0.0
**Status:** Active (2026-06-14)
**Canonical Source:** `internal/layers/observability/telemetry/names.go` · `internal/layers/observability/coverage/registry.go`

---

## Operations

### LLM Gateway（4 ops）+ Adapter（1 op）

| Operation | Kind | Component | Since | Key Attributes |
|-----------|------|-----------|-------|----------------|
| `llm.stream` | CLIENT | llm_gateway | 1.2.0 | model, provider, gen_ai.* |
| `llm.provider.route` | INTERNAL | llm_gateway | 2.0.0 | model, provider |
| `llm.circuit_breaker` | INTERNAL | llm_gateway | 2.0.0 | provider, state |
| `llm.retry` | INTERNAL | llm_gateway | 2.0.0 | attempt, max_attempts |
| `llm.adapter.stream` | CLIENT | llm_adapter | 2.0.0 | provider, model, url |

---

## Trace Tree

```
llm.stream                                   [CLIENT]
├── llm.provider.route                       [INTERNAL]
├── llm.circuit_breaker                      [INTERNAL]
├── llm.retry                                [INTERNAL]
│   └── llm.adapter.stream                   [CLIENT]
```

---

## Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `llm_requests_total` | Int64Counter | provider, model, status | 成功调用计数 |
| `llm_errors_total` | Int64Counter | provider, error_code | 失败调用计数 |
| `llm_latency_seconds` | Float64Histogram | provider, model | 调用延迟分布 |

---

## GenAI Token Recording

通过 `observability.RecordGenAITokenUsage` 记录以下属性：

| Attribute | Source |
|-----------|--------|
| `gen_ai.request.model` | Request.Model |
| `gen_ai.usage.input_tokens` | Usage.PromptTokens |
| `gen_ai.usage.output_tokens` | Usage.CompletionTokens |
| `gen_ai.usage.cache_read.input_tokens` | Usage.CacheReadTokens |
| `gen_ai.usage.reasoning.output_tokens` | Usage.ReasoningTokens |
| `gen_ai.conversation.id` | session_id |

---

## 延迟目标

| 指标 | 目标 | 关联 Span |
|------|------|-----------|
| LLM 调用延迟 | P99 < 5s | `llm.stream` |
| 熔断器切换延迟 | < 10ms | `llm.circuit_breaker` |

---

## 关联文档

- D5 全局 Trace Tree：`openspec/specs/d5-observability/span-registry.md`
- 全局 Spans 索引：`openspec/spans-registry.md`
