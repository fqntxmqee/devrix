package coordinator

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/observability/metrics"
	"github.com/devrix/devrix/internal/layers/observability/settings"
)

// newShadowTestMeter returns a fresh MeterProvider-backed Meter.
func newShadowTestMeter(t *testing.T) *metrics.Meter {
	t.Helper()
	mp := metrics.NewMeterProvider(&settings.MetricsConfig{})
	return mp.Meter("d7-shadow-test")
}

// stubLLM is a test LLMIntentClassifier. delay > 0 sleeps before
// returning; result is the configured classification; err is the
// configured error (returned regardless of result).
type stubLLM struct {
	delay  time.Duration
	result IntentClassification
	err    error
	calls  int32
}

func (s *stubLLM) ClassifyIntent(ctx context.Context, _ string) (IntentClassification, error) {
	atomic.AddInt32(&s.calls, 1)
	if s.delay > 0 {
		select {
		case <-time.After(s.delay):
		case <-ctx.Done():
			return IntentClassification{}, ctx.Err()
		}
	}
	return s.result, s.err
}

// waitForCalls polls until the stub's calls counter reaches n or the
// timeout elapses. Returns true on success.
func waitForCalls(stub *stubLLM, n int32, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(&stub.calls) >= n {
			return true
		}
		time.Sleep(2 * time.Millisecond)
	}
	return false
}

// waitForCounter polls until the counter reaches n or the timeout
// elapses. Returns the final value.
func waitForCounter(counter interface{ Value() int64 }, n int64, timeout time.Duration) int64 {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if v := counter.Value(); v >= n {
			return v
		}
		time.Sleep(2 * time.Millisecond)
	}
	return counter.Value()
}

// T: D7-S5-T07 AC8 — ShadowClassifier with nil LLM does not panic and
// does not invoke any LLM. Returns rule result as-is.
func TestShadowClassifier_NilLLM_NoOp(t *testing.T) {
	rule := NewRuleClassifier(DefaultConfig())
	sc := NewShadowClassifier(rule, nil, nil, 500)
	result, err := sc.Classify(context.Background(), "请帮我设计一个分布式缓存")
	if err != nil {
		t.Fatalf("Classify err: %v", err)
	}
	if result.Kind != IntentOrchestrate {
		t.Fatalf("expected IntentOrchestrate, got %v", result.Kind)
	}
}

// T: D7-S5-T07 AC3 — rule 命中 fast/command/skip 时，LLM 不调用。
func TestShadowClassifier_TailOnly_NotCalledOnFast(t *testing.T) {
	rule := NewRuleClassifier(DefaultConfig())
	llm := &stubLLM{result: IntentClassification{Kind: IntentOrchestrate, Confidence: 90}}
	mtr := newShadowTestMeter(t)
	m := NewShadowMetrics(mtr)
	sc := NewShadowClassifier(rule, llm, m, 500)
	// "hi" matches greeting fast pattern → IntentFast
	result, err := sc.Classify(context.Background(), "hi")
	if err != nil {
		t.Fatalf("Classify err: %v", err)
	}
	if result.Kind != IntentFast {
		t.Fatalf("expected IntentFast, got %v", result.Kind)
	}
	// Wait briefly to ensure no async invocation sneaks in.
	time.Sleep(30 * time.Millisecond)
	if atomic.LoadInt32(&llm.calls) != 0 {
		t.Fatalf("LLM called on fast path, calls=%d", llm.calls)
	}
}

// T: D7-S5-T07 AC3 — empty message → IntentSkip, no LLM.
func TestShadowClassifier_TailOnly_NotCalledOnSkip(t *testing.T) {
	rule := NewRuleClassifier(DefaultConfig())
	llm := &stubLLM{result: IntentClassification{Kind: IntentOrchestrate}}
	sc := NewShadowClassifier(rule, llm, nil, 500)
	result, _ := sc.Classify(context.Background(), "")
	if result.Kind != IntentSkip {
		t.Fatalf("expected IntentSkip, got %v", result.Kind)
	}
	time.Sleep(20 * time.Millisecond)
	if atomic.LoadInt32(&llm.calls) != 0 {
		t.Fatalf("LLM called on skip path")
	}
}

