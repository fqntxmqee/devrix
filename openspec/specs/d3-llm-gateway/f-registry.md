# D3 LLM Gateway Domain — F 层功能点注册表

**Capability:** architecture-layering
**Status:** Active
**Version:** 2.1.0
**Last Updated:** 2026-06-14
**Parent:** `openspec/specs/architecture/layering.md`
**Depends On:** `openspec/specs/d3-llm-gateway/a-registry.md`

---

## Overview

D3 LLM 网关域 F 层功能点注册表。每个 A 下包含若干 F（功能点），是最小可测逻辑单元。

---

## D3-S1-A01 CallModel

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D3-S1-A01-F01 | StreamChat | F-BE | llmgateway.Request | <-chan AdapterChunk | `adapter/openai_stream.go` (OpenAIStreamClient.Stream) |
| D3-S1-A01-F02 | ParseSSE | F-BE | io.Reader | <-chan Chunk (via emit) | `adapter/sse_parser.go` (streamOpenAISSE, streamAccumulator) |
| D3-S1-A01-F03 | BuildOpenAIRequest | F-BE | llmgateway.Request | openAI JSON body | `adapter/openai_request.go` (buildOpenAIChatRequest, mapOpenAIMessage) |

## D3-S2-A01 RouteLLMCall

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D3-S2-A01-F01 | ResolveModel | F-BE | model_name | provider, resolved_model | `gateway/router.go` (Router.Resolve, matchRouting) |
| D3-S2-A01-F02 | StreamWithBreaker | F-BE | ctx, llmgateway.Request | <-chan Chunk | `gateway/gateway.go` (Gateway.Stream) |
| D3-S2-A01-F03 | ResolveTier | F-BE | tier_alias | concrete_model | `gateway/router.go` (Router.ResolveTier) |

## D3-S3-A01 ManageCircuitBreaker

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D3-S3-A01-F01 | BeforeCall | F-BE | provider | allowed/blocked | `breaker/circuit_breaker.go` (Allow) |
| D3-S3-A01-F02 | AfterCall | F-BE | provider, success | — | `breaker/circuit_breaker.go` (RecordSuccess, RecordFailure) |
| D3-S3-A01-F03 | ManageState | F-BE | provider, events | — | `breaker/circuit_breaker.go` (open, finalize) + `state.go` (circuitRecord) |

## D3-S4-A01 ExecuteRetry

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D3-S4-A01-F01 | StreamWithFallback | F-BE | ctx, call, primary, fallback, cfg | <-chan AdapterChunk | `retry/retry.go` (Executor.Stream) |
| D3-S4-A01-F02 | ComputeBackoff | F-BE | cfg, attempt | delay (Full Jitter) | `retry/retry.go` (backoffDelay) |

## D3-S5-A01 CountLLMTokens

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D3-S5-A01-F01 | CountText | F-BE | text | token_count | `token/counter.go` (Counter.CountText) |
| D3-S5-A01-F02 | CountMessages | F-BE | []Message, systemPrompt | total_tokens | `token/counter.go` (Counter.CountMessages, CountWithSystemPrompt) |
| D3-S5-A01-F03 | CheckBudget | F-BE | count, budget | error/nil | `token/counter.go` (Counter.CheckBudget) |
| D3-S5-A01-F04 | TruncateToTokens | F-BE | text, maxTokens | truncated_text | `token/counter.go` (Counter.TruncateToTokens) |
| D3-S5-A01-F05 | LoadBPE | F-BE | — | tiktoken.Encoding | `token/bpe_loader.go` (ensureEmbeddedBPELoader, embeddedBpeLoader) |

## D3-S6-A01 LoadLLMConfig

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D3-S6-A01-F01 | LoadConfig | F-BE | config_file | LLMGatewayConfig | `config/loader.go` (Loader.Load, LoadFromFileConfig) |
| D3-S6-A01-F02 | BuildConfig | F-BE | LLMGatewayFileConfig | LLMGatewayConfig | `shared/config/llmgateway.go` (BuildLLMGatewayConfig, DefaultLLMGatewayConfig) |
| D3-S6-A01-F03 | ValidateProviders | F-BE | LLMGatewayConfig | error/nil | `config/loader.go` (validate) |
| D3-S6-A01-F04 | LoadAPIKey | F-BE | LLMProviderRuntimeConfig | api_key, ok | `config/loader.go` (APIKey) |

## D3-S7-A01 FilterContent

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D3-S7-A01-F01 | CheckContent | F-BE | ctx, system_prompt, []messages | *safety.Result | `safety/filter.go` (Filter.Check) |
| D3-S7-A01-F02 | LoadPatterns | F-BE | — | []Pattern | `safety/patterns.go` (defaultPatterns) |
| D3-S7-A01-F03 | MatchPattern | F-BE | text, pattern | bool | `safety/filter.go` (strings.Contains, case-insensitive) |

## LLM Bridge (D3 → D2)

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D3-S2-A01-F04 | AdaptToContextEngine | F-BE | llmgateway.Request | <-chan Chunk (via ILLMGateway) | `bridges/llm/bridge.go` (Bridge.ChatStream) |
| D3-S2-A01-F05 | WireLLMStack | F-BE | config_file, obs | ContextLLMStack | `bridges/llm/context_wiring.go` (WireContextLLM) + `wire.go` (WireFromConfig) |

---

## Statistics

| Activities with F | Total F Points |
|-------------------|----------------|
| 8 | 22 |
