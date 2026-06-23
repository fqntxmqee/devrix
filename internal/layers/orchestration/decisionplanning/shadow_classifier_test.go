package decisionplanning

import (
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/learn"
	"github.com/devrix/devrix/internal/layers/observability/instrument/metrics"
	"github.com/devrix/devrix/internal/layers/observability/configure/settings"
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
	result orchtypes.IntentClassification
	err    error
	calls  int32
}

func (s *stubLLM) ClassifyIntent(ctx context.Context, _ string) (orchtypes.IntentClassification, error) {
	atomic.AddInt32(&s.calls, 1)
	if s.delay > 0 {
		select {
		case <-time.After(s.delay):
		case <-ctx.Done():
			return orchtypes.IntentClassification{}, ctx.Err()
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
	rule := NewRuleClassifier(orchtypes.DefaultConfig())
	sc := NewShadowClassifier(rule, nil, nil, 500)
	result, err := sc.Classify(context.Background(), "请帮我设计一个分布式缓存")
	if err != nil {
		t.Fatalf("Classify err: %v", err)
	}
	if result.Kind != orchtypes.IntentFast {
		t.Fatalf("expected orchtypes.IntentFast (loop_first routing), got %v", result.Kind)
	}
	if result.Reason != "loop_first_default" {
		t.Fatalf("expected loop_first_default, got %q", result.Reason)
	}
}

// T: D7-S5-T07 AC3 — rule 命中 fast/command/skip 时，LLM 不调用。
func TestShadowClassifier_TailOnly_NotCalledOnFast(t *testing.T) {
	rule := NewRuleClassifier(orchtypes.DefaultConfig())
	llm := &stubLLM{result: orchtypes.IntentClassification{Kind: orchtypes.IntentOrchestrate, Confidence: 90}}
	mtr := newShadowTestMeter(t)
	m := NewShadowMetrics(mtr)
	sc := NewShadowClassifier(rule, llm, m, 500)
	// "hi" matches greeting fast pattern → orchtypes.IntentFast
	result, err := sc.Classify(context.Background(), "hi")
	if err != nil {
		t.Fatalf("Classify err: %v", err)
	}
	if result.Kind != orchtypes.IntentFast {
		t.Fatalf("expected orchtypes.IntentFast, got %v", result.Kind)
	}
	// Wait briefly to ensure no async invocation sneaks in.
	time.Sleep(30 * time.Millisecond)
	if atomic.LoadInt32(&llm.calls) != 0 {
		t.Fatalf("LLM called on fast path, calls=%d", llm.calls)
	}
}

// T: D7-S5-T07 AC3 — empty message → orchtypes.IntentSkip, no LLM.
func TestShadowClassifier_TailOnly_NotCalledOnSkip(t *testing.T) {
	rule := NewRuleClassifier(orchtypes.DefaultConfig())
	llm := &stubLLM{result: orchtypes.IntentClassification{Kind: orchtypes.IntentOrchestrate}}
	sc := NewShadowClassifier(rule, llm, nil, 500)
	result, _ := sc.Classify(context.Background(), "")
	if result.Kind != orchtypes.IntentSkip {
		t.Fatalf("expected orchtypes.IntentSkip, got %v", result.Kind)
	}
	time.Sleep(20 * time.Millisecond)
	if atomic.LoadInt32(&llm.calls) != 0 {
		t.Fatalf("LLM called on skip path")
	}
}

// T: D7-S5-T07 AC3 — command → orchtypes.IntentCommand, no LLM.
func TestShadowClassifier_TailOnly_NotCalledOnCommand(t *testing.T) {
	rule := NewRuleClassifier(orchtypes.DefaultConfig())
	llm := &stubLLM{result: orchtypes.IntentClassification{Kind: orchtypes.IntentOrchestrate}}
	sc := NewShadowClassifier(rule, llm, nil, 500)
	result, _ := sc.Classify(context.Background(), "/plan some goal")
	if result.Kind != orchtypes.IntentCommand {
		t.Fatalf("expected orchtypes.IntentCommand, got %v", result.Kind)
	}
	time.Sleep(20 * time.Millisecond)
	if atomic.LoadInt32(&llm.calls) != 0 {
		t.Fatalf("LLM called on command path")
	}
}

// T: D7-S5-T07 AC4 — rule 返回 orchestrate 时，异步触发 LLM。
func TestShadowClassifier_TailOnly_AsyncOnOrchestrate(t *testing.T) {
	rule := NewRuleClassifier(orchtypes.DefaultConfig())
	llm := &stubLLM{
		delay:  10 * time.Millisecond,
		result: orchtypes.IntentClassification{Kind: orchtypes.IntentOrchestrate, Confidence: 80},
	}
	mtr := newShadowTestMeter(t)
	m := NewShadowMetrics(mtr)
	sc := NewShadowClassifier(rule, llm, m, 500)
	// Return synchronously.
	result, err := sc.Classify(context.Background(), "请帮我设计一个分布式缓存")
	if err != nil {
		t.Fatalf("Classify err: %v", err)
	}
	if result.Kind != orchtypes.IntentFast {
		t.Fatalf("expected orchtypes.IntentFast (loop_first routing), got %v", result.Kind)
	}
	if result.Reason != "loop_first_default" {
		t.Fatalf("expected loop_first_default, got %q", result.Reason)
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
	rule := NewRuleClassifier(orchtypes.DefaultConfig())
	llm := &stubLLM{
		result: orchtypes.IntentClassification{Kind: orchtypes.IntentOrchestrate, Confidence: 95},
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
	rule := NewRuleClassifier(orchtypes.DefaultConfig())
	llm := &stubLLM{
		result: orchtypes.IntentClassification{Kind: orchtypes.IntentCommand, Confidence: 90}, // rule: orchestrate
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
	rule := NewRuleClassifier(orchtypes.DefaultConfig())
	llm := &stubLLM{
		delay:  200 * time.Millisecond, // > 50ms timeout
		result: orchtypes.IntentClassification{Kind: orchtypes.IntentOrchestrate},
	}
	mtr := newShadowTestMeter(t)
	m := NewShadowMetrics(mtr)
	sc := NewShadowClassifier(rule, llm, m, 50) // 50ms timeout
	result, err := sc.Classify(context.Background(), "请帮我设计一个分布式缓存")
	if err != nil {
		t.Fatalf("Classify err (should be silent on shadow error): %v", err)
	}
	if result.Kind != orchtypes.IntentFast {
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
	rule := NewRuleClassifier(orchtypes.DefaultConfig())
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
	rule := NewRuleClassifier(orchtypes.DefaultConfig())
	llm := &stubLLM{
		delay:  5 * time.Millisecond,
		result: orchtypes.IntentClassification{Kind: orchtypes.IntentOrchestrate},
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
	rule := NewRuleClassifier(orchtypes.DefaultConfig())
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
	rule := NewRuleClassifier(orchtypes.DefaultConfig())
	llm := &stubLLM{
		delay:  15 * time.Millisecond,
		result: orchtypes.IntentClassification{Kind: orchtypes.IntentOrchestrate},
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

// T: D7-S12-A42-T04 — ShadowClassifier.ClassifyWithPrior delegates to the
// underlying rule's ClassifyWithPrior. The return value MUST reflect the
// prior adjustment (Mean as confidence multiplier), proving the delegate
// call actually invoked the rule's WithPrior variant and not a stale
// baseline Classify.
func TestShadowClassifier_ClassifyWithPrior_DelegatesToRule(t *testing.T) {
	rule := NewRuleClassifier(orchtypes.DefaultConfig())
	// LLM never matters for this test — we only check the synchronous
	// return value of ClassifyWithPrior (no LLM call expected for fast
	// path "hi" which doesn't fall through to the tail).
	llm := &stubLLM{result: orchtypes.IntentClassification{Kind: orchtypes.IntentOrchestrate}}
	mtr := newShadowTestMeter(t)
	m := NewShadowMetrics(mtr)
	sc := NewShadowClassifier(rule, llm, m, 500)

	// prior Beta(8,3) → Mean = 8/11 ≈ 0.727 → baseline 95 × 0.727 = 69
	prior := learn.BuildAdaptivePrior(nil, learn.TrackModeDeveloper)
	prior.PriorBeta = learn.BetaPrior{Alpha: 8, Beta: 3}

	result, err := sc.ClassifyWithPrior(context.Background(), "hi", prior)
	if err != nil {
		t.Fatalf("ClassifyWithPrior err: %v", err)
	}
	if result.Kind != orchtypes.IntentFast {
		t.Errorf("Kind = %v, want IntentFast (greeting fast pattern)", result.Kind)
	}
	mean := 8.0 / 11.0
	wantConfidence := int(float64(95) * mean)
	if result.Confidence != wantConfidence {
		t.Errorf("Confidence = %d, want %d (baseline 95 × mean 0.727 = delegate must apply prior)",
			result.Confidence, wantConfidence)
	}
	// LLM MUST NOT be called on fast path even with prior (ClassifyWithPrior
	// delegates synchronously to rule, no async shadow on fast kind).
	time.Sleep(30 * time.Millisecond)
	if c := atomic.LoadInt32(&llm.calls); c != 0 {
		t.Errorf("LLM called on fast path: %d (must be 0)", c)
	}
}

// T: D7-S12-A42-T04 — ShadowClassifier.ClassifyWithPrior behavior gap
// (current implementation). The Phase 6 design (shadow_classifier.go
// comment) says the async LLM shadow should still fire on the
// ClassifyWithPrior path with no prior, so shadow samples stay
// comparable. The actual implementation simply delegates to
// rule.ClassifyWithPrior and does NOT fire the async shadow at all.
//
// This test pins the current behavior so we notice if/when someone
// fixes the gap (e.g. extracts shadowAsync trigger into a shared
// helper called by both Classify and ClassifyWithPrior). If the
// assertion below ever flips from "LLM not called" to "LLM called
// once with no prior", that means the design gap has been closed.
//
// Why this is acceptable for v1.0: the synchronous return of
// ClassifyWithPrior is the routing decision; the async LLM shadow is
// only an observability sample stream. When prior is wired (LP-1
// closed loop), losing the async shadow observability is a minor
// regression — Phase 7+ can refactor shadow_async into a shared
// helper without changing the public surface.
func TestShadowClassifier_ClassifyWithPrior_AsyncShadowNotFired(t *testing.T) {
	rule := NewRuleClassifier(orchtypes.DefaultConfig())
	llm := &stubLLM{
		delay:  10 * time.Millisecond,
		result: orchtypes.IntentClassification{Kind: orchtypes.IntentOrchestrate, Confidence: 80},
	}
	mtr := newShadowTestMeter(t)
	m := NewShadowMetrics(mtr)
	sc := NewShadowClassifier(rule, llm, m, 500)

	prior := learn.BuildAdaptivePrior(nil, learn.TrackModeDeveloper)
	prior.PriorBeta = learn.BetaPrior{Alpha: 8, Beta: 3}

	// "请帮我设计一个分布式缓存" falls through to the loop_first_default
	// (would trigger shadow on the no-prior Classify path).
	result, err := sc.ClassifyWithPrior(context.Background(), "请帮我设计一个分布式缓存", prior)
	if err != nil {
		t.Fatalf("ClassifyWithPrior err: %v", err)
	}
	if result.Kind != orchtypes.IntentFast {
		t.Errorf("Kind = %v, want IntentFast (loop_first routing)", result.Kind)
	}
	// Wait the same window Classify test uses to allow any async
	// invocation to surface.
	time.Sleep(60 * time.Millisecond)
	if c := atomic.LoadInt32(&llm.calls); c != 0 {
		t.Errorf("LLM called %d times on ClassifyWithPrior path (current implementation: shadow not fired; "+
			"if this flips to 1, the design gap has been closed — update the comment in shadow_classifier.go)",
			c)
	}
	// Disabled counter MUST NOT increment either (ClassifyWithPrior skips
	// the llm==nil early-return that bumps Disabled).
	if v := m.Disabled.Value(); v != 0 {
		t.Errorf("Disabled counter = %d, want 0 (ClassifyWithPrior does not go through the llm==nil branch)", v)
	}
}
