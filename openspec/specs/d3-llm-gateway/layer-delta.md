# Delta: Domain D3 (LLM)

**Change ID:** devrix-llm-gateway
**Demand:** DM-20260607-004
**Affects:** LLM gateway, model adapters, circuit breaker, token counter, L2 bridge
**Status:** Merged → `openspec/specs/llm-gateway/spec.md` (archived 2026-06-07)
**Version:** 1.0.0

---

## ADDED

### Requirement: Model Adapter Interface

Unified interface for multiple LLM providers.

#### Scenario: Call DeepSeek V4 Pro
- GIVEN provider is 'deepseek' and model is 'deepseek-v4-pro'
- WHEN llmGateway.ChatStream() is called with messages
- THEN DeepSeekAdapter formats request per OpenAI-compatible API spec
- AND streams response back via channel

#### Scenario: Call DeepSeek V4 Flash
- GIVEN provider is 'deepseek' and model is 'deepseek-v4-flash'
- WHEN llmGateway.ChatStream() is called with messages
- THEN DeepSeekAdapter formats request
- AND streams response back via channel

#### Scenario: Call MiniMax 3
- GIVEN provider is 'minimax' and model is 'minimax-3'
- WHEN llmGateway.ChatStream() is called with messages
- THEN MiniMaxAdapter formats request per MiniMax API spec
- AND streams response back via channel

#### Scenario: Call MiniMax 2.7 HighSpeed
- GIVEN provider is 'minimax' and model is 'minimax-2.7-highspeed'
- WHEN llmGateway.ChatStream() is called with messages
- THEN MiniMaxAdapter formats request
- AND streams response back via channel

#### Scenario: Unknown provider
- GIVEN provider is not supported
- WHEN llmGateway.ChatStream() is called
- THEN ErrUnsupportedProvider is returned

#### Scenario: Unknown model
- GIVEN model does not exist for provider
- WHEN llmGateway.ChatStream() is called
- THEN ErrUnsupportedModel is returned

---

### Requirement: Circuit Breaker

Failure rate-based circuit protection with configurable scope.

#### Scenario: Circuit closed (normal operation)
- GIVEN failure count is 0
- WHEN LLM call succeeds
- THEN failure count remains 0
- AND circuit state is 'closed'

#### Scenario: Circuit opens after threshold
- GIVEN failure count exceeds threshold (default: 5)
- WHEN LLM call fails
- THEN circuit state changes to 'open'
- AND subsequent calls are rejected immediately
- AND timer starts for half-open attempt

#### Scenario: Circuit half-open (probe)
- GIVEN circuit has been open for 30 seconds
- WHEN next LLM call is attempted
- THEN circuit state changes to 'half-open'
- AND a single probe request is allowed

#### Scenario: Circuit closes after successful probe
- GIVEN circuit is 'half-open'
- WHEN probe request succeeds (>= 2 consecutive successes)
- THEN circuit state returns to 'closed'
- AND failure count resets to 0

#### Scenario: Circuit reopens after failed probe
- GIVEN circuit is 'half-open'
- WHEN probe request fails
- THEN circuit state returns to 'open'
- AND 30-second timer restarts

#### Scenario: Circuit breaker per provider
- GIVEN circuit breaker scope is 'provider'
- WHEN deepseek fails 5 times
- THEN all deepseek models are blocked
- AND minimax remains unaffected

#### Scenario: Circuit breaker per model
- GIVEN circuit breaker scope is 'provider:model'
- WHEN deepseek-v4-pro fails 5 times
- THEN only deepseek-v4-pro is blocked
- AND deepseek-v4-flash remains available

---

### Requirement: Token Counter

Budget-aware token tracking using cl100k_base encoding.

#### Scenario: Count tokens before call
- GIVEN messages array
- WHEN tokenCounter.Count() is called
- THEN total token count is returned
- AND broken down by message role

#### Scenario: Check budget before call
- GIVEN token count and budget
- WHEN tokenCounter.CheckBudget() is called
- THEN if within budget, proceed
- AND if over budget, throw TokenBudgetExceededError

#### Scenario: Estimate response tokens
- GIVEN current token count and max tokens setting
- WHEN tokenCounter.EstimateRemaining() is called
- THEN available tokens for response is returned

