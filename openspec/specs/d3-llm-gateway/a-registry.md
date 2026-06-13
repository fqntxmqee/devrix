# D3 LLM Gateway Domain — A 层活动注册表

**Capability:** architecture-layering
**Status:** Active
**Version:** 1.0.0
**Last Updated:** 2026-06-13
**Parent:** `openspec/specs/architecture/layering.md`

---

## Overview

D3 LLM 网关域 A 层活动注册表。

---

## D3-S1: Adapter

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D3-S1-A01 | CallModel | A-BE | llm_request | <-chan llm_chunk | — | `llmgateway/adapter/` |

## D3-S2: Gateway

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D3-S2-A01 | RouteLLMCall | A-BE | model, messages | streaming_response | — | `llmgateway/gateway/gateway.go` |

## D3-S3: Breaker

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D3-S3-A01 | ManageCircuitBreaker | A-BE | provider, call_result | circuit_state | circuit.{closed,open,half_open} | `llmgateway/breaker/circuit_breaker.go` |

## D3-S4: Retry

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D3-S4-A01 | ExecuteRetry | A-BE | attempt, error | retry_decision | — | `llmgateway/retry/` |

## D3-S5: Token

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D3-S5-A01 | CountLLMTokens | A-BE | text, encoding | token_count | — | `llmgateway/token/` |

## D3-S6: Config

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| D3-S6-A01 | LoadLLMConfig | A-BE | config_file | llm_config | — | `llmgateway/config/` |

---

## Statistics

| Scenarios | Activities |
|-----------|------------|
| 6 | 6 |
