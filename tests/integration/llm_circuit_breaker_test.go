//go:build integration && d3

package integration

import (
	"errors"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/llmgateway/breaker"
	sharedconfig "github.com/devrix/devrix/internal/shared/config"
	sharederrors "github.com/devrix/devrix/internal/shared/errors"
)

// Covers: L5-LLM-03, L5-LLM-04, L5-LLM-05, L5-LLM-06
func TestIntegration_LLMCircuitBreaker_state_transitions(t *testing.T) {
	start := time.Unix(1_700_000_000, 0)
	now := start
	clock := func() time.Time { return now }

	cfg := sharedconfig.LLMCircuitBreakerConfig{
		FailureThreshold: 2,
		SuccessThreshold: 2,
		OpenDuration:     10 * time.Millisecond,
		Scope:            "provider",
	}
	cb := breaker.New(cfg).WithClock(clock)
	const provider = "deepseek"

	// Closed: success path
	if ok, err := cb.Allow(provider); err != nil || !ok {
		t.Fatalf("closed Allow: ok=%v err=%v", ok, err)
	}
	cb.RecordSuccess(provider)
	if cb.State(provider) != llmgateway.CircuitClosed {
		t.Fatalf("expected closed, got %s", cb.State(provider))
	}

	// Open after failures
	cb.RecordFailure(provider)
	cb.RecordFailure(provider)
	if cb.State(provider) != llmgateway.CircuitOpen {
		t.Fatalf("expected open, got %s", cb.State(provider))
	}
	if ok, err := cb.Allow(provider); ok || err == nil {
		t.Fatalf("expected rejection while open, ok=%v err=%v", ok, err)
	}

	// Half-open after duration
	now = start.Add(20 * time.Millisecond)
	if ok, err := cb.Allow(provider); err != nil || !ok {
		t.Fatalf("half-open Allow: ok=%v err=%v", ok, err)
	}
	if cb.State(provider) != llmgateway.CircuitHalfOpen {
		t.Fatalf("expected half-open, got %s", cb.State(provider))
	}

	// Close after consecutive successes
	cb.RecordSuccess(provider)
	cb.RecordSuccess(provider)
	if cb.State(provider) != llmgateway.CircuitClosed {
		t.Fatalf("expected closed after probes, got %s", cb.State(provider))
	}

	// Re-open path: force open again
	cb.RecordFailure(provider)
	cb.RecordFailure(provider)
	now = now.Add(20 * time.Millisecond)
	if _, err := cb.Allow(provider); err != nil {
		t.Fatalf("Allow half-open: %v", err)
	}
	cb.RecordFailure(provider)
	if cb.State(provider) != llmgateway.CircuitOpen {
		t.Fatalf("expected re-open, got %s", cb.State(provider))
	}
	_, reopenErr := cb.Allow(provider)
	if reopenErr == nil {
		t.Fatal("expected open rejection")
	}
	var llmErr *sharederrors.LLMError
	if !errors.As(reopenErr, &llmErr) || llmErr.Code != sharederrors.CodeLLMCircuitOpen {
		t.Fatalf("err: %v", reopenErr)
	}
}
