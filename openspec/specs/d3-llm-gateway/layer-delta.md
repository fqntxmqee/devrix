# Delta: Domain D3 (LLM Gateway)

**Change ID:** devrix-llm-gateway (V1), devrix-llm-gateway-v2 (V2)
**Demand:** DM-20260607-004, DM-20260608-002
**Affects:** LLM gateway, model adapters, circuit breaker, retry, token counter, safety filter, L2 bridge
**Status:** Merged → `openspec/specs/d3-llm-gateway/spec.md`
**Version:** 2.1.0
**Last Updated:** 2026-06-14

---

## ADDED (V1 - DM-20260607-004)

### Requirement: Model Adapter Interface

Unified OpenAI-compatible streaming interface for multiple LLM providers via `OpenAIStreamClient`.

#### Scenario: Call DeepSeek
- GIVEN provider is 'deepseek' and model is configured
- WHEN Gateway.Stream() is called with messages
- THEN DeepSeekAdapter delegates to OpenAIStreamClient
- AND `buildOpenAIChatRequest` formats request per OpenAI-compatible API spec
- AND `streamOpenAISSE` parses SSE response into chunks

#### Scenario: Call MiniMax
- GIVEN provider is 'minimax' and model is configured
- WHEN Gateway.Stream() is called with messages
- THEN MiniMaxAdapter delegates to OpenAIStreamClient
- AND response is streamed back via channel

#### Scenario: Unknown provider
- GIVEN provider is not in Registry
- WHEN Registry.Get() is called
- THEN error is returned

---

### Requirement: Circuit Breaker

Failure rate-based circuit protection with configurable scope (provider).

#### Scenario: Circuit closed (normal operation)
- GIVEN failure count is 0
- WHEN LLM call succeeds
- THEN failure count remains 0
- AND circuit state is 'closed'

#### Scenario: Circuit opens after threshold
- GIVEN failure count exceeds FailureThreshold (default: 5)
- WHEN LLM call fails
- THEN circuit state changes to 'open'
- AND subsequent calls receive CircuitOpenError
- AND timer starts for half-open attempt

#### Scenario: Circuit half-open (probe)
- GIVEN circuit has been open for OpenDuration (default: 30s)
- WHEN next LLM call is attempted
- THEN circuit state changes to 'half-open'
- AND at most HalfOpenMaxProbes concurrent requests allowed (default: 1)

#### Scenario: Circuit closes after successful probes
- GIVEN circuit is 'half-open'
- WHEN SuccessThreshold consecutive probe requests succeed (default: 2)
- THEN circuit state returns to 'closed'
- AND failure count resets to 0

#### Scenario: Circuit reopens after failed probe
- GIVEN circuit is 'half-open'
- WHEN probe request fails
- THEN circuit state returns to 'open'
- AND OpenDuration timer restarts

---

### Requirement: Token Counter

Budget-aware token tracking using cl100k_base encoding (embedded BPE, no network).

#### Scenario: Count tokens
- GIVEN messages array with system prompt
- WHEN Counter.CountWithSystemPrompt() is called
- THEN total token count includes role/message overhead (+3 per message) and reply priming (+3)
- AND count uses embedded cl100k_base BPE via tiktoken-go

#### Scenario: Check budget
- GIVEN token count and budget (default: 128000)
- WHEN Counter.CheckBudget() is called
- THEN if within budget, proceed
- AND if over budget, return TokenBudgetExceededError

#### Scenario: Truncate to tokens
- GIVEN text and max token limit
- WHEN Counter.TruncateToTokens() is called
- THEN text is truncated to at most maxTokens with "[truncated]" suffix

---

### Requirement: Model Configuration

#### Scenario: Load provider config
- GIVEN provider is configured in devrix.yaml
- WHEN Loader.Load() is called
- THEN validates DefaultProvider exists in Providers
- AND validates each provider has BaseURL, APIKeyEnv, and DefaultModel

#### Scenario: Provider routing
- GIVEN model_routing maps "deepseek-*" → "deepseek"
- WHEN Router.Resolve("deepseek-v4-flash") is called
- THEN provider="deepseek", model="deepseek-v4-flash"

#### Scenario: Empty model defaults
- GIVEN model="" and DefaultProvider="minimax"
- WHEN Router.Resolve("") is called
- THEN provider="minimax", model=provider's DefaultModel

---

### Requirement: Retry Strategy

#### Scenario: Retry on transient failure with Full Jitter
- GIVEN LLM call fails with retryable error
- WHEN retry is enabled
- THEN Retry.Stream applies Full Jitter exponential backoff
- AND delay = random(0, min(cap, MaxDelay))

#### Scenario: Fallback model
- GIVEN primary model fails after all retry attempts
- AND fallback model is configured
- WHEN Retry.Stream is called
- THEN switches to fallback model with fresh attempts

#### Scenario: Do not retry auth failure
- GIVEN LLM call fails with AuthFailed (401/403)
- WHEN retry is attempted
- THEN IsRetryable returns false and retry stops immediately

---

### Requirement: L2 Bridge

Thin adapter implements `llmgateway.ILLMGateway` without L3 importing L2.

#### Scenario: Bridge.ChatStream delegates to Gateway.Stream
- GIVEN IGateway implementation
- WHEN Bridge.ChatStream is called
- THEN forces Stream=true on the internal request
- AND delegates to Gateway.Stream()

#### Scenario: Bridge.ResolveTier
- GIVEN Bridge wrapping Gateway
- WHEN Bridge.ResolveTier("fast") is called
- THEN delegates to Gateway.ResolveTier()
- AND returns error if result is empty

