# LLM Gateway Specification

**Capability:** llm-gateway
**Change ID:** devrix-llm-gateway (archived 2026-06-07)
**Demand:** DM-20260607-004
**Layer:** 3
**Version:** 1.0.0
**Status:** Canonical — source of truth

---

## Overview

LLM Gateway Layer（Layer 3）提供多 Provider 流式调用、熔断保护、重试与 fallback、统一 Token 计数（cl100k_base），并通过 `bridges/llm` 适配 Context Engine 的 `ILLMGateway` 契约。

---

## Feature: DeepSeek Adapter

### Scenario: Call DeepSeek V4 Pro

- **Given** provider is "deepseek" and model is "deepseek-v4-pro"
- **When** `llmGateway.ChatStream()` is called with messages
- **Then** `DeepSeekAdapter` formats request per OpenAI-compatible API spec
- **And** response is streamed back via channel

### Scenario: Call DeepSeek V4 Flash

- **Given** provider is "deepseek" and model is "deepseek-v4-flash"
- **When** `llmGateway.ChatStream()` is called with messages
- **Then** `DeepSeekAdapter` formats request
- **And** response is streamed back via channel

### Scenario: DeepSeek fallback to flash

- **Given** deepseek-v4-pro fails
- **When** fallback model "deepseek-v4-flash" is configured
- **Then** request is retried with fallback model

---

## Feature: MiniMax Adapter

### Scenario: Call MiniMax 3

- **Given** provider is "minimax" and model is "minimax-3"
- **When** `llmGateway.ChatStream()` is called with messages
- **Then** `MiniMaxAdapter` formats request per MiniMax API spec
- **And** response is streamed back via channel

### Scenario: Call MiniMax 2.7 HighSpeed

- **Given** provider is "minimax" and model is "minimax-2.7-highspeed"
- **When** `llmGateway.ChatStream()` is called with messages
- **Then** `MiniMaxAdapter` formats request
- **And** response is streamed back via channel

### Scenario: MiniMax fallback to highspeed

- **Given** minimax-3 fails
- **When** fallback model "minimax-2.7-highspeed" is configured
- **Then** request is retried with fallback model

---

## Feature: Circuit Breaker

### Scenario: Circuit closed (normal operation)

- **Given** failure count is 0
- **When** LLM call succeeds
- **Then** failure count remains 0
- **And** circuit state is "closed"

### Scenario: Circuit opens after threshold

- **Given** failure count exceeds threshold (default: 5)
- **When** LLM call fails
- **Then** circuit state changes to "open"
- **And** subsequent calls are rejected immediately
- **And** timer starts for half-open attempt

### Scenario: Circuit half-open (probe)

- **Given** circuit has been open for 30 seconds
- **When** next LLM call is attempted
- **Then** circuit state changes to "half-open"
- **And** a single probe request is allowed

### Scenario: Circuit closes after successful probe

- **Given** circuit is "half-open"
- **When** probe request succeeds
- **Then** circuit state returns to "closed"
- **And** failure count resets to 0

### Scenario: Circuit reopens after failed probe

- **Given** circuit is "half-open"
- **When** probe request fails
- **Then** circuit state returns to "open"
- **And** 30-second timer restarts

---

## Feature: Token Counter

### Scenario: Count tokens before call

- **Given** messages array
- **When** `tokenCounter.Count()` is called
- **Then** total token count is returned
- **And** broken down by message role

### Scenario: Check budget before call

- **Given** token count and budget
- **When** `tokenCounter.CheckBudget()` is called
- **Then** if within budget, proceed
- **And** if over budget, throw `TokenBudgetExceededError`

### Scenario: Estimate response tokens

- **Given** current token count and max tokens setting
- **When** `tokenCounter.EstimateRemaining()` is called
- **Then** available tokens for response is returned

---

## Feature: Model Configuration

### Scenario: Load model config for DeepSeek

- **Given** provider is "deepseek"
- **When** `getModelConfig()` is called
- **Then** config includes: endpoint, timeout, maxTokens, retryConfig
- **And** default headers are set

### Scenario: Load model config for MiniMax

- **Given** provider is "minimax"
- **When** `getModelConfig()` is called
- **Then** config includes: endpoint, timeout, maxTokens, retryConfig
- **And** API key is loaded from MINIMAX_API_KEY env

### Scenario: Fallback model on failure

- **Given** primary model fails with retryable error
- **When** fallback model is configured in ProviderConfig
- **Then** RetryExecutor applies exponential backoff on primary
- **And** after max attempts switches to fallback model
- **And** circuit breaker records failure after retry chain exhausts

---

## Feature: L2 Bridge

### Scenario: Bridge maps types without RiskLevel

- **Given** Gateway returns ToolCall without RiskLevel
- **When** `bridges/llm.Bridge.ChatStream()` is called
- **Then** provider is resolved via model_routing
- **And** chunks map to contextengine.LLMChunk
- **And** Context Engine fills RiskLevel via IToolRegistry

### Scenario: Unknown model routing

- **Given** model matches no routing prefix and is non-empty
- **When** Bridge.ChatStream is called
- **Then** LLM_UNSUPPORTED_1008 is returned

---

## Feature: Retry Strategy

### Scenario: Retry on transient failure

- **Given** LLM call fails with timeout
- **When** retry is enabled
- **Then** request is retried with exponential backoff
- **And** max attempts is respected

### Scenario: Fail after max retries

- **Given** LLM call fails 3 times consecutively
- **When** max retries is 3
- **Then** error is returned to caller

---

## Feature: Observability

### Scenario: Emit LLM call metrics

- **Given** LLM call is made
- **When** call completes
- **Then** metrics are emitted with provider, model, duration, success flag

### Scenario: Emit token usage

- **Given** LLM call completes
- **When** usage is available
- **Then** token usage is emitted with provider, model, prompt/completion counts

### Scenario: Emit circuit state changes

- **Given** circuit state changes
- **When** transition occurs
- **Then** event is emitted with provider and new state