// T: D7-S5-T07 AC3 — command → IntentCommand, no LLM.
func TestShadowClassifier_TailOnly_NotCalledOnCommand(t *testing.T) {
	rule := NewRuleClassifier(DefaultConfig())
	llm := &stubLLM{result: IntentClassification{Kind: IntentOrchestrate}}
	sc := NewShadowClassifier(rule, llm, nil, 500)
	result, _ := sc.Classify(context.Background(), "/plan some goal")
	if result.Kind != IntentCommand {
		t.Fatalf("expected IntentCommand, got %v", result.Kind)
	}
	time.Sleep(20 * time.Millisecond)
	if atomic.LoadInt32(&llm.calls) != 0 {
		t.Fatalf("LLM called on command path")
	}
}

// T: D7-S5-T07 AC4 — rule 返回 orchestrate 时，异步触发 LLM。
func TestShadowClassifier_TailOnly_AsyncOnOrchestrate(t *testing.T) {
	rule := NewRuleClassifier(DefaultConfig())
	llm := &stubLLM{
		delay:  10 * time.Millisecond,
		result: IntentClassification{Kind: IntentOrchestrate, Confidence: 80},
	}
	mtr := newShadowTestMeter(t)
	m := NewShadowMetrics(mtr)
	sc := NewShadowClassifier(rule, llm, m, 500)
	// Return synchronously.
	result, err := sc.Classify(context.Background(), "请帮我设计一个分布式缓存")
	if err != nil {
		t.Fatalf("Classify err: %v", err)
	}
	if result.Kind != IntentOrchestrate {
		t.Fatalf("expected IntentOrchestrate, got %v", result.Kind)
	}
	// Wait for async LLM call AND metric update.
	if !waitForCalls(llm, 1, 200*time.Millisecond) {
		t.Fatalf("LLM was not called within 200ms")
	}
	if v := waitForCounter(m.Match, 1, 500*time.Millisecond); v != 1 {
		t.Fatalf("Match counter = %d, want 1", v)
	}
}

// T: D7-S5-T07 AC6 — LLM match rule → match counter 递增。
func TestShadowClassifier_LLM_Match(t *testing.T) {
	rule := NewRuleClassifier(DefaultConfig())
	llm := &stubLLM{
		result: IntentClassification{Kind: IntentOrchestrate, Confidence: 95},
	}
	mtr := newShadowTestMeter(t)
	m := NewShadowMetrics(mtr)
	sc := NewShadowClassifier(rule, llm, m, 500)
	sc.Classify(context.Background(), "请帮我设计一个分布式缓存")
	if !waitForCalls(llm, 1, 200*time.Millisecond) {
		t.Fatalf("LLM not called")
	}
	if v := waitForCounter(m.Match, 1, 500*time.Millisecond); v != 1 {
		t.Fatalf("Match counter = %d, want 1", v)
	}
	if v := m.Mismatch.Value(); v != 0 {
		t.Fatalf("Mismatch counter = %d, want 0", v)
	}
}

// T: D7-S5-T07 AC7 — LLM mismatch rule → mismatch counter 递增。
func TestShadowClassifier_LLM_Mismatch(t *testing.T) {
	rule := NewRuleClassifier(DefaultConfig())
	llm := &stubLLM{
		result: IntentClassification{Kind: IntentCommand, Confidence: 90}, // rule: orchestrate
	}
	mtr := newShadowTestMeter(t)
	m := NewShadowMetrics(mtr)
	sc := NewShadowClassifier(rule, llm, m, 500)
	sc.Classify(context.Background(), "请帮我设计一个分布式缓存")
	if !waitForCalls(llm, 1, 200*time.Millisecond) {
		t.Fatalf("LLM not called")
	}
	if v := waitForCounter(m.Mismatch, 1, 500*time.Millisecond); v != 1 {
		t.Fatalf("Mismatch counter = %d, want 1", v)
	}
	if v := m.Match.Value(); v != 0 {
		t.Fatalf("Match counter = %d, want 0", v)
	}
}