#### Scenario: WireContextLLM bootstrap
- GIVEN devrix.yaml config file path
- WHEN WireContextLLM is called
- THEN loads config → wires Gateway → wraps in Bridge
- AND returns ContextLLMStack with Gateway, TokenCounter, DefaultModel, TierResolver
- AND falls back to mock on any error

---

### Requirement: Observability

#### Scenario: Gateway spans
- GIVEN Gateway.Stream is called
- WHEN processing
- THEN creates hierarchical spans: llm.stream → llm.provider.route → llm.circuit_breaker → llm.retry → llm.adapter.stream
- AND records request/response payload as span events

#### Scenario: Metrics emission
- GIVEN LLM call completes
- WHEN success
- THEN llm_requests_total counter + llm_latency_seconds histogram recorded
- WHEN failure
- THEN llm_errors_total counter recorded

#### Scenario: GenAI token usage
- GIVEN LLM call completes with usage data
- WHEN span finalizes
- THEN RecordGenAITokenUsage records input/output/cache_read/reasoning tokens

---

## ADDED (V2 - DM-20260608-002)

### Requirement: CB+Retry Coordination

Gateway calls `RecordFailure` once per retry chain (not per attempt). `context.Canceled` / `context.DeadlineExceeded` do NOT trigger CB failure.

#### Scenario: Single retry success does not open circuit
- GIVEN CB failureThreshold=3, Retry maxAttempts=3
- WHEN first 2 attempts fail, 3rd succeeds
- THEN CB remains closed

#### Scenario: Context cancellation does not trigger CB failure
- GIVEN CB closed
- WHEN streaming request is cancelled
- THEN shouldRecordBreakerFailure returns false → no RecordFailure

---

### Requirement: Context Timeout Propagation

Stream injects provider Timeout when parent context has no deadline.

#### Scenario: Default timeout injection
- GIVEN parent context has no deadline
- WHEN Stream is called
- THEN request ctx has provider-configured timeout (or default 30s)

---

### Requirement: Full Jitter Retry

Retry uses `rand.Int63n(cap)` for randomized delay.

#### Scenario: Randomized delays
- GIVEN retry config with InitialDelay=1s, Backoff=2.0
- WHEN 10 delays are computed
- THEN delays are randomized (not all identical)

---

### Requirement: Half-Open Concurrent Probe Limiting

Half-Open state enforces `HalfOpenMaxProbes` concurrent limit.

#### Scenario: Probe limiting
- GIVEN CB in Half-Open with HalfOpenMaxProbes=1
- WHEN 3 concurrent requests arrive
- THEN 1 allowed, 2 get CircuitOpenError

---

### Requirement: Chinese Token Compensation (P2)

TokenCounter MAY apply CJK multiplier; pure ASCII unaffected.

---

## ADDED (V2.1)

### Requirement: Model Tier Resolution

Gateway supports model tier aliases (fast/default/powerful) resolved before provider routing.

#### Scenario: Tier alias resolution
- GIVEN ModelTiers["fast"] = "MiniMax-M2.7-highspeed"
- WHEN Router.Resolve("fast") is called
- THEN tier is resolved to "MiniMax-M2.7-highspeed" before provider routing

#### Scenario: Gateway.ResolveTier pass-through
- GIVEN tier "powerful" → ModelTiers["powerful"] = "deepseek-v4-latest"
- WHEN Gateway.ResolveTier("powerful") is called
- THEN returns "deepseek-v4-latest"
- WHEN Gateway.ResolveTier("unknown") is called
- THEN returns "unknown" unchanged

---

### Requirement: Safety Filter

Content safety filtering with configurable patterns (case-insensitive substring matching).

#### Scenario: Critical pattern rejection
- GIVEN system prompt contains "generate malware"
- WHEN Filter.Check() is called
- THEN Result.Allowed is false
- AND Result.Reason describes rejection

#### Scenario: Warning pattern
- GIVEN message contains "sk-" (API key prefix leak)
- WHEN Filter.Check() is called
- THEN Result.Allowed is true (warn, not reject)
- AND Result.Matches includes hardcoded_credential warning

#### Scenario: Custom patterns
- GIVEN Filter.AddPattern() adds a custom pattern
- WHEN Filter.Check() is called
- THEN custom pattern is evaluated alongside defaults

---

## MODIFIED

| Item | Version | Change |
|------|---------|--------|
| `TokenUsage` struct | V2.1 | Added `CacheReadTokens`, `ReasoningTokens` fields from provider details |
| `CircuitBreakerConfig` | V2.1 | `OpenDuration` → `time.Duration` (was `int` seconds) |
| `LLMGatewayConfig` | V2.1 | Added `DefaultTier`, `ModelTiers` fields |
| `Router.Resolve` | V2.1 | Tier resolution happens before provider routing |
| SSE parsing | V2.1 | `streamOptions.include_usage` ensures provider emits usage chunk |
| Gateway.Stream | V2 | Added hierarchical span creation, GenAI token recording |

## REMOVED

| Item | Version | Reason |
|------|---------|--------|
| `observer/` package (ILLMObserver, NoOp) | V2 | Replaced by `observability.Bridge` direct integration in Gateway |
| `IHealthCheck` interface | V1.1 | Out of scope |
| `ChatComplete` (non-streaming) | V1 | Out of scope — D3 is streaming-only |
| `ICircuitBreaker.Reset()` | V2 | Not needed; state auto-recovers via Half-Open |
| `adapter/adapter.go` (IAdapter def) | V1 | IAdapter moved to `contracts.go` |
| `token/estimator.go` | V1 | Merged into `counter.go` |
| `config/provider.go` | V1 | Moved to `shared/config/llmgateway.go` |
