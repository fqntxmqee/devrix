# LLM Gateway Reliability Specification

**Change ID:** devrix-llm-gateway-v2
**Parent Spec:** `openspec/archive/2026-06-07-devrix-llm-gateway/specs/llm-gateway/spec.md`
**Status:** Archived
**Version:** 2.0.0

---

## 1. CircuitBreaker + Retry Coordination

```gherkin
Feature: CircuitBreaker and Retry Coordination

  Scenario: Single retry success does not open circuit
    Given a provider with CB failureThreshold=3
    And Retry maxAttempts=3
    When the first 2 attempts fail and the 3rd succeeds
    Then the CB remains closed
    And the request returns success

  Scenario: All retry attempts fail opens circuit
    Given a provider with CB failureThreshold=3
    And Retry maxAttempts=3
    When all 3 attempts fail with retryable errors
    Then Gateway calls RecordFailure once (not 3 times)
    And the CB opens after the failure

  Scenario: Half-Open limits concurrent probes
    Given a CB in Half-Open state with maxProbes=1
    When 3 concurrent requests arrive
    Then only 1 request is allowed
    And the other 2 receive CircuitOpenError

  Scenario: Half-Open successful probe closes circuit
    Given a CB in Half-Open state with successThreshold=2
    When 2 consecutive requests succeed
    Then the CB transitions to Closed
    And failureCount is reset to 0

  Scenario: Half-Open failed probe re-opens circuit
    Given a CB in Half-Open state
    When a probe request fails
    Then the CB transitions back to Open

  Scenario: Context cancellation does not trigger CB failure
    Given a provider with CB closed
    When a streaming request is in progress and the context is cancelled
    Then the CB remains closed (no RecordFailure)
    And the stream error is context.Canceled

  Scenario: Context deadline exceeded does not trigger CB failure
    Given a provider with CB closed
    When a streaming request exceeds its deadline
    Then the CB remains closed (no RecordFailure)
    And the stream error is context.DeadlineExceeded
```

---

## 2. Context Timeout Propagation

```gherkin
Feature: Context Timeout Propagation

  Scenario: Gateway injects default timeout when none set
    Given the parent context has no deadline
    And gateway DefaultTimeout is 30s
    When Stream is called
    Then the request context has a deadline within 30s

  Scenario: Gateway respects existing context deadline
    Given the parent context has a 5s deadline
    When Stream is called
    Then the 5s deadline is not overridden

  Scenario: Stream goroutine exits on context cancellation
    Given an active streaming request
    When the context is cancelled
    Then the output channel is closed
    And goroutine exits without leaking
```

---

## 3. Retry Jitter

```gherkin
Feature: Retry Jitter

  Scenario: Retry delays are randomized
    Given a retry config with initialDelay=1s, backoff=2.0
    When 10 retry delays are computed for attempt=1
    Then the delays are not all identical
    And all delays are between 0 and 2s (full jitter range)

  Scenario: Jitter respects max delay
    Given maxDelay=10s
    When backoff computation would exceed 10s
    Then all jittered delays are capped at 10s

  Scenario: Deterministic with fixed seed
    Given a retry executor with a fixed random seed
    When the same retry sequence is run twice
    Then both runs produce identical delay sequences
```

---

## 4. Token CJK Compensation (P2)

```gherkin
Feature: Chinese Token Compensation

  Scenario: CJK text gets multiplied by coefficient
    Given cjkMultiplier is 1.5
    When counting "你好世界，这是一个测试" (Chinese text)
    Then the returned token count is cl100k_base_count * 1.5

  Scenario: Pure ASCII text is unaffected
    Given cjkMultiplier is 1.5
    When counting "hello world, this is a test"
    Then the returned token count equals cl100k_base_count (no multiplier)
```
