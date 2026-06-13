# D3 LLM Gateway Domain — F 层功能点注册表

**Capability:** architecture-layering
**Status:** Active
**Version:** 1.0.0
**Last Updated:** 2026-06-13
**Parent:** `openspec/specs/architecture/layering.md`
**Depends On:** `openspec/specs/d3-llm-gateway/a-registry.md`

---

## Overview

D3 LLM 网关域 F 层功能点注册表。

---

## D3-S1-A01 CallModel

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D3-S1-A01-F01 | StreamChat | F-BE | openai_request | <-chan chunk | `adapter/deepseek.go` / `adapter/minimax.go` |
| D3-S1-A01-F02 | ParseSSE | F-BE | raw_bytes | parsed_chunk | `adapter/sse_parser.go` |

## D3-S2-A01 RouteLLMCall

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D3-S2-A01-F01 | ResolveModel | F-BE | model_name | provider, resolved_model | `gateway/router.go` |
| D3-S2-A01-F02 | StreamWithBreaker | F-BE | ctx, request | <-chan chunk | `gateway/gateway.go` |

## D3-S3-A01 ManageCircuitBreaker

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D3-S3-A01-F01 | BeforeCall | F-BE | provider | allowed/blocked | `breaker/circuit_breaker.go` |
| D3-S3-A01-F02 | AfterCall | F-BE | provider, success | — | `breaker/circuit_breaker.go` |
| D3-S3-A01-F03 | TransitionState | F-BE | provider, state | — | `breaker/state.go` |

## D3-S5-A01 CountLLMTokens

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D3-S5-A01-F01 | EncodeText | F-BE | text, encoding | []int | `token/` |
| D3-S5-A01-F02 | DecodeTokens | F-BE | []int, encoding | string | `token/` |

## LLM Bridge (D3 → D2)

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D3-S1-A01-F03 | AdaptToContextEngine | F-BE | llmgateway_request | contextengine_response | `bridges/llm/bridge.go` |

---

## Statistics

| Activities with F | Total F Points |
|-------------------|----------------|
| 4 | 10 |
