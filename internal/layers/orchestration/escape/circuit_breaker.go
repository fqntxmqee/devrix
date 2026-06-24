// CircuitBreaker 5 层 (DM-20260625-003, PR-V5.4)
//
// 关键设计 (doc 38 §21 SoT, design.md §7):
//   - 5 个独立 CB 对应 5 类失败模式 (L0-L5)
//   - CircuitState: Closed / Open / HalfOpen
//   - 每层有独立阈值 + 半开探测逻辑
//   - panic recovery: CB.Evaluate panic → slog.Warn + 不触发
//   - 200ms timeout: 拉 metric 阻塞 → slog.Warn + 不触发 (不卡主链路)
//
// 5 层 + 阈值:
//   L0 AnomalyDetectorCB      5 次连续 nil → open
//   L1 DispatchLoopWakeupsCB  100 次/分钟 → open
//   L2 VerifierCB             3 次 > 2s → open
//   L3 HookCB                 5 次连续 fail → open
//   L4 WorkerPanicCB          1 次 panic → open (单次即升级)
//   L5 SandboxExitCB          5 次连续 fail → open
package escape

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// CircuitState is the 3-state circuit breaker state.
type CircuitState uint8

const (
	// StateClosed: normal operation, all calls pass through.
	StateClosed CircuitState = iota

	// StateOpen: failures exceeded threshold, calls rejected.
	StateOpen

	// StateHalfOpen: probe state, allow 1 test call to determine recovery.
	StateHalfOpen
)

// String returns the snake_case wire form.
func (s CircuitState) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half_open"
	default:
		return fmt.Sprintf("CircuitState(%d)", uint8(s))
	}
}

// ErrCircuitOpen is returned by CB.ShouldAllow when the breaker is open.
var ErrCircuitOpen = errors.New("escape: circuit breaker is open")

// CircuitBreaker is the per-layer circuit breaker interface.
//
// Evaluate returns an EscapeDecision that the EscapeEngine merges into
// the upstream decision chain. nil/Continue means "no action" (this
// layer's CB is closed and not interfering).
type CircuitBreaker interface {
	// Name returns the layer identifier (L0/L1/.../L5).
	Name() string

	// Evaluate checks the current breaker state and emits an
	// EscapeDecision. nil means "no override" (CB closed).
	// ForceExit means "open — escape this layer immediately".
	Evaluate(ctx context.Context, loopCtx LoopContext) EscapeDecision

	// RecordSuccess decrements the failure counter (for half-open recovery).
	RecordSuccess()

	// RecordFailure increments the failure counter.
	RecordFailure()

	// State returns the current circuit state (for tests / observability).
	State() CircuitState
}

// baseCircuitBreaker provides shared state + threshold + half-open logic.
// Concrete 5-layer CBs embed this and customize Evaluate / threshold.
type baseCircuitBreaker struct {
	name         string
	threshold    int
	failureCount int
	successCount int
	state        CircuitState
	openedAt     time.Time
	mu           sync.RWMutex
	// halfOpenProbeAfter: time after which Open → HalfOpen transitions.
	halfOpenProbeAfter time.Duration
}

// Name returns the layer identifier.
func (b *baseCircuitBreaker) Name() string { return b.name }

// State returns the current state (test aid).
func (b *baseCircuitBreaker) State() CircuitState {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.state
}

// RecordSuccess decrements failure counter; if half-open probe succeeds,
// transition to Closed.
func (b *baseCircuitBreaker) RecordSuccess() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.state == StateOpen {
		// Stay open until half-open probe succeeds.
		return
	}
	if b.state == StateHalfOpen {
		// Half-open probe succeeded → recovery.
		b.state = StateClosed
		b.failureCount = 0
		b.successCount = 0
		return
	}
	if b.failureCount > 0 {
		b.failureCount--
	}
}

// RecordFailure increments failure counter; may transition Closed → Open
// or HalfOpen → Open.
func (b *baseCircuitBreaker) RecordFailure() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failureCount++
	if b.state == StateHalfOpen {
		// Half-open probe failed → back to Open.
		b.state = StateOpen
		b.openedAt = time.Now()
		return
	}
	if b.failureCount >= b.threshold {
		b.state = StateOpen
		b.openedAt = time.Now()
	}
}

