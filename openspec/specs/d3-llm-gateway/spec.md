# LLM Gateway Specification

**Capability:** llm-gateway
**Change ID:** devrix-llm-gateway (archived 2026-06-07), devrix-llm-gateway-v2 (archived 2026-06-08)
**Demand:** DM-20260607-004, DM-20260608-002
**Domain:** D3
**DSAFT Type:** 公共域 (Common Domain)
**Version:** 2.1.0
**Status:** Canonical — source of truth
**Last Updated:** 2026-06-14

---

## Overview

LLM Gateway Domain（D3）提供多 Provider 流式调用、熔断保护、重试与 fallback、统一 Token 计数（cl100k_base）、模型层级别名（ModelTier）、Safety 内容过滤，并通过 `bridges/llm` 适配 Context Engine 的 `ILLMGateway` 契约。

V1（DM-20260607-004）建立 Provider 路由、适配器、熔断、重试与 Token 计数；V2（DM-20260608-002）增强 CB+Retry 协调、Half-Open 探测限制、Context 超时传播、Full Jitter 退避与 CJK Token 补偿；V2.1 新增 Safety Filter 内容安全过滤与 ModelTier 层级别名。

**Archive:** `openspec/archive/2026-06-07-devrix-llm-gateway/`, `openspec/archive/2026-06-08-devrix-llm-gateway-v2/`

---

## DSAFT 结构

| 层级 | ID | 名称 | 说明 |
|------|-----|------|------|
| D | D3 | LLM Gateway | 公共域，提供 LLM 调用横向共享能力 |
| S | D3-S1 | Adapter | 多 Provider 统一适配 |
| S | D3-S2 | Gateway | 路由、编排、可观测性 |
| S | D3-S3 | Breaker | 熔断器保护 |
| S | D3-S4 | Retry | 重试与 fallback |
| S | D3-S5 | Token | Token 计数与预算 |
| S | D3-S6 | Config | 配置加载与验证 |
| S | D3-S7 | Safety | 内容安全过滤 |

---

## Feature: DeepSeek Adapter

### Scenario: Call DeepSeek

- **Given** provider is "deepseek" and model is configured
- **When** `Gateway.Stream()` is called with messages
- **Then** `DeepSeekAdapter` formats request per OpenAI-compatible API spec via `OpenAIStreamClient`
- **And** response is streamed back via channel with SSE parsing

### Scenario: DeepSeek fallback

- **Given** deepseek primary model fails
- **When** fallback model is configured in ProviderConfig
- **Then** RetryExecutor switches to fallback model after primary attempts exhaust

---

## Feature: MiniMax Adapter

### Scenario: Call MiniMax

- **Given** provider is "minimax" and model is configured
- **When** `Gateway.Stream()` is called with messages
- **Then** `MiniMaxAdapter` formats request per OpenAI-compatible API spec via `OpenAIStreamClient`
- **And** response is streamed back via channel with SSE parsing

### Scenario: MiniMax fallback

- **Given** minimax primary model fails
- **When** fallback model is configured in ProviderConfig
- **Then** RetryExecutor switches to fallback model after primary attempts exhaust

---

## Feature: Model Tier Resolution

### Scenario: Resolve tier alias to concrete model

- **Given** `ModelTiers` config maps "fast" → "MiniMax-M2.7-highspeed"
- **When** `Gateway.ResolveTier("fast")` or `Router.Resolve("fast")` is called
- **Then** returns "MiniMax-M2.7-highspeed"

### Scenario: Unknown tier passes through

- **Given** tier name is not in `ModelTiers` config
- **When** `Gateway.ResolveTier("unknown")` is called
- **Then** returns "unknown" unchanged

---

## Feature: Circuit Breaker

### Scenario: Circuit closed (normal operation)

- **Given** failure count is 0
- **When** LLM call succeeds
- **Then** failure count resets to 0
- **And** circuit state is "closed"

### Scenario: Circuit opens after threshold

- **Given** failure count exceeds FailureThreshold (default: 5)
- **When** LLM call fails
- **Then** circuit state changes to "open"
- **And** subsequent calls are rejected immediately with CircuitOpenError
- **And** timer starts for half-open attempt

### Scenario: Circuit half-open (probe)

- **Given** circuit has been open for OpenDuration (default: 30s)
- **When** next LLM call is attempted
- **Then** circuit state changes to "half-open"
- **And** a limited number of probe requests are allowed (HalfOpenMaxProbes, default: 1)