#### Scenario: Token count accuracy
- GIVEN a known text string
- WHEN tokenCounter.Count() is called
- THEN count matches cl100k_base ± 5%
- AND this is verified by unit tests

---

### Requirement: Model Configuration

#### Scenario: Load model config for DeepSeek
- GIVEN provider is 'deepseek'
- WHEN getModelConfig() is called
- THEN config includes: endpoint, timeout, maxTokens, retryConfig
- AND API key is loaded from DEEPSEEK_API_KEY env

#### Scenario: Load model config for MiniMax
- GIVEN provider is 'minimax'
- WHEN getModelConfig() is called
- THEN config includes: endpoint, timeout, maxTokens, retryConfig
- AND API key is loaded from MINIMAX_API_KEY env

#### Scenario: Fallback model on failure
- GIVEN primary model 'deepseek-v4-pro' fails with retryable error
- WHEN fallback model 'deepseek-v4-flash' is configured in ProviderConfig
- THEN RetryExecutor retries with exponential backoff on primary
- AND after max attempts switches to fallback model
- AND circuit breaker records failure only after retry chain exhausts

#### Scenario: No fallback configured
- GIVEN primary model fails
- AND no fallback model is configured
- WHEN request is made
- THEN error is returned immediately

---

### Requirement: Retry Strategy

#### Scenario: Retry on transient failure
- GIVEN LLM call fails with timeout
- WHEN retry is enabled (max_attempts: 3)
- THEN request is retried with exponential backoff (1s, 2s, 4s)
- AND max attempts is respected

#### Scenario: Retry on parse error
- GIVEN LLM call returns unparseable response
- WHEN retry is enabled
- THEN request is retried (parse errors may be transient)

#### Scenario: Do not retry on auth failure
- GIVEN LLM call fails with auth error
- WHEN request is made
- THEN error is returned immediately
- AND no retry is attempted

#### Scenario: Fail after max retries
- GIVEN LLM call fails 3 times consecutively
- WHEN max retries is 3
- THEN error is returned to caller
- AND circuit breaker failure count is incremented

---

### Requirement: L2 Bridge

Thin adapter implements `contextengine.ILLMGateway` without L3 importing L2.

#### Scenario: Bridge adapts ChatStream types
- GIVEN Gateway Stream returns llmgateway.Chunk without RiskLevel
- WHEN Bridge.ChatStream is called with contextengine.LLMRequest
- THEN provider is resolved via model_routing
- AND chunks are mapped to contextengine.LLMChunk
- AND ToolCall.RiskLevel is left empty for L2 to fill

#### Scenario: Unknown model routing
- GIVEN model name matches no model_routing prefix
- AND model is non-empty
- WHEN Bridge.ChatStream is called
- THEN ErrUnsupportedModel (LLM_UNSUPPORTED_1008) is returned

---

### Requirement: Observability

#### Scenario: Emit LLM call metrics
- GIVEN LLM call is made
- WHEN call completes
- THEN metrics are emitted with provider, model, duration, success flag
- AND devrix_llm_latency_seconds histogram is recorded

#### Scenario: Emit token usage
- GIVEN LLM call completes
- WHEN usage is available from API response
- THEN token usage is emitted with provider, model, prompt/completion counts
- AND devrix_llm_tokens_total counter is incremented

#### Scenario: Emit circuit state changes
- GIVEN circuit state changes from closed to open
- WHEN transition occurs
- THEN event is emitted with provider and new state
- AND devrix_llm_circuit_breaker_state gauge is updated

#### Scenario: Emit span for LLM call
- GIVEN LLM call is initiated
- WHEN call is in progress
- THEN OpenTelemetry span is created
- AND span includes provider, model, prompt tokens as attributes

---

## MODIFIED

(None - initial layer specification)

---

## REMOVED

| Item | Reason |
|------|--------|
| Anthropic adapter | V2 feature |
| OpenAI adapter | V2 feature |
| Rate Limiter | V2 feature |
| Health Check (IHealthCheck) | V1.1 feature |
| ChatComplete (non-streaming) | Out of scope V1 |
| Adaptive Model Selection | Future feature |