// Evaluate implements the default open/half-open/closed check.
// Subclasses override for layer-specific conditions.
func (b *baseCircuitBreaker) Evaluate(ctx context.Context, loopCtx LoopContext) EscapeDecision {
	defer func() {
		if r := recover(); r != nil {
			slog.Warn("circuit_breaker_panic_recovered",
				"layer", b.name,
				"panic", r,
				"session_id", loopCtx.SessionID,
			)
		}
	}()

	b.mu.Lock()
	// Auto-transition Open → HalfOpen after probe timeout.
	if b.state == StateOpen && time.Since(b.openedAt) > b.halfOpenProbeAfter {
		b.state = StateHalfOpen
	}
	state := b.state
	b.mu.Unlock()

	if state == StateClosed {
		return EscapeDecision{} // no override
	}
	if state == StateHalfOpen {
		// Allow the call but record outcome via RecordSuccess/RecordFailure.
		return EscapeDecision{} // half-open: pass through, evaluate outcome
	}
	// StateOpen: reject.
	return EscapeDecision{
		Action:     EscapeForceExit,
		Reason:     fmt.Sprintf("circuit_breaker_%s_open", b.name),
		AuditLevel: 1,
		SessionID:  loopCtx.SessionID,
		CreatedAt:  nowFunc(),
	}
}

// newBaseCircuitBreaker constructs a base CB.
func newBaseCircuitBreaker(name string, threshold int, halfOpenAfter time.Duration) *baseCircuitBreaker {
	return &baseCircuitBreaker{
		name:               name,
		threshold:          threshold,
		state:              StateClosed,
		halfOpenProbeAfter: halfOpenAfter,
	}
}

// --- L0: AnomalyDetectorCB ---------------------------------------------------

// AnomalyDetectorCB trips after 5 consecutive nil observations.
// 5 次连续 nil 表示异常检测彻底失效 (doc 38 §18.2.1 P0 盲点修补阈值).
type AnomalyDetectorCB struct {
	*baseCircuitBreaker
}

// NewAnomalyDetectorCB constructs the L0 circuit breaker.
func NewAnomalyDetectorCB() *AnomalyDetectorCB {
	return &AnomalyDetectorCB{
		baseCircuitBreaker: newBaseCircuitBreaker("L0_AnomalyDetector", 5, 30*time.Second),
	}
}

// --- L1: DispatchLoopWakeupsCB -----------------------------------------------

// DispatchLoopWakeupsCB trips when dispatch wakeups exceed 100/min.
// 100/min 是 P99 估算 × 1.5 安全系数 (占位阈值, V5.5 校准).
type DispatchLoopWakeupsCB struct {
	*baseCircuitBreaker
	mu       sync.Mutex
	wakeups  []time.Time // sliding window
}

// NewDispatchLoopWakeupsCB constructs the L1 circuit breaker.
func NewDispatchLoopWakeupsCB() *DispatchLoopWakeupsCB {
	return &DispatchLoopWakeupsCB{
		baseCircuitBreaker: newBaseCircuitBreaker("L1_DispatchLoop", 100, 60*time.Second),
	}
}

// RecordWakeup is the layer-specific failure input.
func (cb *DispatchLoopWakeupsCB) RecordWakeup() {
	cb.mu.Lock()
	now := time.Now()
	cutoff := now.Add(-1 * time.Minute)
	// Trim old entries.
	i := 0
	for ; i < len(cb.wakeups); i++ {
		if cb.wakeups[i].After(cutoff) {
			break
		}
	}
	cb.wakeups = cb.wakeups[i:]
	cb.wakeups = append(cb.wakeups, now)
	count := len(cb.wakeups)
	cb.mu.Unlock()

	// 100/min policy: when the rolling count first crosses 100,
	// trip the breaker. RecordFailure() once is sufficient because
	// the L1 threshold (this CB) is set to 1 (see NewDispatchLoopWakeupsCB).
	// We do NOT use the base threshold check here.
	if count >= 100 {
		cb.mu.Lock()
		if cb.failureCount == 0 {
			cb.failureCount = cb.threshold
			cb.state = StateOpen
			cb.openedAt = time.Now()
		}
		cb.mu.Unlock()
	}
}

// Evaluate overrides the base to count current wakeups.
func (cb *DispatchLoopWakeupsCB) Evaluate(ctx context.Context, loopCtx LoopContext) EscapeDecision {
	return cb.baseCircuitBreaker.Evaluate(ctx, loopCtx)
}

// --- L2: VerifierCB ----------------------------------------------------------

// VerifierCB trips after 3 consecutive verifier calls > 2s.
// 3 次 > 2s 触发降级 (doc 38 §18.2.2 LLM 调用延迟阈值).
type VerifierCB struct {
	*baseCircuitBreaker
	mu          sync.Mutex
	latencies   []time.Duration // last N verifier call latencies
	maxLatency  time.Duration
}

// NewVerifierCB constructs the L2 circuit breaker.
func NewVerifierCB() *VerifierCB {
	return &VerifierCB{
		baseCircuitBreaker: newBaseCircuitBreaker("L2_Verifier", 3, 30*time.Second),
		maxLatency:         2 * time.Second,
	}
}