### Scenario: Circuit closes after successful probes

- **Given** circuit is "half-open"
- **When** SuccessThreshold consecutive probe requests succeed (default: 2)
- **Then** circuit state returns to "closed"
- **And** failure count resets to 0

### Scenario: Circuit reopens after failed probe

- **Given** circuit is "half-open"
- **When** probe request fails
- **Then** circuit state returns to "open"
- **And** OpenDuration timer restarts

---

## Feature: Token Counter

### Scenario: Count tokens before call

- **Given** messages array with system prompt
- **When** `Counter.CountWithSystemPrompt()` is called
- **Then** total token count is returned (including role/message overhead)

### Scenario: Check budget before call

- **Given** token count and budget (default: 128000)
- **When** `Counter.CheckBudget()` is called
- **Then** if within budget, proceed
- **And** if over budget, return `TokenBudgetExceededError`

### Scenario: Truncate text to token limit

- **Given** text and max token limit
- **When** `Counter.TruncateToTokens()` is called
- **Then** text is truncated to at most maxTokens

### Scenario: Chinese CJK compensation (P2)

- **Given** CJK text content
- **When** token counting is performed
- **Then** CJK multiplier MAY be applied; pure ASCII text MUST NOT be affected

---

## Feature: Model Configuration

### Scenario: Load provider config via Router

- **Given** provider is configured in `LLMGatewayConfig`
- **When** `Router.Resolve(model)` is called
- **Then** returns provider name, resolved model, and nil error
- **And** empty model defaults to `DefaultProvider` + provider's `DefaultModel`

### Scenario: Model routing by pattern

- **Given** `ModelRouting` config has "deepseek-*" → "deepseek"
- **When** model "deepseek-v4-flash" is resolved
- **Then** provider is "deepseek", model stays "deepseek-v4-flash"

### Scenario: Tier alias in model routing

- **Given** model "fast" is passed, `ModelTiers["fast"]` = "MiniMax-M2.7-highspeed"
- **When** `Router.Resolve("fast")` is called
- **Then** tier is resolved BEFORE provider routing, model becomes "MiniMax-M2.7-highspeed"

### Scenario: Fallback model on failure

- **Given** primary model fails with retryable error
- **When** fallback model is configured in ProviderConfig
- **Then** RetryExecutor applies Full Jitter backoff on primary
- **And** after max attempts switches to fallback model
- **And** circuit breaker records failure only after entire retry chain exhausts

---

## Feature: L2 Bridge

### Scenario: Bridge adapts D3 gateway to ILLMGateway contract

- **Given** `IGateway` implementation
- **When** `Bridge.ChatStream()` is called with `llmgateway.Request`
- **Then** request.Stream is forced to true
- **And** delegates to `IGateway.Stream()`
- **And** returns `<-chan llmgateway.Chunk` for D2 consumers

### Scenario: Bridge resolves tier aliases

- **Given** Bridge wrapping an IGateway
- **When** `Bridge.ResolveTier(tier)` is called
- **Then** delegates to `IGateway.ResolveTier()`
- **And** returns error if resolved model is empty

---

## Feature: Retry Strategy

### Scenario: Retry on transient failure with Full Jitter

- **Given** LLM call fails with retryable error (timeout/provider_unavailable/parse_error)
- **When** retry is enabled
- **Then** request is retried with Full Jitter exponential backoff
- **And** delay is randomized between 0 and cap (capped at MaxDelay)

### Scenario: Fail after max retries

- **Given** LLM call fails on all attempts
- **When** max attempts is exhausted for both primary and fallback
- **Then** error is returned to caller
- **And** Gateway calls CircuitBreaker.RecordFailure once

### Scenario: Do not retry on auth failure

- **Given** LLM call fails with auth error (401/403)
- **When** request is made
- **Then** error is returned immediately without retry

---

## Feature: Safety Filter

### Scenario: Content filtering on system prompt and messages

- **Given** Safety Filter is configured with default patterns
- **When** `Filter.Check()` is called with system prompt and messages
- **Then** patterns are matched case-insensitively
- **And** Result includes matches with severity and action

### Scenario: Reject on critical match

- **Given** system prompt or message matches a critical pattern (e.g., malware_generation)
- **When** `Filter.Check()` is called
- **Then** Result.Allowed is false
- **And** Result.Reason describes the rejection

