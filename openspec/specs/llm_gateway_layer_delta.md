# Delta: LLM Gateway Layer (Layer 3)

**Change ID:** devrix-foundation
**Affects:** LLM gateway, model adapters, circuit breaker, token counter

---

## ADDED

### Requirement: Model Adapter Interface

Unified interface for multiple LLM providers.

#### Scenario: Call Anthropic Claude
- GIVEN provider is 'anthropic' and model is 'claude-3-5-sonnet'
- WHEN llmGateway.chat is called with messages
- THEN AnthropicAdapter formats request per API spec
- AND streams response back

#### Scenario: Call DeepSeek
- GIVEN provider is 'deepseek' and model is 'deepseek-chat'
- WHEN llmGateway.chat is called with messages
- THEN DeepSeekAdapter formats request
- AND streams response back

#### Scenario: Call OpenAI Compatible
- GIVEN provider is 'openai-compatible' and model is 'qwen'
- WHEN llmGateway.chat is called with messages
- THEN OpenAIAdapter formats request
- AND streams response back

#### Scenario: Unknown provider
- GIVEN provider is not supported
- WHEN llmGateway.chat is called
- THEN UnsupportedModelError is thrown

---

### Requirement: Circuit Breaker

Failure rate-based circuit protection.

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
- WHEN probe request succeeds
- THEN circuit state returns to 'closed'
- AND failure count resets to 0

#### Scenario: Circuit reopens after failed probe
- GIVEN circuit is 'half-open'
- WHEN probe request fails
- THEN circuit state returns to 'open'
- AND 30-second timer restarts

---

### Requirement: Token Counter

Budget-aware token tracking.

#### Scenario: Count tokens before call
- GIVEN messages array
- WHEN tokenCounter.count is called
- THEN total token count is returned
- AND broken down by message role

#### Scenario: Check budget before call
- GIVEN token count and budget
- WHEN tokenCounter.checkBudget is called
- THEN if within budget, proceed
- AND if over budget, throw TokenBudgetExceededError

#### Scenario: Estimate response tokens
- GIVEN current token count and max tokens setting
- WHEN tokenCounter.estimateRemaining is called
- THEN available tokens for response is returned

---

### Requirement: Model Configuration

#### Scenario: Load model config for provider
- GIVEN provider is 'anthropic'
- WHEN getModelConfig is called
- THEN config includes: endpoint, timeout, maxTokens, retryConfig
- AND default headers are set

#### Scenario: Fallback model on failure
- GIVEN primary model 'claude-3-5-sonnet' fails
- WHEN fallback model 'claude-3-5-haiku' is configured
- THEN circuit breaker triggers fallback
- AND request is retried with fallback model

---

## MODIFIED

(None - initial layer specification)

---

## REMOVED

| Item | Reason |
|------|--------|
| Rate Limiter | V2 feature |
| Adaptive Model Selection | Future feature |
