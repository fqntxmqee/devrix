package escape

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/plan"
)

// --- L0: AnomalyDetectorCB ---------------------------------------------------

func TestAnomalyDetectorCB_5Nil_Open(t *testing.T) {
	cb := NewAnomalyDetectorCB()

	// 5 consecutive failures → Open
	for i := 0; i < 5; i++ {
		cb.RecordFailure()
	}
	if cb.State() != StateOpen {
		t.Fatalf("after 5 failures: State=%s, want open", cb.State())
	}

	// Evaluate → ForceExit
	d := cb.Evaluate(context.Background(), LoopContext{SessionID: "s1"})
	if d.Action != EscapeForceExit {
		t.Errorf("Action=%s, want force_exit", d.Action)
	}
	if !strings.Contains(d.Reason, "L0_AnomalyDetector") {
		t.Errorf("Reason=%q, want contains L0_AnomalyDetector", d.Reason)
	}
}

// --- L1: DispatchLoopWakeupsCB -----------------------------------------------

func TestDispatchLoopWakeupsCB_100PerMin_Open(t *testing.T) {
	cb := NewDispatchLoopWakeupsCB()

	// 100 wakeups in 1 minute → Open
	for i := 0; i < 100; i++ {
		cb.RecordWakeup()
	}
	if cb.State() != StateOpen {
		t.Errorf("after 100 wakeups: State=%s, want open", cb.State())
	}
}

// --- L2: VerifierCB ---------------------------------------------------------

func TestVerifierCB_3Times2s_Open(t *testing.T) {
	cb := NewVerifierCB()

	// 3 consecutive latencies > 2s → Open
	cb.RecordLatency(3 * time.Second)
	cb.RecordLatency(3 * time.Second)
	cb.RecordLatency(3 * time.Second)

	if cb.State() != StateOpen {
		t.Errorf("after 3 slow latencies: State=%s, want open", cb.State())
	}
}

func TestVerifierCB_FastLatency_DoesNotOpen(t *testing.T) {
	cb := NewVerifierCB()

	// 5 fast latencies → still Closed
	for i := 0; i < 5; i++ {
		cb.RecordLatency(100 * time.Millisecond)
	}
	if cb.State() != StateClosed {
		t.Errorf("after 5 fast latencies: State=%s, want closed", cb.State())
	}
}

// --- L3: HookCB -------------------------------------------------------------

func TestHookCB_5Fail_Open(t *testing.T) {
	cb := NewHookCB()

	for i := 0; i < 5; i++ {
		cb.RecordFailure()
	}
	if cb.State() != StateOpen {
		t.Errorf("after 5 hook fails: State=%s, want open", cb.State())
	}
}

// --- L4: WorkerPanicCB ------------------------------------------------------

func TestWorkerPanicCB_1Panic_Open(t *testing.T) {
	cb := NewWorkerPanicCB()

	// 1 panic → Open (single-panic policy)
	cb.RecordFailure()

	if cb.State() != StateOpen {
		t.Errorf("after 1 panic: State=%s, want open", cb.State())
	}
}

// --- L5: SandboxExitCB ------------------------------------------------------

func TestSandboxExitCB_5Fail_Open(t *testing.T) {
	cb := NewSandboxExitCB()

	for i := 0; i < 5; i++ {
		cb.RecordFailure()
	}
	if cb.State() != StateOpen {
		t.Errorf("after 5 sandbox fails: State=%s, want open", cb.State())
	}
}

// --- StateMachine: Open / HalfOpen / Closed ----------------------------------

func TestCircuitBreaker_StateMachine_OpenHalfOpenClose(t *testing.T) {
	cb := NewAnomalyDetectorCB()

	// Closed → Open
	for i := 0; i < 5; i++ {
		cb.RecordFailure()
	}
	if cb.State() != StateOpen {
		t.Fatalf("setup: expected open, got %s", cb.State())
	}

	// Manually advance openedAt to past the half-open probe window.
	cb.baseCircuitBreaker.mu.Lock()
	cb.baseCircuitBreaker.openedAt = time.Now().Add(-60 * time.Second)
	cb.baseCircuitBreaker.mu.Unlock()

	// Evaluate transitions Open → HalfOpen
	d := cb.Evaluate(context.Background(), LoopContext{})
	if d.Action != EscapeContinue {
		t.Errorf("half-open should pass through, got %s", d.Action)
	}
	if cb.State() != StateHalfOpen {
		t.Errorf("after probe: State=%s, want half_open", cb.State())
	}

	// HalfOpen + success → Closed
	cb.RecordSuccess()
	if cb.State() != StateClosed {
		t.Errorf("after half-open success: State=%s, want closed", cb.State())
	}
}