### Scenario: Warn on medium match

- **Given** message matches a medium-severity pattern (e.g., prompt_injection, hardcoded_credential)
- **When** `Filter.Check()` is called
- **Then** Result.Allowed is true
- **And** matches are logged as warnings

---

## Feature: Observability

### Scenario: Emit LLM call metrics

- **Given** LLM call is made via Gateway.Stream
- **When** call completes (success or error)
- **Then** metrics are emitted via `observability.Bridge.Meter()`:
  - `llm_requests_total` counter (success)
  - `llm_errors_total` counter (error)
  - `llm_latency_seconds` histogram

### Scenario: Emit OpenTelemetry spans

- **Given** LLM call is in progress
- **When** Gateway processes the request
- **Then** child spans are created for: `llm.stream`, `llm.provider.route`, `llm.circuit_breaker`, `llm.retry`, `llm.adapter.stream`
- **And** span attributes include provider, model, token counts, usage_received flag

### Scenario: Emit GenAI token breakdown

- **Given** LLM call completes with usage data
- **When** span is finalized
- **Then** `observability.RecordGenAITokenUsage` records input/output/cache_read/reasoning tokens

---

## ADDED Requirements (V2 Reliability)

### Requirement: CircuitBreaker and Retry Coordination

Gateway MUST 仅在 Retry 链整体失败后调用 `RecordFailure` 一次；`context.Canceled` 与 `context.DeadlineExceeded` MUST NOT 触发 CB failure。Half-Open 状态 MUST 限制并发探测数（`HalfOpenMaxProbes`，默认 1），超限请求返回 `CircuitOpenError`。

**Priority**: P0
**L4**: L4-LLM-BREAKER, L4-LLM-GATEWAY
**T**: D3-S2-A01-T04, D3-S2-A01-T05

#### Scenario: Single retry success does not open circuit

- **Given** a provider with CB failureThreshold=3 and Retry maxAttempts=3
- **When** the first 2 attempts fail and the 3rd succeeds
- **Then** the CB remains closed

#### Scenario: All retry attempts fail opens circuit

- **Given** a provider with CB failureThreshold=3 and Retry maxAttempts=3
- **When** all 3 attempts fail with retryable errors
- **Then** Gateway calls RecordFailure once (not 3 times)
- **And** the CB opens after the failure

#### Scenario: Half-Open limits concurrent probes

- **Given** a CB in Half-Open state with HalfOpenMaxProbes=1
- **When** 3 concurrent requests arrive
- **Then** only 1 request is allowed
- **And** the other 2 receive CircuitOpenError

#### Scenario: Context cancellation does not trigger CB failure

- **Given** a provider with CB closed
- **When** a streaming request is cancelled via context
- **Then** the CB remains closed (no RecordFailure)

### Requirement: Context Timeout Propagation

当父 Context 无 deadline 时，Gateway MUST 注入 provider 级 timeout（默认 30s）；已有 deadline MUST NOT 被覆盖。流式 goroutine MUST 在 Context 取消或超时时退出并关闭输出 channel。

**Priority**: P1
**L4**: L4-LLM-GATEWAY
**T**: D3-S2-A01-T04

#### Scenario: Gateway injects default timeout when none set

- **Given** the parent context has no deadline
- **When** Stream is called
- **Then** the request context has a provider-configured deadline (ProviderConfig.Timeout or default 30s)

#### Scenario: Stream goroutine exits on context cancellation

- **Given** an active streaming request
- **When** the context is cancelled
- **Then** the output channel is closed without goroutine leak

### Requirement: Retry Jitter

Retry 退避 MUST 使用 Full Jitter 随机化；延迟 MUST 受 `MaxDelay` 上限约束。测试环境 MAY 通过固定 RNG seed 实现确定性。

**Priority**: P1
**L4**: L4-LLM-RETRY
**T**: D3-S4-A01-T01

#### Scenario: Retry delays are randomized

- **Given** a retry config with InitialDelay=1s, Backoff=2.0
- **When** multiple retry delays are computed
- **Then** the delays are randomized with Full Jitter
- **And** all delays are between 0 and cap (max MaxDelay)

### Requirement: Chinese Token Compensation (P2)

TokenCounter MAY 对 CJK 文本应用补偿系数；纯 ASCII 文本 MUST NOT 受影响。

**Priority**: P2
**L4**: L4-LLM-TOKEN
