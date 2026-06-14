package protect_test

import (
	"errors"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/llmgateway/protect"
	sharedconfig "github.com/devrix/devrix/internal/shared/config"
	sharederrors "github.com/devrix/devrix/internal/shared/errors"
)

func newTestBreaker() (*protect.CircuitBreaker, *fakeClock) {
	clock := &fakeClock{t: time.Unix(0, 0)}
	cfg := sharedconfig.LLMCircuitBreakerConfig{
		FailureThreshold: 3,
		SuccessThreshold: 2,
		OpenDuration:     30 * time.Second,
		Scope:            "provider",
	}
	cb := protect.New(cfg).WithClock(clock.Now)
	return cb, clock
}

type fakeClock struct {
	t time.Time
}

func (c *fakeClock) Now() time.Time { return c.t }

func (c *fakeClock) Advance(d time.Duration) { c.t = c.t.Add(d) }

// T: D3-S3-A01-T01
func TestCircuitBreaker_should_stay_closed_on_success(t *testing.T) {
	cb, _ := newTestBreaker()
	const key = "deepseek"

	allowed, err := cb.Allow(key)
	if err != nil || !allowed {
		t.Fatalf("Allow: allowed=%v err=%v", allowed, err)
	}
	cb.RecordSuccess(key)
	if cb.State(key) != llmgateway.CircuitClosed {
		t.Errorf("state: got %s", cb.State(key))
	}
}

// T: D3-S3-A01-T02
func TestCircuitBreaker_should_open_after_failure_threshold(t *testing.T) {
	cb, _ := newTestBreaker()
	const key = "deepseek"

	for i := 0; i < 3; i++ {
		cb.RecordFailure(key)
	}
	if cb.State(key) != llmgateway.CircuitOpen {
		t.Errorf("state: got %s", cb.State(key))
	}
	allowed, err := cb.Allow(key)
	if allowed || err == nil {
		t.Fatalf("expected circuit open rejection, allowed=%v err=%v", allowed, err)
	}
	var llmErr *sharederrors.LLMError
	if !errors.As(err, &llmErr) || llmErr.Code != sharederrors.CodeLLMCircuitOpen {
		t.Errorf("err: %v", err)
	}
}

// T: D3-S3-A01-T03
func TestCircuitBreaker_should_close_after_half_open_successes(t *testing.T) {
	cb, clock := newTestBreaker()
	const key = "minimax"

	for i := 0; i < 3; i++ {
		cb.RecordFailure(key)
	}
	clock.Advance(31 * time.Second)

	allowed, err := cb.Allow(key)
	if err != nil || !allowed {
		t.Fatalf("half-open Allow: %v", err)
	}
	if cb.State(key) != llmgateway.CircuitHalfOpen {
		t.Errorf("state after timeout: got %s", cb.State(key))
	}

	cb.RecordSuccess(key)
	if cb.State(key) != llmgateway.CircuitHalfOpen {
		t.Errorf("state after one success: got %s", cb.State(key))
	}
	cb.RecordSuccess(key)
	if cb.State(key) != llmgateway.CircuitClosed {
		t.Errorf("state after two successes: got %s", cb.State(key))
	}
}

// T: D3-S3-A01-T04
func TestCircuitBreaker_should_reopen_when_half_open_probe_fails(t *testing.T) {
	cb, clock := newTestBreaker()
	const key = "minimax"

	for i := 0; i < 3; i++ {
		cb.RecordFailure(key)
	}
	clock.Advance(31 * time.Second)

	if _, err := cb.Allow(key); err != nil {
		t.Fatalf("Allow: %v", err)
	}
	cb.RecordFailure(key)
	if cb.State(key) != llmgateway.CircuitOpen {
		t.Errorf("state: got %s", cb.State(key))
	}

	allowed, err := cb.Allow(key)
	if allowed || err == nil {
		t.Fatal("expected open rejection immediately after failed probe")
	}
}

// T: D3-S3-A01-T05
func TestCircuitBreaker_should_limit_half_open_concurrent_probes(t *testing.T) {
	clock := &fakeClock{t: time.Unix(0, 0)}
	cfg := sharedconfig.LLMCircuitBreakerConfig{
		FailureThreshold:  3,
		SuccessThreshold:  2,
		OpenDuration:      30 * time.Second,
		HalfOpenMaxProbes: 1,
		Scope:             "provider",
	}
	cb := protect.New(cfg).WithClock(clock.Now)
	const key = "deepseek"

	for i := 0; i < 3; i++ {
		cb.RecordFailure(key)
	}
	clock.Advance(31 * time.Second)

	allowed, err := cb.Allow(key)
	if err != nil || !allowed {
		t.Fatalf("first half-open probe: allowed=%v err=%v", allowed, err)
	}
	allowed, err = cb.Allow(key)
	if allowed || err == nil {
		t.Fatalf("expected second probe rejected, allowed=%v err=%v", allowed, err)
	}
	var llmErr *sharederrors.LLMError
	if !errors.As(err, &llmErr) || llmErr.Code != sharederrors.CodeLLMCircuitOpen {
		t.Fatalf("err: %v", err)
	}
}

func TestCircuitBreaker_should_isolate_providers(t *testing.T) {
	cb, _ := newTestBreaker()
	for i := 0; i < 3; i++ {
		cb.RecordFailure("deepseek")
	}
	if cb.State("deepseek") != llmgateway.CircuitOpen {
		t.Fatal("deepseek should be open")
	}
	allowed, err := cb.Allow("minimax")
	if err != nil || !allowed {
		t.Errorf("minimax should remain closed: allowed=%v err=%v", allowed, err)
	}
}
