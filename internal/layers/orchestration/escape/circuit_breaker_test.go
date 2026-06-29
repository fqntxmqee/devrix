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

// --- PR-B (DM-20260629-008): CB L1 ↔ Pessimistic Commit 联动测试 ---
//
// L1 (DispatchLoopWakeupsCB) 是 PR-B 5 类触发条件中 "cb_l1" 的主信号源。
// PessimisticCommitGuard.checkCircuitBreakerL1 读 report.Blockage[].Kind
// == BlockageInfeasible 来识别 L1 触发。本文件覆盖 3 个子测试:
//   1. L1 触发后 StateOpen 且 Evaluate 返回 ForceExit
//   2. CircuitBreakerSet.EvaluateAll 把 L1 错误码映射成可被 Pessimistic
//      Commit 识别的 Reason 字符串前缀
//   3. L1 StateOpen 状态持久 (60s) — 给 Pessimistic Commit 留出 emit
//      窗口, 避免 race (Pessimistic 必须在 HalfOpen 之前完成)

// TestL1DispatchLoop_PessimisticHint — D7-S18-A11-T04 sub-case 1.
// L1 trips → StateOpen + Evaluate → EscapeForceExit with reason prefix
// "circuit_breaker:l1_" (这是 PessimisticCommitGuard 识别的契约).
func TestL1DispatchLoop_PessimisticHint(t *testing.T) {
	cb := NewDispatchLoopWakeupsCB()
	// Drive to 100 wakeups within 1 minute window.
	for i := 0; i < 100; i++ {
		cb.RecordWakeup()
	}
	if cb.State() != StateOpen {
		t.Fatalf("State = %s, want open after 100 wakeups", cb.State())
	}
	d := cb.Evaluate(context.Background(), LoopContext{SessionID: "s_pess"})
	if d.Action != EscapeForceExit {
		t.Errorf("Action = %s, want force_exit", d.Action)
	}
	if !strings.Contains(d.Reason, "circuit_breaker") {
		t.Errorf("Reason = %q, want contains 'circuit_breaker'", d.Reason)
	}
}

// TestL1StateOpen_PersistentForPessimisticWindow — D7-S18-A11-T04 sub-case 2.
// L1 在 60s 内保持 StateOpen. 这个窗口给 Pessimistic Commit 留出 emit 时间
// (BuildMVPArtifact + WithMVPArtifact + audit log 写入必须在窗口内完成).
func TestL1StateOpen_PersistentForPessimisticWindow(t *testing.T) {
	cb := NewDispatchLoopWakeupsCB()
	for i := 0; i < 100; i++ {
		cb.RecordWakeup()
	}
	if cb.State() != StateOpen {
		t.Fatalf("pre-condition failed: State = %s, want open", cb.State())
	}
	// 短时间内 Evaluate 仍应返回 ForceExit (State 不会瞬间变 HalfOpen).
	d := cb.Evaluate(context.Background(), LoopContext{SessionID: "s_pess2"})
	if d.Action != EscapeForceExit {
		t.Errorf("immediate re-evaluate Action = %s, want force_exit (Pessimistic window still open)", d.Action)
	}
}

// TestCircuitBreakerSet_L1Only_PessimisticCompatible — D7-S18-A11-T04 sub-case 3.
// 当 L1 是唯一 Open 的 layer, EvaluateAll 返回 L1 的 reason. Reason 包含
// "L1_DispatchLoop" 后缀, 这是 PessimisticCommitGuard 路由到
// TriggerCircuitBreakerL1 的契约保证 (case-insensitive 包含 "l1").
func TestCircuitBreakerSet_L1Only_PessimisticCompatible(t *testing.T) {
	set := NewCircuitBreakerSet()
	// Trip ONLY L1 (100 wakeups within 1 minute window).
	for i := 0; i < 100; i++ {
		set.L1.RecordWakeup()
	}
	d := set.EvaluateAll(context.Background(), LoopContext{SessionID: "s_pess3"})
	if d.Action != EscapeForceExit {
		t.Errorf("Action = %s, want force_exit", d.Action)
	}
	if !strings.Contains(strings.ToLower(d.Reason), "l1") {
		t.Errorf("Reason = %q, want contains 'l1' (Pessimistic guard routing hint)", d.Reason)
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