// T: D7-S5-T07 AC5 — LLM error/timeout → error counter 递增；不传播到 caller。
func TestShadowClassifier_LLMTimeout_Error(t *testing.T) {
	rule := NewRuleClassifier(DefaultConfig())
	llm := &stubLLM{
		delay:  200 * time.Millisecond, // > 50ms timeout
		result: IntentClassification{Kind: IntentOrchestrate},
	}
	mtr := newShadowTestMeter(t)
	m := NewShadowMetrics(mtr)
	sc := NewShadowClassifier(rule, llm, m, 50) // 50ms timeout
	result, err := sc.Classify(context.Background(), "请帮我设计一个分布式缓存")
	if err != nil {
		t.Fatalf("Classify err (should be silent on shadow error): %v", err)
	}
	if result.Kind != IntentOrchestrate {
		t.Fatalf("rule result mutated: %v", result.Kind)
	}
	if !waitForCalls(llm, 1, 300*time.Millisecond) {
		t.Fatalf("LLM not called")
	}
	// Wait for shadow to finish processing the timeout.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if m.Error.Value() >= 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if v := m.Error.Value(); v < 1 {
		t.Fatalf("Error counter = %d, want ≥ 1 (timeout)", v)
	}
}

// T: D7-S5-T07 AC5b — LLM returns error sentinel → error counter 递增。
func TestShadowClassifier_LLMError_Handled(t *testing.T) {
	rule := NewRuleClassifier(DefaultConfig())
	llm := &stubLLM{
		err: errors.New("simulated LLM gateway failure"),
	}
	mtr := newShadowTestMeter(t)
	m := NewShadowMetrics(mtr)
	sc := NewShadowClassifier(rule, llm, m, 500)
	_, err := sc.Classify(context.Background(), "请帮我设计一个分布式缓存")
	if err != nil {
		t.Fatalf("Classify err (caller should not see shadow error): %v", err)
	}
	if !waitForCalls(llm, 1, 200*time.Millisecond) {
		t.Fatalf("LLM not called")
	}
	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		if m.Error.Value() >= 1 {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("Error counter = %d, want ≥ 1 (LLM error)", m.Error.Value())
}

// T: D7-S5-T07 AC9 — nil receiver Classify returns error (defensive).
func TestShadowClassifier_NilReceiver_ReturnsError(t *testing.T) {
	var sc *ShadowClassifier
	_, err := sc.Classify(context.Background(), "x")
	if err == nil {
		t.Fatalf("nil receiver should return error")
	}
}

// T: D7-S5-T07 — concurrent Classify calls are race-free.
func TestShadowClassifier_Concurrent(t *testing.T) {
	rule := NewRuleClassifier(DefaultConfig())
	llm := &stubLLM{
		delay:  5 * time.Millisecond,
		result: IntentClassification{Kind: IntentOrchestrate},
	}
	mtr := newShadowTestMeter(t)
	m := NewShadowMetrics(mtr)
	sc := NewShadowClassifier(rule, llm, m, 500)
	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sc.Classify(context.Background(), "请帮我设计一个分布式缓存")
		}()
	}
	wg.Wait()
	if !waitForCalls(llm, 30, 500*time.Millisecond) {
		t.Fatalf("LLM calls = %d, want 30", llm.calls)
	}
}

// T: NewShadowMetrics with nil meter returns nil (disabled).
func TestNewShadowMetrics_NilMeter_NoOp(t *testing.T) {
	m := NewShadowMetrics(nil)
	if m != nil {
		t.Fatalf("nil meter should yield nil metrics")
	}
}

// T: disabled counter increments when LLM is nil.
func TestShadowClassifier_NilLLM_DisabledCounter(t *testing.T) {
	rule := NewRuleClassifier(DefaultConfig())
	mtr := newShadowTestMeter(t)
	m := NewShadowMetrics(mtr)
	sc := NewShadowClassifier(rule, nil, m, 500)
	sc.Classify(context.Background(), "请帮我设计一个分布式缓存")
	// No goroutine → counter increment is synchronous.
	if v := m.Disabled.Value(); v != 1 {
		t.Fatalf("Disabled counter = %d, want 1", v)
	}
}

// T: latency histogram records observations.
func TestShadowClassifier_Latency_Recorded(t *testing.T) {
	rule := NewRuleClassifier(DefaultConfig())
	llm := &stubLLM{
		delay:  15 * time.Millisecond,
		result: IntentClassification{Kind: IntentOrchestrate},
	}
	mtr := newShadowTestMeter(t)
	m := NewShadowMetrics(mtr)
	sc := NewShadowClassifier(rule, llm, m, 500)
	sc.Classify(context.Background(), "请帮我设计一个分布式缓存")
	if !waitForCalls(llm, 1, 200*time.Millisecond) {
		t.Fatalf("LLM not called")
	}
	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		if m.Latency.Count() >= 1 {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("Latency histogram Count = %d, want ≥ 1", m.Latency.Count())
}
