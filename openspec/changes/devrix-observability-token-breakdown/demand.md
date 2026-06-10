---
demand-id: DM-20260610-007
title: 可观察层 P3 — cache_read / reasoning Token 细分
source: observability deferred token breakdown
priority: P2
status: ACCEPTED
l1-domain: observability
created: 2026-06-10
parent-demand: DM-20260610-005
---

# Token 类型细分（cache_read / reasoning）

Provider 已支持 `prompt_tokens_details.cached_tokens` 与 `completion_tokens_details.reasoning_tokens`。本 change 解析并写入 metrics + span attributes。

## 验收

- `devrix_gen_ai.client.token.usage` 支持 `token_type=cache_read|reasoning`
- LLM span 含 `gen_ai.usage.cache_read.input_tokens` / `gen_ai.usage.reasoning.output_tokens`（非零时）
- 无 provider 字段时行为与现网一致（仅 input/output）