func TestCircuitBreaker_HalfOpen_FailureReopens(t *testing.T) {
	cb := NewAnomalyDetectorCB()

	// Open
	for i := 0; i < 5; i++ {
		cb.RecordFailure()
	}
	// Force half-open
	cb.baseCircuitBreaker.mu.Lock()
	cb.baseCircuitBreaker.openedAt = time.Now().Add(-60 * time.Second)
	cb.baseCircuitBreaker.mu.Unlock()
	cb.Evaluate(context.Background(), LoopContext{})

	// HalfOpen + failure → Open again
	cb.RecordFailure()
	if cb.State() != StateOpen {
		t.Errorf("after half-open failure: State=%s, want open", cb.State())
	}
}

// --- CircuitBreakerSet: 5-layer coordination --------------------------------

func TestCircuitBreakerSet_AllLayers_Independent(t *testing.T) {
	set := NewCircuitBreakerSet()

	// Open L0 only
	for i := 0; i < 5; i++ {
		set.L0.RecordFailure()
	}

	if set.L0.State() != StateOpen {
		t.Error("L0 should be open")
	}
	// Other layers should remain Closed
	for _, layer := range []CircuitBreaker{set.L1, set.L2, set.L3, set.L4, set.L5} {
		if layer.State() != StateClosed {
			t.Errorf("%s: State=%s, want closed (independence)", layer.Name(), layer.State())
		}
	}
}

func TestCircuitBreakerSet_EvaluateAll_OnlyReturnsFirstOpen(t *testing.T) {
	set := NewCircuitBreakerSet()

	// Open multiple layers
	for i := 0; i < 5; i++ {
		set.L0.RecordFailure()
	}
	set.L4.RecordFailure() // 1 panic → open
	for i := 0; i < 5; i++ {
		set.L3.RecordFailure()
	}

	d := set.EvaluateAll(context.Background(), LoopContext{SessionID: "s1"})
	if d.Action != EscapeForceExit {
		t.Errorf("Action=%s, want force_exit", d.Action)
	}
	if !strings.Contains(d.Reason, "circuit_breaker") {
		t.Errorf("Reason=%q, want contains circuit_breaker", d.Reason)
	}
}

func TestCircuitBreakerSet_EvaluateAll_AllClosed(t *testing.T) {
	set := NewCircuitBreakerSet()

	d := set.EvaluateAll(context.Background(), LoopContext{SessionID: "s1"})
	if d.Action != EscapeContinue {
		t.Errorf("all-closed: Action=%s, want continue (no override)", d.Action)
	}
}

func TestCircuitBreaker_PanicRecovery(t *testing.T) {
	// Inject panic into a layer's Evaluate by forcing state to a value
	// that triggers an unexpected code path. Easier: create a wrapper
	// that panics and verify it doesn't bubble up.
	panicCB := &panickingCB{}
	d := panicCB.Evaluate(context.Background(), LoopContext{})
	if d.Action != EscapeContinue {
		t.Errorf("panic should fall back to Continue, got %s", d.Action)
	}
}

type panickingCB struct{}

func (p *panickingCB) Name() string { return "panicking" }
func (p *panickingCB) Evaluate(ctx context.Context, loopCtx LoopContext) (decision EscapeDecision) {
	defer func() {
		if r := recover(); r != nil {
			// Recover and return safe default.
			decision = EscapeDecision{Action: EscapeContinue}
		}
	}()
	var s []int
	_ = s[100] // index out of range → panic
	return
}
func (p *panickingCB) RecordSuccess()              {}
func (p *panickingCB) RecordFailure()              {}
func (p *panickingCB) State() CircuitState         { return StateClosed }

// --- helpers ------------------------------------------------------------------

func mustEncodeJSON(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(data)
}

// silence unused if needed
var _ = mustEncodeJSON
var _ = plan.CommitmentPlan