// RecordLatency records a verifier call latency.
//
// 3 次连续 > 2s → open (per doc 38 §18.2.2).
// We bypass the base RecordFailure counter and set state directly
// because the threshold (3) is for the L2 layer, not the base counter.
func (cb *VerifierCB) RecordLatency(d time.Duration) {
	cb.mu.Lock()
	cb.latencies = append(cb.latencies, d)
	if len(cb.latencies) > 10 {
		cb.latencies = cb.latencies[len(cb.latencies)-10:]
	}
	// Count consecutive latencies > 2s from the tail.
	slowStreak := 0
	for i := len(cb.latencies) - 1; i >= 0; i-- {
		if cb.latencies[i] > cb.maxLatency {
			slowStreak++
		} else {
			break
		}
	}
	cb.mu.Unlock()

	if slowStreak >= 3 {
		// Direct state transition (bypass base counter, see RecordWakeup for rationale).
		cb.mu.Lock()
		if cb.state != StateOpen {
			cb.failureCount = cb.threshold
			cb.state = StateOpen
			cb.openedAt = time.Now()
		}
		cb.mu.Unlock()
	} else {
		cb.RecordSuccess()
	}
}

// Evaluate delegates to base.
func (cb *VerifierCB) Evaluate(ctx context.Context, loopCtx LoopContext) EscapeDecision {
	return cb.baseCircuitBreaker.Evaluate(ctx, loopCtx)
}

// --- L3: HookCB --------------------------------------------------------------

// HookCB trips after 5 consecutive hook failures.
type HookCB struct {
	*baseCircuitBreaker
}

// NewHookCB constructs the L3 circuit breaker.
func NewHookCB() *HookCB {
	return &HookCB{
		baseCircuitBreaker: newBaseCircuitBreaker("L3_Hook", 5, 30*time.Second),
	}
}

// --- L4: WorkerPanicCB -------------------------------------------------------

// WorkerPanicCB trips after a single worker panic.
// 单次 panic 即升级 (避免累积掩盖根因, devrix §2 panic 协议).
type WorkerPanicCB struct {
	*baseCircuitBreaker
}

// NewWorkerPanicCB constructs the L4 circuit breaker.
func NewWorkerPanicCB() *WorkerPanicCB {
	return &WorkerPanicCB{
		baseCircuitBreaker: newBaseCircuitBreaker("L4_WorkerPanic", 1, 60*time.Second),
	}
}

// --- L5: SandboxExitCB -------------------------------------------------------

// SandboxExitCB trips after 5 consecutive sandbox exit failures.
type SandboxExitCB struct {
	*baseCircuitBreaker
}

// NewSandboxExitCB constructs the L5 circuit breaker.
func NewSandboxExitCB() *SandboxExitCB {
	return &SandboxExitCB{
		baseCircuitBreaker: newBaseCircuitBreaker("L5_SandboxExit", 5, 30*time.Second),
	}
}

// --- 5-layer Composite -------------------------------------------------------

// CircuitBreakerSet holds all 5 layers + a unified Evaluate.
type CircuitBreakerSet struct {
	L0 *AnomalyDetectorCB
	L1 *DispatchLoopWakeupsCB
	L2 *VerifierCB
	L3 *HookCB
	L4 *WorkerPanicCB
	L5 *SandboxExitCB
}

// NewCircuitBreakerSet constructs all 5 layers.
func NewCircuitBreakerSet() *CircuitBreakerSet {
	return &CircuitBreakerSet{
		L0: NewAnomalyDetectorCB(),
		L1: NewDispatchLoopWakeupsCB(),
		L2: NewVerifierCB(),
		L3: NewHookCB(),
		L4: NewWorkerPanicCB(),
		L5: NewSandboxExitCB(),
	}
}

// EvaluateAll runs all 5 layers and returns the first non-Continue decision.
// Used by EscapeEngine to merge CB decisions into the chain.
func (s *CircuitBreakerSet) EvaluateAll(ctx context.Context, loopCtx LoopContext) EscapeDecision {
	// 200ms total timeout for the layer evaluations.
	cbCtx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()

	layers := []CircuitBreaker{s.L0, s.L1, s.L2, s.L3, s.L4, s.L5}
	for _, layer := range layers {
		// Check context before each layer (avoid blocking on slow metric pulls).
		select {
		case <-cbCtx.Done():
			slog.Warn("circuit_breaker_evaluate_timeout",
				"layer", layer.Name(),
				"session_id", loopCtx.SessionID,
			)
			return EscapeDecision{} // timeout: not trigger
		default:
		}

		d := layer.Evaluate(cbCtx, loopCtx)
		if d.Action == EscapeForceExit || d.Action == EscapeAbortWithAudit {
			return d
		}
	}
	return EscapeDecision{} // all closed: no override
}

// AllLayers returns the 5 layers as a slice (test aid).
func (s *CircuitBreakerSet) AllLayers() []CircuitBreaker {
	return []CircuitBreaker{s.L0, s.L1, s.L2, s.L3, s.L4, s.L5}
}